export const Config = {
  LISTEN_PORT: parseInt(process.env.CERC_NITRO_AUTH_LISTEN_PORT || '8547'),
  LISTEN_ADDR: process.env.CERC_NITRO_AUTH_LISTEN_ADDR || '0.0.0.0',
  RPC_HOST: process.env.CERC_NITRO_AUTH_RPC_HOST || '127.0.0.1',
  RPC_PORT: process.env.CERC_NITRO_AUTH_RPC_PORT || '4006',
  RPC_SECURE: 'true' === (process.env.CERC_NITRO_RPC_SECURE || 'false'),
  HTTP_SERVER_KEEPALIVE_TIMEOUT: parseInt(process.env.CERC_NITRO_HTTP_SERVER_KEEPALIVE_TIMEOUT || '60000'),
  HTTP_SERVER_TIMEOUT: parseInt(process.env.CERC_NITRO_HTTP_SERVER_TIMEOUT || '30000'),
  HTTP_SERVER_BACKLOG: parseInt(process.env.CERC_NITRO_HTTP_SERVER_BACKLOG || '10000'),
  FASTIFY_LOGGER: 'true' === (process.env.CERC_NITRO_FASTIFY_LOGGER || 'true'),
};
