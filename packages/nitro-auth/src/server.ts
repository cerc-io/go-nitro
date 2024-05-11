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


class AuthToken {
  private _channel: string;
  public _value: string;
  public _used: bigint;
  public _total: bigint;

  constructor(channel: string, total = 0n, used = 0n, value?: string) {
    this._channel = channel;
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

  get channel() {
    return this._channel;
  }

  toJSON() {
    return {
      token: this.value,
      total: this._total,
      used: this._used,
      channel: this.channel
    };
  }
}

const tokenByChannel = new Map<string, AuthToken>();
const tokenByValue = new Map<string, AuthToken>();
const nitro = await NitroRpcClient.CreateHttpNitroClient(`${Config.NITRO_RPC_HOST}:${Config.NITRO_RPC_PORT}/api/v1`, Config.NITRO_RPC_SECURE);
const authTokenSchema = {
  schema: {
    response: {
      '2xx': {
        token: { type: 'string' },
        total: { type: 'number' },
        used: { type: 'number' },
        channel: { type: 'string' },
      }
    }
  },
  validatorCompiler: () => () => true,
};

fastify.get('/auth/:token', authTokenSchema, async (req: any, res: any) => {
  metrics.incCounter('get_auth_token');
  const token = tokenByValue.get(req.params.token);
  if (token) {
    metrics.incCounter(`get_auth_${token.channel}`);
    if (token.use(1n)) {
      return token;
    }
    res.code(402);
    return '402 Payment Required';

  }
  res.code(401);
  return '401 Unauthorized';
});

fastify.get('/pay/address',
  {
    schema: {
      response: {
        '2xx': {
          address: { type: 'string' },
          multiaddrs: { type: 'array', items: {type: 'string'} },
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
      ]
    };
  }
);

fastify.post('/pay/receive', authTokenSchema, async (req: any, res: any) => {
  metrics.incCounter('post_pay_receive');
  const voucher: Voucher = req.body;
  let result;

  try {
    result = await nitro.ReceiveVoucher(voucher);
  } catch (e) {
    console.error(e);
    res.status(400).send();
    return;
  }

  let token = tokenByChannel.get(voucher.ChannelId);
  if (!token) {
    token = new AuthToken(voucher.ChannelId, result.Total);
    tokenByChannel.set(voucher.ChannelId, token);
    tokenByValue.set(token.value, token);
  } else {
    token.updateTotal(result.Total);
  }

  metrics.incCounter(`post_pay_receive_${token.channel}`);
  return token;
});

fastify.post('/', async (req, res) => {
  const body = req.body as any;
  const method = body?.method ?? 'NONE';
  let allowed = false;

  switch (method) {
  case 'eth_chainId':
  case 'eth_blockNumber':
    allowed = true;
    break;
  case 'eth_call':
    // For the moment, all calls.
    allowed = true;
    break;
  default:
    allowed = false;
  }

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
