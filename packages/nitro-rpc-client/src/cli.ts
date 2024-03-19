#!/usr/bin/env ts-node
/* eslint-disable @typescript-eslint/no-empty-function */
/* eslint-disable @typescript-eslint/no-shadow */
/* eslint-disable @typescript-eslint/no-explicit-any */

import yargs from "yargs/yargs";
import { hideBin } from "yargs/helpers";

import { NitroRpcClient } from "./rpc-client.js";
import { compactJson, getCustomRPCUrl, logOutChannelUpdates } from "./utils.js";

(BigInt.prototype as any).toJSON = function () {
  return this.toString();
};

yargs(hideBin(process.argv))
  .scriptName("nitro-rpc-client")
  .option({
    p: { alias: "port", default: 4005, type: "number" },
    n: {
      alias: "printnotifications",
      default: false,
      type: "boolean",
      description: "Whether channel notifications are printed to the console",
    },
    h: {
      alias: "host",
      default: "127.0.0.1",
      type: "string",
      description: "Custom hostname",
    },
    s: {
      alias: "isSecure",
      default: true,
      type: "boolean",
      description: "Is it a secured connection",
    },
  })
  .command(
    "version",
    "Get the version of the Nitro RPC server",
    async () => {},
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );
      const version = await rpcClient.GetVersion();
      console.log(version);
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "address",
    "Get the address of the Nitro RPC server",
    async () => {},
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );
      const address = await rpcClient.GetAddress();
      console.log(address);
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "get-all-ledger-channels",
    "Get all ledger channels",
    async () => {},
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );
      const ledgers = await rpcClient.GetAllLedgerChannels();
      console.log(`${compactJson(ledgers)}`);
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "get-payment-channels-by-ledger <ledgerId>",
    "Gets any payment channels funded by the given ledger",
    (yargsBuilder) => {
      return yargsBuilder.positional("ledgerId", {
        describe: "The id of the ledger channel to defund",
        type: "string",
        demandOption: true,
      });
    },

    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );
      const paymentChans = await rpcClient.GetPaymentChannelsByLedger(
        yargs.ledgerId
      );
      console.log(`${compactJson(paymentChans)}`);
      await rpcClient.Close();
      process.exit(0);
    }
  )

  .command(
    "direct-fund <counterparty>",
    "Creates a directly funded ledger channel",
    (yargsBuilder) => {
      return yargsBuilder
        .positional("counterparty", {
          describe: "The counterparty's address",
          type: "string",
          demandOption: true,
        })
        .option("amount", {
          describe: "The amount to fund the channel with",
          type: "number",
          default: 1_000_000,
        });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );
      if (yargs.n) logOutChannelUpdates(rpcClient);

      const dfObjective = await rpcClient.CreateLedgerChannel(
        yargs.counterparty,
        yargs.amount
      );
      const { Id, ChannelId } = dfObjective;

      console.log(`Objective started ${Id}`);
      await rpcClient.WaitForLedgerChannelStatus(ChannelId, "Open");
      console.log(`Channel Open ${ChannelId}`);
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "direct-defund <channelId>",
    "Defunds a directly funded ledger channel",
    (yargsBuilder) => {
      return yargsBuilder.positional("channelId", {
        describe: "The id of the ledger channel to defund",
        type: "string",
        demandOption: true,
      });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );
      if (yargs.n) logOutChannelUpdates(rpcClient);

      const id = await rpcClient.CloseLedgerChannel(yargs.channelId);
      console.log(`Objective started ${id}`);
      await rpcClient.WaitForPaymentChannelStatus(yargs.channelId, "Complete");
      console.log(`Channel Complete ${yargs.channelId}`);
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "virtual-fund <counterparty> [intermediaries...]",
    "Creates a virtually funded payment channel",
    (yargsBuilder) => {
      return yargsBuilder
        .positional("counterparty", {
          describe: "The counterparty's address",
          type: "string",
          demandOption: true,
        })
        .array("intermediaries")
        .option("amount", {
          describe: "The amount to fund the channel with",
          type: "number",
          default: 1000,
        });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );
      if (yargs.n) logOutChannelUpdates(rpcClient);

      // Parse all intermediary args to strings
      const intermediaries =
        yargs.intermediaries?.map((intermediary) => {
          if (typeof intermediary === "string") {
            return intermediary;
          }
          return intermediary.toString(16);
        }) ?? [];

      const vfObjective = await rpcClient.CreatePaymentChannel(
        yargs.counterparty,
        intermediaries,
        yargs.amount
      );

      const { ChannelId, Id } = vfObjective;
      console.log(`Objective started ${Id}`);
      await rpcClient.WaitForPaymentChannelStatus(ChannelId, "Open");
      console.log(`Channel Open ${ChannelId}`);
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "virtual-defund <channelId>",
    "Defunds a virtually funded payment channel",
    (yargsBuilder) => {
      return yargsBuilder.positional("channelId", {
        describe: "The id of the payment channel to defund",
        type: "string",
        demandOption: true,
      });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );

      if (yargs.n) logOutChannelUpdates(rpcClient);

      const id = await rpcClient.ClosePaymentChannel(yargs.channelId);

      console.log(`Objective started ${id}`);
      await rpcClient.WaitForPaymentChannelStatus(yargs.channelId, "Complete");
      console.log(`Channel complete ${yargs.channelId}`);
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "get-ledger-channel <channelId>",
    "Gets information about a ledger channel",
    (yargsBuilder) => {
      return yargsBuilder.positional("channelId", {
        describe: "The channel ID of the ledger channel",
        type: "string",
        demandOption: true,
      });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );

      const ledgerInfo = await rpcClient.GetLedgerChannel(yargs.channelId);
      console.log(compactJson(ledgerInfo));
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "get-payment-channel <channelId>",
    "Gets information about a payment channel",
    (yargsBuilder) => {
      return yargsBuilder.positional("channelId", {
        describe: "The channel ID of the payment channel",
        type: "string",
        demandOption: true,
      });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );
      const paymentChannelInfo = await rpcClient.GetPaymentChannel(
        yargs.channelId
      );
      console.log(compactJson(paymentChannelInfo));
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "pay <channelId> <amount>",
    "Sends a payment on the given channel",
    (yargsBuilder) => {
      return yargsBuilder
        .positional("channelId", {
          describe: "The channel ID of the payment channel",
          type: "string",
          demandOption: true,
        })
        .positional("amount", {
          describe: "The amount to pay",
          type: "number",
          demandOption: true,
        });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );
      if (yargs.n) logOutChannelUpdates(rpcClient);

      const paymentChannelInfo = await rpcClient.Pay(
        yargs.channelId,
        yargs.amount
      );
      console.log(compactJson(paymentChannelInfo));
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "create-voucher <channelId> <amount>",
    "Create a payment on the given channel",
    (yargsBuilder) => {
      return yargsBuilder
        .positional("channelId", {
          describe: "The channel ID of the payment channel",
          type: "string",
          demandOption: true,
        })
        .positional("amount", {
          describe: "The amount to pay",
          type: "number",
          demandOption: true,
        });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );
      if (yargs.n) logOutChannelUpdates(rpcClient);

      const voucher = await rpcClient.CreateVoucher(
        yargs.channelId,
        yargs.amount
      );
      console.log(compactJson(voucher));
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "receive-voucher <voucher>",
    "Receive a payment voucher",
    (yargsBuilder) => {
      return yargsBuilder.positional("voucher", {
        describe: "voucher JSON",
        type: "string",
        demandOption: true,
      });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const isSecure = yargs.s;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getCustomRPCUrl(rpcHost, rpcPort),
        isSecure
      );
      if (yargs.n) logOutChannelUpdates(rpcClient);

      const voucher = JSON.parse(yargs.voucher);
      const result = await rpcClient.ReceiveVoucher(voucher);
      console.log(compactJson(result));
      await rpcClient.Close();
      process.exit(0);
    }
  )

  .demandCommand(1, "You need at least one command before moving on")
  .parserConfiguration({ "parse-numbers": false })
  .strict()
  .parse();
