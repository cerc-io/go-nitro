import cors from '@fastify/cors';
import {NitroRpcClient} from '@statechannels/nitro-rpc-client';
import {Voucher} from '@statechannels/nitro-rpc-client/src/types.js';
import crypto from 'crypto';
import Fastify from 'fastify';

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

const genToken = () => {
  return crypto.randomBytes(48).toString('base64url');
};


class AuthToken {
  private _channel: string;
  public _value: string;
  public _balance: bigint;

  constructor(channel: string, balance = 0n, value?: string) {
    this._channel = channel;
    this._balance = balance;
    this._value = value ?? genToken();
  }

  public add(delta: bigint) {
    this._balance += delta;
    return this._balance;
  }

  public sub(delta: bigint) {
    this._balance -= delta;
    return this._balance;
  }

  public checkedSub(delta: bigint) {
    if (delta <= this.balance) {
      this.sub(delta);
      return true;
    }

    return false;
  }

  get balance() {
    return this._balance;
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
      balance: Number(this.balance),
      channel: this.channel
    };
  }
}

const nitro = await NitroRpcClient.CreateHttpNitroClient(`${Config.RPC_HOST}:${Config.RPC_PORT}/api/v1`, Config.RPC_SECURE);

const tokenByChannel = new Map<string, AuthToken>();
const tokenByValue = new Map<string, AuthToken>();

fastify.get('/auth/:token', async (req: any, res: any) => {
  const token = tokenByValue.get(req.params.token);
  if (token && token.checkedSub(1n)) {
    return token;
  } 
  res.code(401);
  
});

fastify.get('/pay/address', async (req: any, res: any) => {
  const result = await nitro.GetAddress();
  return result;
});

fastify.post('/pay/receive', async (req: any, res: any) => {
  const voucher: Voucher = req.body;
  const result = await nitro.ReceiveVoucher(voucher);
  if (result.Delta <= 0n) {
    res.status(400).send();
  }

  let token = tokenByChannel.get(voucher.ChannelId);
  if (!token) {
    token = new AuthToken(voucher.ChannelId);
    tokenByChannel.set(voucher.ChannelId, token);
    tokenByValue.set(token.value, token);
  }
  token.add(result.Delta);

  return token;
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
