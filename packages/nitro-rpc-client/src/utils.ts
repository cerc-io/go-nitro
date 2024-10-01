import JSONbig from "json-bigint";

import { NitroRpcClient } from "./rpc-client";
import {
  AssetData,
  LedgerChannelInfo,
  Outcome,
  PaymentChannelInfo,
  RequestMethod,
  RPCRequestAndResponses,
  SwapAssetsData,
} from "./types";

export const RPC_PATH = "api/v1";

/**
 * createOutcome creates a basic outcome for a channel
 *
 * @param asset - The asset to fund the channel with
 * @param alpha - The address of the first participant
 * @param beta - The address of the second participant
 * @param amount - The amount to allocate to each participant
 * @returns An outcome for a directly funded channel with 100 wei allocated to each participant
 */
export function createOutcome(
  alpha: string,
  beta: string,
  assetData: AssetData[]
): Outcome {
  return assetData.map((asset) => {
    return {
      Asset: asset.assetAddress,
      AssetMetadata: {
        AssetType: 0,
        Metadata: null,
      },

      Allocations: [
        {
          Destination: convertAddressToBytes32(alpha),
          Amount: asset.alphaAmount,
          AllocationType: 0,
          Metadata: null,
        },
        {
          Destination: convertAddressToBytes32(beta),
          Amount: asset.betaAmount,
          AllocationType: 0,
          Metadata: null,
        },
      ],
    };
  });
}

/**
 * Left pads a 20 byte address hex string with zeros until it is a 32 byte hex string
 * e.g.,
 * 0x9546E319878D2ca7a21b481F873681DF344E0Df8 becomes
 * 0x0000000000000000000000009546E319878D2ca7a21b481F873681DF344E0Df8
 *
 * @param address - 20 byte hex string
 * @returns 32 byte padded hex string
 */
export function convertAddressToBytes32(address: string): string {
  const digits = address.startsWith("0x") ? address.substring(2) : address;
  return `0x${digits.padStart(24, "0")}`;
}

/**
 * generateRequest is a helper function that generates a request object for the given method and payloads
 *
 * @param method - The RPC method to generate a request for
 * @param payload - The payloads to include in the request
 * @returns A request object of the correct type
 */
export function generateRequest<
  K extends RequestMethod,
  T extends RPCRequestAndResponses[K][0]
>(method: K, payload: T["params"]["payload"], authToken: string): T {
  return {
    jsonrpc: "2.0",
    method,
    params: { authtoken: authToken, payload: payload },
    // Our schema defines id as a uint32. We mod the current time to ensure that we don't overflow
    id: Date.now() % 1_000_000_000,
  } as T; // TODO: We shouldn't have to cast here
}

export function getLocalRPCUrl(port: number): string {
  return getRPCUrl("127.0.0.1", port);
}

export function getRPCUrl(host: string, port: number): string {
  return `${host}:${port}/${RPC_PATH}`;
}

export async function logOutChannelUpdates(rpcClient: NitroRpcClient) {
  const shortAddress = (await rpcClient.GetAddress()).slice(0, 8);

  rpcClient.Notifications.on(
    "ledger_channel_updated",
    (info: LedgerChannelInfo) => {
      console.log(
        `${shortAddress}: Ledger channel update\n${prettyJson(info)}`
      );
    }
  );
  rpcClient.Notifications.on(
    "payment_channel_updated",
    (info: PaymentChannelInfo) => {
      console.log(
        `${shortAddress}: Payment channel update\n${prettyJson(info)}`
      );
    }
  );
}

export function prettyJson(obj: unknown): string {
  return JSONbig.stringify(obj, null, 2);
}

export function compactJson(obj: unknown): string {
  return JSONbig.stringify(obj, null, 0);
}

export function parseAssetData(assets: string[]): AssetData[] {
  return assets.map((entry) => {
    const [addressPart, amountsPart] = entry.split(":");
    if (!addressPart || !amountsPart) {
      throw new Error(`Invalid format for asset entry: ${entry}`);
    }

    const [alphaAmount, betaAmount] = amountsPart.split(",").map(Number);
    if (isNaN(alphaAmount) || isNaN(betaAmount)) {
      throw new Error(`Invalid alpha or beta amounts in: ${entry}`);
    }

    return {
      assetAddress: addressPart,
      alphaAmount,
      betaAmount,
    };
  });
}

export function parseSwapAssetsData(
  assetIn: string,
  assetOut: string
): SwapAssetsData {
  const [tokenIn, amountIn] = assetIn.split(":");
  if (!tokenIn || !amountIn) {
    throw new Error(`Invalid format for asset entry: ${assetIn}`);
  }

  const [tokenOut, amountOut] = assetOut.split(":");
  if (!tokenOut || !amountOut) {
    throw new Error(`Invalid format for asset entry: ${assetOut}`);
  }

  return {
    TokenIn: tokenIn,
    TokenOut: tokenOut,
    AmountIn: Number(amountIn),
    AmountOut: Number(amountOut),
  };
}
