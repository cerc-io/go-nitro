#!/usr/bin/env ts-node
/* eslint-disable @typescript-eslint/no-empty-function */
/* eslint-disable @typescript-eslint/no-shadow */

import * as fs from "fs";

import yargs from "yargs/yargs";
import { hideBin } from "yargs/helpers";

import { NitroRpcClient } from "./rpc-client";
import { compactJson, getRPCUrl, logOutChannelUpdates } from "./utils";
import { CounterChallengeAction } from "./types";
import { ZERO_ETHEREUM_ADDRESS } from "./constants";

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
    h: { alias: "host", default: "127.0.0.1", type: "string" },
  })
  .command(
    "version",
    "Get the version of the Nitro RPC server",
    async () => {},
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
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

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
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

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
      );
      const ledgers = await rpcClient.GetAllLedgerChannels();
      for (const ledger of ledgers) {
        console.log(`${compactJson(ledger)}`);
      }
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "get-all-l2-channels",
    "Get all L2 channels",
    async () => {},
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
      );
      const ledgers = await rpcClient.GetAllL2Channels();
      for (const ledger of ledgers) {
        console.log(`${compactJson(ledger)}`);
      }
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "get-l2-signed-state <channelId> <jsonFilePath>",
    "Get latest L2 signed state",
    (yargsBuilder) => {
      return yargsBuilder
        .positional("channelId", {
          describe: "The channel ID of the ledger channel",
          type: "string",
          demandOption: true,
        })
        .positional("jsonFilePath", {
          describe: "Path to JSON file for saving L2 signed state",
          type: "string",
          demandOption: true,
        });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;
      const channelId = yargs.channelId;
      const jsonFilePath = yargs.jsonFilePath;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
      );
      const l2SignedState = await rpcClient.GetL2SignedState(channelId);
      console.log(`${compactJson(l2SignedState)}`);

      fs.writeFile(
        jsonFilePath,
        JSON.stringify(l2SignedState, null, 2),
        "utf8",
        (err) => {
          if (err) {
            console.error("Error writing file:", err);
          } else {
            console.log("File has been saved.");
          }
        }
      );

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

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
      );
      const paymentChans = await rpcClient.GetPaymentChannelsByLedger(
        yargs.ledgerId
      );
      for (const p of paymentChans) {
        console.log(`${compactJson(p)}`);
      }
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
        .option("assetAddress", {
          describe: "Address of the token to be used",
          type: "string",
          default: ZERO_ETHEREUM_ADDRESS,
        })
        .option("alphaAmount", {
          describe: "The amount to be funded by alpha node",
          type: "number",
          default: 1_000_000,
        })
        .option("betaAmount", {
          describe: "The amount to be funded by beta node",
          type: "number",
          default: 1_000_000,
        });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
      );
      if (yargs.n) logOutChannelUpdates(rpcClient);

      const dfObjective = await rpcClient.CreateLedgerChannel(
        yargs.counterparty,
        yargs.assetAddress,
        yargs.alphaAmount,
        yargs.betaAmount
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
      return yargsBuilder
        .positional("channelId", {
          describe: "The id of the ledger channel to defund",
          type: "string",
          demandOption: true,
        })
        .option("isChallenge", {
          describe: "To initiate challenge transaction",
          type: "boolean",
          default: false,
        });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
      );
      if (yargs.n) logOutChannelUpdates(rpcClient);

      const id = await rpcClient.CloseLedgerChannel(
        yargs.channelId,
        yargs.isChallenge
      );
      console.log(`Objective started ${id}`);
      // Not using WaitForLedgerChannelStatus method with complete status, as the ledger channel status will be open if a challenge is cleared using checkpoint
      await rpcClient.WaitForObjectiveToComplete(
        `DirectDefunding-${yargs.channelId}`
      );
      console.log(`Objective Complete ${yargs.channelId}`);
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "bridged-defund <channelId>",
    "Defunds a mirror ledger channel",
    (yargsBuilder) => {
      return yargsBuilder.positional("channelId", {
        describe: "The id of mirror ledger channel to defund",
        type: "string",
        demandOption: true,
      });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
      );
      if (yargs.n) logOutChannelUpdates(rpcClient);

      const id = await rpcClient.CloseBridgeChannel(yargs.channelId);
      console.log(`Objective started ${id}`);
      await rpcClient.WaitForObjectiveToComplete(
        `bridgeddefunding-${yargs.channelId}`
      );
      console.log(`Objective Complete ${yargs.channelId}`);
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

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
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

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
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

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
      );

      const ledgerInfo = await rpcClient.GetLedgerChannel(yargs.channelId);
      console.log(ledgerInfo);
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

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
      );
      const paymentChannelInfo = await rpcClient.GetPaymentChannel(
        yargs.channelId
      );
      console.log(paymentChannelInfo);
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

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
      );
      if (yargs.n) logOutChannelUpdates(rpcClient);

      const paymentChannelInfo = await rpcClient.Pay(
        yargs.channelId,
        yargs.amount
      );
      console.log(paymentChannelInfo);
      await rpcClient.Close();
      process.exit(0);
    }
  )
  .command(
    "counter-challenge <channelId> <action>",
    "Counter challenge the registered challenge",
    (yargsBuilder) => {
      return yargsBuilder
        .positional("channelId", {
          describe: "The channel ID of the payment channel",
          type: "string",
          demandOption: true,
        })
        .positional("action", {
          describe: "The action to take",
          type: "string",
          choices: ["checkpoint", "challenge"],
          demandOption: true,
        });
    },
    async (yargs) => {
      const rpcPort = yargs.p;
      const rpcHost = yargs.h;

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
      );
      if (yargs.n) logOutChannelUpdates(rpcClient);

      const response = await rpcClient.CounterChallenge(
        yargs.channelId,
        CounterChallengeAction[
          yargs.action as keyof typeof CounterChallengeAction
        ]
      );
      console.log(
        `Sending ${response.Action} transaction for channel ${response.ChannelId}`
      );
      await rpcClient.WaitForObjectiveToComplete(
        `DirectDefunding-${yargs.channelId}`
      );
      console.log(`Objective Complete ${response.ChannelId}`);
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

      const rpcClient = await NitroRpcClient.CreateHttpNitroClient(
        getRPCUrl(rpcHost, rpcPort)
      );
      if (yargs.n) logOutChannelUpdates(rpcClient);

      const voucher = await rpcClient.CreateVoucher(
        yargs.channelId,
        yargs.amount
      );
      console.log(voucher);
      await rpcClient.Close();
      process.exit(0);
    }
  )

  .demandCommand(1, "You need at least one command before moving on")
  .parserConfiguration({ "parse-numbers": false })
  .strict()
  .parse();
