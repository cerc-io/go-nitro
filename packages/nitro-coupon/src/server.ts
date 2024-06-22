import cors from '@fastify/cors';
import {NitroRpcClient} from '@statechannels/nitro-rpc-client';
import {Voucher} from '@statechannels/nitro-rpc-client/src/types.js';
import crypto from 'crypto';
import Fastify from 'fastify';
import JSONBig from 'json-bigint';
import jwt from 'jsonwebtoken';

import {Config} from './config.js';
import {Logger} from './log/index.js';
import {Metrics} from './metrics/index.js';

const log = new Logger('cerc:nitro:coupon');

const metrics = new Metrics();

const fastify: any = Fastify({
  logger: Config.FASTIFY_LOGGER
});

await fastify.register(cors, {
  origin: true
});

fastify.addContentTypeParser('application/json', { parseAs: 'string' }, function (req, body, done) {
  try {
    const json = JSONBig.parse(body);
    done(null, json);
  } catch (err) {
    err.statusCode = 400;
    done(err, undefined);
  }
});

fastify.setSerializerCompiler(({ schema, method, url, httpStatus, contentType }) => {
  return data => JSONBig.stringify(data);
});

const genCouponCode = () => {
  return crypto.randomBytes(48).toString('base64url');
};

class Coupon {
  public _issuer: string; // who is offering the coupon?
  public _expiresIn: string; // the expiration as string, eg "30d" (relative to issuedAt)
  public _issuedAt: number;
  public _subject: string; // the coupon code itself
  public _audience: string; // which provider (or providers) is this coupon good for?
  public _promo: string; // what is the associated promo id?
  public _account: string; // to what account was this coupon issued?
  public _total_value: bigint; // the total value of this coupon (eg, 100)
  public _redemption_increment: bigint; // how much of the value can be redeemed at one time? (eg, 10)
  public _redemption_period: string; // with what frequency can it be redeemed (eg, hourly, daily)

  constructor(issuer: string, audience: string, _account: string, _promo: string, total_value: bigint, issuedAt?: number, subject?: string, redemption_increment?: bigint, redemption_period?: string, expiresIn?: string) {
    this._issuer = issuer;
    this._audience = audience;
    this._account = _account;
    this._promo = _promo;
    this._expiresIn = expiresIn ?? '7d';
    this._issuedAt = issuedAt ?? new Date().getTime();
    this._subject = subject ?? genCouponCode();
    this._total_value = total_value;
    this._redemption_increment = redemption_increment;
    this._redemption_period = redemption_period;
  }

  static fromJWT(j: any) {
    return new Coupon(
      j.iss,
      j.aud,
      j.u,
      j.pm,
      BigInt(j.v),
      j.iat,
      j.sub,
      BigInt(j.ri),
      j.rp,
      j.exp ? `${j.exp - j.iat}s` : undefined
    );
  }

  private _toPayload() {
    const ret: any = {
      pm: this._promo,
      u: this._account,
      v: JSONBig.stringify(this._total_value),
      ri: JSONBig.stringify(this._redemption_increment),
      rp: this._redemption_period
    };
    if (this._issuedAt) {
      ret.iat = this._issuedAt;
    }
    return ret;
  }

  toJSON() {
    return jwt.decode(this.toJWT('fake'));
  }

  toJWT(key) {
    return jwt.sign(
      this._toPayload(),
      key,
      {
        issuer: this._issuer,
        audience: this._audience,
        expiresIn: this._expiresIn,
        subject: this._subject,
      }
    );
  }
}

class CouponStore {
  private _coupons = new Map<string, Coupon>();

  public put(key: string, v: Coupon) {
    this._coupons.set(v._subject, v);
    this._coupons.set(key, v);
  }

  public get(v: string) {
    return this._coupons.get(v);
  }

  public has(v: string) {
    return this._coupons.get(v);
  }
}

const nitro = await NitroRpcClient.CreateHttpNitroClient(`${Config.NITRO_RPC_HOST}:${Config.NITRO_RPC_PORT}/api/v1`, Config.NITRO_RPC_SECURE);
const couponStore = new CouponStore();
const couponToPaymentChannel = new Map<string, string>();
const myAddress = await nitro.GetAddress();

const isActiveProvider = async (address: string) => {
  // TODO: For know, we decide base on the presence of an existing ledger channel between us.
  // We'll need to add additional checks later, but this keeps things simple to start.

  const ledgerChannels = await nitro.GetAllLedgerChannels();
  for (const lc of ledgerChannels) {
    if (lc.Status === 'Open') {
      if (lc.Balance.Them === address) {
        return true;
      }
    }
  }

  return false;
};

const isActivePromotion = async (promo: string, provider: string) => {
  return true;
};

const getChannelForCoupon = async (coupon: Coupon) => {
  let paymentChannelId = couponToPaymentChannel.get(coupon._subject);
  if (!paymentChannelId) {
    const response = await nitro.CreatePaymentChannel(coupon._audience, [], coupon._total_value);
    paymentChannelId = response.ChannelId;
    couponToPaymentChannel.set(coupon._subject, paymentChannelId);
  }

  return nitro.GetPaymentChannel(paymentChannelId);
};

fastify.post('/coupon/issue',
  async (req: any, res: any) => {
    metrics.incCounter('coupon_issue');

    const providerAddress = req.body.provider;
    const userAddress = req.body.user;
    const promotionId = req.body.promo;
    const key = `${providerAddress}:${promotionId}:${userAddress}`;

    // Check that we recognize the provider.
    const knownProvider = await isActiveProvider(providerAddress);
    if (!knownProvider) {
      log.warn(`Unknown provider ${providerAddress}.`);
      res.code(401);
      return '401 Unauthorized';
    }

    // Check that we recognize
    const knownPromo = await isActivePromotion(promotionId, providerAddress);
    if (!knownPromo) {
      log.warn(`Unknown promotion ${promotionId}.`);
      res.code(401);
      return '401 Unauthorized';
    }

    // Check if any existing coupon exists.
    // TODO: Allow issuing coupons by policy, but for now, one per user+promo+provider combo.
    let coupon = couponStore.get(key);

    // Issue a new coupon.
    if (!coupon) {
      coupon = new Coupon(
        myAddress,
        providerAddress,
        userAddress,
        promotionId,
        Config.COUPON_DEFAULT_VALUE,
        undefined,
        undefined,
        Config.COUPON_DEFAULT_REDEMPTION_INCREMENT,
        Config.COUPON_DEFAULT_REDEMPTION_PERIOD,
        Config.COUPON_DEFAULT_EXPIRATION,
      );
      couponStore.put(key, coupon);
    }

    return coupon.toJWT(Config.JWT_KEY);
  }
);


fastify.post('/coupon/redeem',
  {
    schema: {
      response: {
        '2xx': {
          ChannelId: { type: 'string' },
          Signature: { type: 'string' },
          Amount: { type: 'number' },
        }
      }
    }
  },
  async (req: any, res: any) => {
    const couponJwt = jwt.verify(req.body, Config.JWT_KEY);
    if (!couponJwt) {
      log.warn(`Unable to verify coupon ${req.body}.`);
      res.code(401);
      return '401 Unauthorized';
    }

    const coupon = Coupon.fromJWT(couponJwt);

    // Check that we recognize the provider.
    const knownProvider = await isActiveProvider(coupon._audience);
    if (!knownProvider) {
      log.warn(`Unknown provider ${coupon._audience}.`);
      res.code(401);
      return '401 Unauthorized';
    }

    // Check that we (still) recognize the promotion.
    const knownPromo = await isActivePromotion(coupon._promo, coupon._audience);
    if (!knownPromo) {
      log.warn(`Unknown promotion ${coupon._promo}.`);
      res.code(401);
      return '401 Unauthorized';
    }

    // Get (or create) a payment channel to back this coupon.
    const paymentChannel = await getChannelForCoupon(coupon);

    // TODO: Check if the redemption period means it is eligible for redemption (yet).

    // Then create a voucher and return it.
    const howMuch = coupon._redemption_increment ?? coupon._total_value;
    return nitro.CreateVoucher(paymentChannel.ID, howMuch);
  }
);

fastify.get('/metrics', async (req: any, res: any) => {
  return metrics.render();
});

try {
  fastify.server.keepAliveTimeout = Config.HTTP_SERVER_KEEPALIVE_TIMEOUT;
  fastify.server.timeout = Config.HTTP_SERVER_TIMEOUT;
  fastify.listen({ port: Config.LISTEN_PORT, host: Config.LISTEN_ADDR, backlog: Config.HTTP_SERVER_BACKLOG }, () => {
    console.log(`nitro-voucher-auth listening on ${Config.LISTEN_ADDR}:${Config.LISTEN_PORT}`);
  });
} catch (e) {
  log.error(e);
}
