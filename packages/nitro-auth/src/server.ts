import cors from '@fastify/cors';
import {NitroRpcClient} from '@statechannels/nitro-rpc-client';
import {Voucher} from '@statechannels/nitro-rpc-client/src/types.js';
import crypto from 'crypto';
import Fastify from 'fastify';
import JSONBig from 'json-bigint';

import {Config} from './config.js';
import {Logger} from './log/index.js';
import {Metrics} from './metrics/index.js';

const log = new Logger('cerc:nitro:auth');

const metrics = new Metrics();

class AccountStore {
  private _accountByAddress = new Map<string, Account>();

  public put(v: Account) {
    this._accountByAddress.set(v.address, v);
  }

  public get(v: string) {
    return this._accountByAddress.get(v);
  }

  public has(v: string) {
    return this._accountByAddress.get(v);
  }
}

class TokenStore {
  // TODO: More than one token allowed per-address.
  private _tokenByAddress = new Map<string, AuthToken>();
  private _tokenByValue = new Map<string, AuthToken>();

  public put(token: AuthToken) {
    this._tokenByAddress.set(token.account.address, token);
    this._tokenByValue.set(token.value, token);
  }

  public get(v: string) {
    return this._tokenByValue.has(v)  ? this._tokenByValue.get(v) : this._tokenByAddress.get(v);
  }

  public has(v: string) {
    return this._tokenByValue.has(v) || this._tokenByAddress.has(v);
  }
}

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

const genToken = () => {
  return crypto.randomBytes(48).toString('base64url');
};


class Account {
  public address: string;

  constructor(address: string) {
    this.address = address;
  }

  toString() {
    return this.address;
  }
}

class AuthToken {
  public _account: Account;
  public _value: string;
  public _used: bigint;
  public _total: bigint;

  constructor(account: Account, total = 0n, used = 0n, value?: string) {
    this._account = account;
    this._total = total;
    this._used = used;
    this._value = value ?? genToken();
  }

  public add(delta: bigint) {
    this._total += delta;
    return this._total;
  }

  public use(amount: bigint) {
    if (amount + this._used <= this._total) {
      this._used += amount;
      return true;
    }
    return false;
  }

  public updateTotal(t: bigint) {
    if (t > this._total) {
      this._total = t;
      return true;
    }
    return false;
  }

  get remainder() {
    return this._total - this._used;
  }

  get value() {
    return this._value;
  }

  get account() {
    return this._account;
  }

  toJSON() {
    return {
      token: this.value,
      total: this._total,
      used: this._used,
      address: this._account.address,
    };
  }
}

const nitro = await NitroRpcClient.CreateHttpNitroClient(`${Config.NITRO_RPC_HOST}:${Config.NITRO_RPC_PORT}/api/v1`, Config.NITRO_RPC_SECURE);

// TODO: persistence
const tokenStore = new TokenStore();
const accountStore = new AccountStore();

const authTokenSchema = {
  schema: {
    response: {
      '2xx': {
        token: { type: 'string' },
        total: { type: 'number' },
        used: { type: 'number' },
        address: { type: 'string' },
      }
    }
  },
  validatorCompiler: () => () => true,
};

fastify.get('/auth/:token', authTokenSchema, async (req: any, res: any) => {
  metrics.incCounter('get_auth');
  const token = tokenStore.get(req.params.token);
  if (token) {
    metrics.incCounter(`get_auth_${token.account.address}`);
    if (token.use(1n)) {
      metrics.incCounter('get_auth_200');
      return token;
    }
    res.code(402);
    metrics.incCounter('get_auth_402');
    return '402 Payment Required';

  }
  res.code(401);
  metrics.incCounter('get_auth_401');
  return '401 Unauthorized';
});

fastify.post('/account/create', authTokenSchema, async (req: any, res: any) => {
  metrics.incCounter('post_auth_account_create');

  // TODO: Define schema object for request.
  const accountRequest: any = req.body;

  // TODO: This method needs to require a signed message corresponding to the address indicated,
  // with a nonce or other challenge to avoid replay.

  const address = accountRequest.address;

  let account = accountStore.get(address);
  if (!account) {
    account = new Account(address);
    accountStore.put(account);
  }

  let token = tokenStore.get(account.address);
  if (!token) {
    // Create a valid, but empty token.
    token = new AuthToken(account, 0n, 0n);
    tokenStore.put(token);
  }

  res.code(201);
  return token;
});

fastify.get('/pay/address',
  {
    schema: {
      response: {
        '2xx': {
          address: { type: 'string' },
          multiaddrs: { type: 'array', items: {type: 'string'} },
          contractAddresses: {
            nitroAdjudicatorAddress: { type: 'string' },
            virtualPaymentAppAddress: { type: 'string' },
            consensusAppAddress: { type: 'string' },
          }
        }
      }
    }
  },
  async (req: any, res: any) => {
    metrics.incCounter('get_pay_address');
    const address = await nitro.GetAddress();

    const peerId = await nitro.GetPeerId();

    return {
      address,
      multiaddrs: [
        `/ip4/${Config.NITRO_WS_MSG_PUBLIC_IP}/tcp/${Config.NITRO_WS_MSG_PUBLIC_PORT}/ws/p2p/${peerId}`
      ],
      contractAddresses: {
        nitroAdjudicatorAddress: Config.NITRO_CONTRACT_NA_ADDRESS,
        virtualPaymentAppAddress: Config.NITRO_CONTRACT_VPA_ADDRESS,
        consensusAppAddress: Config.NITRO_CONTRACT_CA_ADDRESS
      }
    };
  }
);

fastify.post('/pay/coupon/:token', authTokenSchema, async (req: any, res: any) => {
  metrics.incCounter('post_pay_coupon');

  const token: AuthToken = tokenStore.get(req.params.token);
  const body: any = req.body;

  if (typeof body === 'string') {
    // This _is_ the coupon.
  } else {
    // This is a request for us to get a coupon.
    // Choose the coupon provider.
    // Request a coupon from them.
  }

  // Check that the coupon was issued for this account address and for this provider.
  // Redeem the coupon (in whole or part).
  // Apply the increase to the token.
  // If the coupon is not fully cashed, associate it with the token so we can keep redeeming it.

  metrics.incCounter(`post_pay_coupon_${token.account.address}`);
  return token;
});

fastify.post('/pay/receive/:token', authTokenSchema, async (req: any, res: any) => {
  metrics.incCounter('post_pay_receive');

  const token = tokenStore.get(req.params.token);
  if (!token) {
    res.code(401);
    metrics.incCounter('pay_receive_401');
    return '401 Unauthorized';
  }

  const voucher: Voucher = req.body;
  let result;

  try {
    result = await nitro.ReceiveVoucher(voucher);
  } catch (e) {
    console.error(e);
    res.status(400).send();
    return;
  }

  token.updateTotal(result.Total);

  metrics.incCounter(`post_pay_receive_${token.account.address}`);
  return token;
});

fastify.post('/', async (req, res) => {
  const body = req.body as any;
  const method = body?.method ?? 'NONE';
  const allowed = Config.GETH_FREE_METHODS.includes(method);

  if (!allowed) {
    log.info(`Rejecting { "method": "${body.method}", "id": ${body.id}, ... } to ${Config.GETH_HTTP_URL}`);
    res.code(401);
    return '401 Unauthorized';
  }

  log.info(`Proxying { "method": "${body.method}", "id": ${body.id}, ... } to ${Config.GETH_HTTP_URL}`);

  const response = await fetch(Config.GETH_HTTP_URL, {
    method: 'post',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(body),
  });

  return response.json();
});

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
