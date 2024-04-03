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
  amount: number,
  setToken: (p: any | null) => void
) {
  if (nitroClient && paymentChannel) {
    const voucher = await nitroClient.CreateVoucher(paymentChannel.ID, amount);
    const response = await fetch(`${targetUrl}/pay/receive`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(voucher),
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
        response.text().then((v) => {
          setTheirNitroAddress(v);
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
        <div id="my-server" className="info-line">
          My Nitro Server:{" "}
          <input
            type="text"
            onChange={(e) => setMyNitroRpcUrl(e.target.value)}
            value={myNitroRpcUrl?.toString()}
          />
        </div>
        <div id="my-address" className="info-line">
          My Nitro Address: {myNitroAddress}
        </div>
        <div id="target-server" className="info-line">
          Their ETH/Payment Server:{" "}
          <input
            type="text"
            onChange={(e) => setTargetServerUrl(e.target.value)}
            value={targetServerUrl?.toString()}
          />
        </div>
        <div id="their-address" className="info-line">
          Their Nitro Address: {theirNitroAddress}
        </div>
        <div id="ledger-channel" className="info-line">
          Ledger Channel:{" "}
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
                setCreatingPaymentChannel(true);
                nitroClient!.CreateLedgerChannel(theirNitroAddress, 100_000);
              }}
              disabled={
                creatingPaymentChannel || !myNitroAddress || !theirNitroAddress
              }
            >
              {creatingPaymentChannel ? "Please wait ..." : "Create"}
            </button>
          )}
        </div>
        <div id="ledger-balance" className="info-line">
          Ledger Balance:{" "}
          {focusedLedgerChannel
            ? `${focusedLedgerChannel.Balance.TheirBalance} / ${focusedLedgerChannel.Balance.MyBalance}`
            : ""}
        </div>
        <div id="payment-channel" className="info-line">
          Payment Channel:{" "}
          {focusedPaymentChannel ? (
            <span>
              {focusedPaymentChannel.ID}{" "}
              <button
                onClick={() =>
                  nitroClient!.ClosePaymentChannel(focusedPaymentChannel.ID)
                }
              >
                Close
              </button>
            </span>
          ) : (
            <button
              onClick={() => {
                setCreatingLedgerChannel(true);
                nitroClient!.CreatePaymentChannel(
                  theirNitroAddress,
                  [],
                  Number(100)
                );
              }}
              disabled={creatingLedgerChannel || !focusedLedgerChannel}
            >
              {creatingLedgerChannel ? "Please wait ..." : "Create"}
            </button>
          )}
        </div>
        <div id="payment-balance" className="info-line">
          Channel Balance:{" "}
          {focusedPaymentChannel
            ? `${focusedPaymentChannel.Balance.PaidSoFar} / ${focusedPaymentChannel.Balance.RemainingFunds}`
            : ""}
        </div>
        <div id="payment-balance" className="info-line">
          Token: {token && `${token.token} (balance ${token.balance})`}
        </div>
      </div>
      <div id="mid-group">
        <table width="100%">
          <tbody>
            <tr>
              <td>
                <textarea
                  id="api-send"
                  defaultValue={JSON.stringify(
                    {
                      jsonrpc: "2.0",
                      id: 42,
                      method: "eth_blockNumber",
                      params: [],
                    },
                    null,
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
                    if (token.balance > 0) {
                      token.balance -= 1;
                      setToken({ ...token });
                    }
                  }}
                >
                  Send Request
                </button>
                {focusedPaymentChannel && (
                  <button
                    onClick={() => {
                      pay(
                        nitroClient,
                        targetServerUrl,
                        focusedPaymentChannel,
                        10,
                        setToken
                      ).then(() => updateEverything());
                    }}
                  >
                    {token ? "Renew Token" : "Obtain Token"}
                  </button>
                )}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </>
  );
}

export default App;
