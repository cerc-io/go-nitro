import JSONBig from "json-bigint";
import { useEffect, useState } from "react";
import { NitroRpcClient } from "@statechannels/nitro-rpc-client";
import {
  LedgerChannelInfo,
  PaymentChannelInfo,
} from "@statechannels/nitro-rpc-client/src/types";

import "./App.css";

async function updateChannels(
  nitroClient: NitroRpcClient,
  setFocusedLedgerChannel: (l: LedgerChannelInfo | null) => void,
  setFocusedPaymentChannel: (p: PaymentChannelInfo | null) => void,
  setCreatingLedgerChannel: (v: boolean) => void,
  setCreatingPaymentChannel: (v: boolean) => void
) {
  if (!nitroClient) {
    return;
  }
  const ledgerChannels = (await nitroClient.GetAllLedgerChannels()).filter(
    (lc) => lc.Status == "Open"
  );
  const paymentChannels = new Map<string, PaymentChannelInfo[]>();

  let focusedLedgerChannel: LedgerChannelInfo | null = null;
  let focusedPaymentChannel: PaymentChannelInfo | null = null;

  for (const lc of ledgerChannels) {
    const pcs = (await nitroClient.GetPaymentChannelsByLedger(lc.ID)).filter(
      (pc) => pc.Status == "Open"
    );
    paymentChannels.set(lc.ID, pcs);
    for (const pc of pcs) {
      if (
        null == focusedPaymentChannel ||
        pc.Balance.RemainingFunds > focusedPaymentChannel.Balance.RemainingFunds
      ) {
        focusedLedgerChannel = lc;
        focusedPaymentChannel = pc;
      }
    }
  }

  if (!focusedLedgerChannel && ledgerChannels.length) {
    focusedLedgerChannel = ledgerChannels[0];
  }

  setFocusedPaymentChannel(focusedPaymentChannel);
  if (focusedPaymentChannel) {
    setCreatingPaymentChannel(false);
  }
  setFocusedLedgerChannel(focusedLedgerChannel);
  if (focusedLedgerChannel) {
    setCreatingLedgerChannel(false);
  }
}

async function pay(
  nitroClient: NitroRpcClient | null,
  targetUrl: string,
  paymentChannel: PaymentChannelInfo | null,
  amount: bigint,
  setToken: (p: any | null) => void
) {
  if (nitroClient && paymentChannel) {
    const voucher = await nitroClient.CreateVoucher(paymentChannel.ID, amount);
    const response = await fetch(`${targetUrl}/pay/receive`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSONBig.stringify(voucher),
    });
    const token = await response.json();
    setToken(token);
  }
}

function getRpcUrl(rpcUrl?: string): string {
  if (rpcUrl) {
    return rpcUrl ?? "";
  } else if (import.meta.env.VITE_RPC_URL) {
    return import.meta.env.VITE_RPC_URL;
  }
  return "http://localhost:4006";
}

function getTargetUrl(targetUrl?: string): string {
  if (targetUrl) {
    return targetUrl ?? "";
  } else if (import.meta.env.VITE_TARGET_URL) {
    return import.meta.env.VITE_TARGET_URL;
  }

  return "http://localhost:5678";
}

async function send(url: string): Promise<any> {
  try {
    const fromEl = document.getElementById("api-send");
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: fromEl!.value,
    });

    const text = await response.text();
    const recvEl = document.getElementById("api-recv");
    recvEl.value = text;
  } catch (e) {
    const recvEl = document.getElementById("api-recv");
    recvEl.value = e;
  }
}

function App() {
  const [nitroClient, setNitroClient] = useState<NitroRpcClient | null>(null);
  const [targetServerUrl, setTargetServerUrl] = useState<string>(
    getTargetUrl()
  );
  const [myNitroRpcUrl, setMyNitroRpcUrl] = useState<string>(getRpcUrl());
  const [myNitroAddress, setMyNitroAddress] = useState<string>("");
  const [theirNitroAddress, setTheirNitroAddress] = useState<string>("");
  const [focusedLedgerChannel, setFocusedLedgerChannel] =
    useState<LedgerChannelInfo | null>(null);
  const [focusedPaymentChannel, setFocusedPaymentChannel] =
    useState<PaymentChannelInfo | null>(null);
  const [token, setToken] = useState<any>(null);
  const [creatingLedgerChannel, setCreatingLedgerChannel] =
    useState<boolean>(false);
  const [creatingPaymentChannel, setCreatingPaymentChannel] =
    useState<boolean>(false);

  let updateEverything = async () => {};
  let updateInterval: NodeJS.Timeout | undefined;

  useEffect(() => {
    const delayDebounceFn = setTimeout(() => {
      setFocusedPaymentChannel(null);
      setFocusedLedgerChannel(null);
      setMyNitroAddress("");
      const nitroUrl = new URL(myNitroRpcUrl);
      NitroRpcClient.CreateHttpNitroClient(
        `${nitroUrl.hostname}:${nitroUrl.port}/api/v1`,
        nitroUrl?.protocol == "https"
      ).then((c) => {
        setNitroClient(c);
        updateEverything = async () =>
          updateChannels(
            c,
            setFocusedLedgerChannel,
            setFocusedPaymentChannel,
            setCreatingLedgerChannel,
            setCreatingPaymentChannel
          );
        if (updateInterval) {
          clearInterval(updateInterval);
        }
        updateInterval = setInterval(updateEverything, 1000);
      });
    }, 1000);

    return () => clearTimeout(delayDebounceFn);
  }, [myNitroRpcUrl]);

  useEffect(() => {
    if (nitroClient) {
      nitroClient.GetAddress().then((a) => setMyNitroAddress(a));
      updateEverything();
      nitroClient.Notifications.on("objective_completed", updateEverything);
    }
  }, [nitroClient]);

  useEffect(() => {
    const delayDebounceFn = setTimeout(() => {
      setFocusedPaymentChannel(null);
      setFocusedLedgerChannel(null);
      setTheirNitroAddress("");
      fetch(targetServerUrl + "/pay/address").then((response) => {
        response.json().then((v: any) => {
          setTheirNitroAddress(v?.address);
          if (nitroClient) {
            updateEverything();
          }
        });
      });
    }, 1000);

    return () => clearTimeout(delayDebounceFn);
  }, [targetServerUrl]);

  return (
    <>
      <div id="top-group">
        <h2>Nitro Details</h2>
        <table>
          <tbody>
            <tr>
              <td className="key">Consumer Nitro Node</td>
              <td className="value">
                <input
                  type="text"
                  onChange={(e) => setMyNitroRpcUrl(e.target.value)}
                  value={myNitroRpcUrl?.toString()}
                />
              </td>
            </tr>
            <tr>
              <td className="key">Consumer Address</td>
              <td className="value">{myNitroAddress}</td>
            </tr>
            <tr>
              <td className="key">Provider Endpoint</td>
              <td className="value">
                <input
                  type="text"
                  onChange={(e) => setTargetServerUrl(e.target.value)}
                  value={targetServerUrl?.toString()}
                />
              </td>
            </tr>
            <tr>
              <td className="key">Provider Address</td>
              <td className="value">{theirNitroAddress}</td>
            </tr>
            <tr>
              <td className="key">Ledger Channel</td>
              <td className="value">
                {focusedLedgerChannel ? (
                  <span>
                    {focusedLedgerChannel.ID}{" "}
                    <button
                      onClick={() =>
                        nitroClient!.CloseLedgerChannel(focusedLedgerChannel.ID)
                      }
                    >
                      Close
                    </button>
                  </span>
                ) : (
                  <button
                    onClick={() => {
                      setCreatingLedgerChannel(true);
                      nitroClient!.CreateLedgerChannel(
                        theirNitroAddress,
                        100_000n
                      );
                    }}
                    disabled={
                      creatingLedgerChannel ||
                      !myNitroAddress ||
                      !theirNitroAddress
                    }
                  >
                    {creatingLedgerChannel ? "Please wait ..." : "Create"}
                  </button>
                )}
              </td>
            </tr>
            <tr>
              <td className="key">Ledger Balance</td>
              <td className="value">
                {focusedLedgerChannel
                  ? `${focusedLedgerChannel.Balance.TheirBalance} / ${focusedLedgerChannel.Balance.MyBalance}`
                  : ""}
              </td>
            </tr>
            <tr>
              <td className="key">Payment Channel</td>
              <td className="value">
                {focusedPaymentChannel ? (
                  <span>
                    {focusedPaymentChannel.ID}{" "}
                    <button
                      onClick={() =>
                        nitroClient!.ClosePaymentChannel(
                          focusedPaymentChannel.ID
                        )
                      }
                    >
                      Close
                    </button>
                  </span>
                ) : focusedLedgerChannel ? (
                  <button
                    onClick={() => {
                      setCreatingPaymentChannel(true);
                      nitroClient!.CreatePaymentChannel(
                        theirNitroAddress,
                        [],
                        100n
                      );
                    }}
                    disabled={
                      creatingPaymentChannel ||
                      creatingLedgerChannel ||
                      !focusedLedgerChannel
                    }
                  >
                    {creatingPaymentChannel || creatingLedgerChannel
                      ? "Please wait ..."
                      : "Create"}
                  </button>
                ) : (
                  ""
                )}
              </td>
            </tr>
            <tr>
              <td className="key">Channel Balance</td>
              <td className="value">
                {focusedPaymentChannel
                  ? `${focusedPaymentChannel.Balance.PaidSoFar} / ${focusedPaymentChannel.Balance.RemainingFunds}`
                  : ""}
              </td>
            </tr>
            <tr>
              <td className="key">API Token</td>
              <td className="value">
                {token && `${token.token}`}{" "}
                {focusedPaymentChannel && (
                  <button
                    className={
                      token &&
                      (token.used >= token.total ||
                        0n == focusedPaymentChannel.Balance.RemainingFunds)
                        ? "empty"
                        : ""
                    }
                    onClick={() => {
                      pay(
                        nitroClient,
                        targetServerUrl,
                        focusedPaymentChannel,
                        10n,
                        setToken
                      ).then(() => updateEverything());
                    }}
                    disabled={
                      0n == focusedPaymentChannel.Balance.RemainingFunds
                    }
                  >
                    {token ? `Renew (${token.total - token.used})` : "Obtain"}
                  </button>
                )}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div id="mid-group">
        <h2>Ethereum API</h2>
        <table width="100%">
          <tbody>
            <tr>
              <td>
                <textarea
                  id="api-send"
                  defaultValue={JSONBig.stringify(
                    {
                      jsonrpc: "2.0",
                      id: 42,
                      method: "eth_blockNumber",
                      params: [],
                    },
                    undefined,
                    2
                  )}
                />
              </td>
              <td>
                <textarea id="api-recv" contentEditable={false}></textarea>
              </td>
            </tr>
            <tr>
              <td colSpan={2}>
                <button
                  onClick={() => {
                    send(`${targetServerUrl}/eth/${token ? token.token : ""}`);
                    if (token.used < token.total) {
                      token.used += 1;
                      setToken({ ...token });
                    }
                  }}
                >
                  Send Request
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </>
  );
}

export default App;
