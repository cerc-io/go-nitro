import {
  DefundObjectiveRequest,
  DirectFundPayload,
  LedgerChannelInfo,
  PaymentChannelInfo,
  PaymentPayload,
  VirtualFundPayload,
  RequestMethod,
  RPCRequestAndResponses,
  ObjectiveResponse,
  Voucher,
  ReceiveVoucherResult,
  ChannelStatus,
  LedgerChannelUpdatedNotification,
  PaymentChannelUpdatedNotification,
  DirectDefundObjectiveRequest,
  CounterChallengeAction,
  CounterChallengeResult,
  ObjectiveCompleteNotification,
  MirrorBridgedDefundObjectiveRequest,
  GetNodeInfo,
  AssetData,
  SwapFundPayload,
  SwapAssetsData,
  SwapInitiatePayload,
  GetPendingSwap,
  ConfirmSwapAction,
  ConfirmSwapResult,
  SwapChannelInfo,
} from "./types";
import { Transport } from "./transport";
import { createOutcome, generateRequest } from "./utils";
import { HttpTransport } from "./transport/http";
import { getAndValidateResult } from "./serde";
import { RpcClientApi } from "./interface";
import { ZERO_ETHEREUM_ADDRESS } from "./constants";

export class NitroRpcClient implements RpcClientApi {
  private transport: Transport;

  // We fetch the address from the RPC server on first use
  private myAddress: string | undefined;

  private authToken: string | undefined;

  public get Notifications() {
    return this.transport.Notifications;
  }

  public async CreateVoucher(
    channelId: string,
    amount: number
  ): Promise<Voucher> {
    const payload = {
      Amount: amount,
      Channel: channelId,
    };
    const request = generateRequest(
      "create_voucher",
      payload,
      this.authToken || ""
    );
    const res = await this.transport.sendRequest<"create_voucher">(request);
    return getAndValidateResult(res, "create_voucher");
  }

  public async ReceiveVoucher(voucher: Voucher): Promise<ReceiveVoucherResult> {
    const request = generateRequest(
      "receive_voucher",
      voucher,
      this.authToken || ""
    );
    const res = await this.transport.sendRequest<"receive_voucher">(request);
    return getAndValidateResult(res, "receive_voucher");
  }

  public async WaitForObjectiveToComplete(objectiveId: string): Promise<void> {
    const promise = new Promise<void>((resolve) => {
      this.transport.Notifications.on(
        "objective_completed",
        (payload: ObjectiveCompleteNotification["params"]["payload"]) => {
          if (payload === objectiveId) {
            resolve();
          }
        }
      );
    });
    return promise;
  }

  public async WaitForLedgerChannelStatus(
    channelId: string,
    status: ChannelStatus
  ): Promise<void> {
    const promise = new Promise<void>((resolve) => {
      this.transport.Notifications.on(
        "ledger_channel_updated",
        (payload: LedgerChannelUpdatedNotification["params"]["payload"]) => {
          if (payload.ID === channelId) {
            this.GetLedgerChannel(channelId).then((l) => {
              if (l.Status == status) resolve();
            });
          }
        }
      );
    });
    const ledger = await this.GetLedgerChannel(channelId);
    if (ledger.Status == status) return;
    return promise;
  }

  public async WaitForPaymentChannelStatus(
    channelId: string,
    status: ChannelStatus
  ): Promise<void> {
    const promise = new Promise<void>((resolve) => {
      this.transport.Notifications.on(
        "payment_channel_updated",
        (payload: PaymentChannelUpdatedNotification["params"]["payload"]) => {
          if (payload.ID === channelId) {
            this.GetPaymentChannel(channelId).then((l) => {
              console.log(
                "DEBUG: rpc-client.ts-WaitForPaymentChannelStatus payment channel status after payment_channel_updated notification",
                l.Status
              );
              if (l.Status == status) resolve();
            });
          }
        }
      );
    });

    const channel = await this.GetPaymentChannel(channelId);
    if (channel.Status == status) return;
    return promise;
  }

  public onPaymentChannelUpdated(
    channelId: string,
    callback: (info: PaymentChannelInfo) => void
  ): () => void {
    const wrapperFn = (info: PaymentChannelInfo) => {
      if (info.ID.toLowerCase() == channelId.toLowerCase()) {
        callback(info);
      }
    };
    this.transport.Notifications.on("payment_channel_updated", wrapperFn);
    return () => {
      this.transport.Notifications.off("payment_channel_updated", wrapperFn);
    };
  }

  public async CreateLedgerChannel(
    counterParty: string,
    assetsData: AssetData[],
    challengeDuration: number
  ): Promise<ObjectiveResponse> {
    const payload: DirectFundPayload = {
      CounterParty: counterParty,
      ChallengeDuration: challengeDuration,
      Outcome: createOutcome(await this.GetAddress(), counterParty, assetsData),
      AppDefinition: ZERO_ETHEREUM_ADDRESS,
      AppData: "0x00",
      Nonce: Date.now(),
    };
    return this.sendRequest("create_ledger_channel", payload);
  }

  public async RetryObjectiveTx(objectiveId: string): Promise<string> {
    return this.sendRequest("retry_objective_tx", { ObjectiveId: objectiveId });
  }

  public async RetryTx(txHash: string): Promise<string> {
    return this.sendRequest("retry_tx", { TxHash: txHash });
  }

  public async CreateSwapChannel(
    counterParty: string,
    intermediaries: string[],
    assetsData: AssetData[]
  ): Promise<ObjectiveResponse> {
    const payload: SwapFundPayload = {
      CounterParty: counterParty,
      Intermediaries: intermediaries,
      ChallengeDuration: 0,
      Outcome: createOutcome(await this.GetAddress(), counterParty, assetsData),
      AppDefinition: ZERO_ETHEREUM_ADDRESS,
      Nonce: Date.now(),
    };

    return this.sendRequest("create_swap_channel", payload);
  }

  public async SwapAssets(
    channelId: string,
    swapAssetsData: SwapAssetsData
  ): Promise<SwapInitiatePayload> {
    const payload = {
      SwapAssetsData: swapAssetsData,
      Channel: channelId,
    };
    const request = generateRequest(
      "swap_initiate",
      payload,
      this.authToken || ""
    );
    const res = await this.transport.sendRequest<"swap_initiate">(request);
    return getAndValidateResult(res, "swap_initiate");
  }

  public async GetPendingSwap(channelId: string): Promise<string> {
    const payload: GetPendingSwap = {
      Id: channelId,
    };
    return this.sendRequest("get_pending_swap", payload);
  }

  public async GetRecentSwaps(channelId: string): Promise<string> {
    const payload = {
      Id: channelId,
    };
    return this.sendRequest("get_recent_swaps", payload);
  }

  public async CreatePaymentChannel(
    counterParty: string,
    intermediaries: string[],
    amount: number
  ): Promise<ObjectiveResponse> {
    const asset = `0x${"00".repeat(20)}`;
    const payload: VirtualFundPayload = {
      CounterParty: counterParty,
      Intermediaries: intermediaries,
      ChallengeDuration: 0,
      Outcome: createOutcome(await this.GetAddress(), counterParty, [
        {
          assetAddress: asset,
          alphaAmount: amount,
          // As payment channel is simplex, only alpha node can pay beta node and not vice-versa hence beta node's allocation amount is 0
          betaAmount: 0,
        },
      ]),
      AppDefinition: asset,
      Nonce: Date.now(),
    };

    return this.sendRequest("create_payment_channel", payload);
  }

  public async Pay(channelId: string, amount: number): Promise<PaymentPayload> {
    const payload = {
      Amount: amount,
      Channel: channelId,
    };
    const request = generateRequest("pay", payload, this.authToken || "");
    const res = await this.transport.sendRequest<"pay">(request);
    return getAndValidateResult(res, "pay");
  }

  public async GetVoucher(channelId: string): Promise<Voucher> {
    const payload = {
      Id: channelId,
    };
    return this.sendRequest("get_voucher", payload);
  }

  public async CloseLedgerChannel(
    channelId: string,
    isChallenge: boolean
  ): Promise<string> {
    const payload: DirectDefundObjectiveRequest = {
      ChannelId: channelId,
      IsChallenge: isChallenge,
    };
    return this.sendRequest("close_ledger_channel", payload);
  }

  public async CloseBridgeChannel(channelId: string): Promise<string> {
    const payload: DefundObjectiveRequest = {
      ChannelId: channelId,
    };
    return this.sendRequest("close_bridge_channel", payload);
  }

  public async MirrorBridgedDefund(
    channelId: string,
    stringifiedL2SignedState: string,
    isChallenge: boolean
  ): Promise<string> {
    const payload: MirrorBridgedDefundObjectiveRequest = {
      ChannelId: channelId,
      IsChallenge: isChallenge,
      StringifiedL2SignedState: stringifiedL2SignedState,
    };
    return this.sendRequest("mirror_bridged_defund", payload);
  }

  public async CounterChallenge(
    channelId: string,
    action: CounterChallengeAction,
    signedState?: string
  ): Promise<CounterChallengeResult> {
    const payload = {
      ChannelId: channelId,
      Action: action,
      StringifiedL2SignedState: signedState,
    };
    return this.sendRequest("counter_challenge", payload);
  }

  public async ConfirmSwap(
    swapId: string,
    action: ConfirmSwapAction
  ): Promise<ConfirmSwapResult> {
    const payload = {
      SwapId: swapId,
      Action: action,
    };

    return this.sendRequest("confirm_swap", payload);
  }

  public async ClosePaymentChannel(channelId: string): Promise<string> {
    const payload: DefundObjectiveRequest = { ChannelId: channelId };
    return this.sendRequest("close_payment_channel", payload);
  }

  public async CloseSwapChannel(channelId: string): Promise<string> {
    const payload: DefundObjectiveRequest = { ChannelId: channelId };
    return this.sendRequest("close_swap_channel", payload);
  }

  public async GetVersion(): Promise<string> {
    return this.sendRequest("version", {});
  }

  public async GetNodeInfo(): Promise<GetNodeInfo> {
    return this.sendRequest("get_node_info", {});
  }

  public async GetAddress(): Promise<string> {
    if (this.myAddress) {
      return this.myAddress;
    }

    this.myAddress = await this.sendRequest("get_address", {});
    return this.myAddress;
  }

  public async GetLedgerChannel(channelId: string): Promise<LedgerChannelInfo> {
    return this.sendRequest("get_ledger_channel", { Id: channelId });
  }

  public async GetAllLedgerChannels(): Promise<LedgerChannelInfo[]> {
    return this.sendRequest("get_all_ledger_channels", {});
  }

  public async GetAllL2Channels(): Promise<LedgerChannelInfo[]> {
    return this.sendRequest("get_all_l2_channels", {});
  }

  public async GetSignedState(channelId: string): Promise<string> {
    return this.sendRequest("get_signed_state", { Id: channelId });
  }

  public async GetPaymentChannel(
    channelId: string
  ): Promise<PaymentChannelInfo> {
    return this.sendRequest("get_payment_channel", { Id: channelId });
  }

  public async GetSwapChannel(channelId: string): Promise<SwapChannelInfo> {
    return this.sendRequest("get_swap_channel", { Id: channelId });
  }

  public async GetPaymentChannelsByLedger(
    ledgerId: string
  ): Promise<PaymentChannelInfo[]> {
    return this.sendRequest("get_payment_channels_by_ledger", {
      LedgerId: ledgerId,
    });
  }

  public async GetSwapChannelsByLedger(
    ledgerId: string
  ): Promise<SwapChannelInfo[]> {
    return this.sendRequest("get_swap_channels_by_ledger", {
      LedgerId: ledgerId,
    });
  }

  public async GetObjective(objectiveId: string, l2: boolean): Promise<string> {
    return this.sendRequest("get_objective", {
      ObjectiveId: objectiveId,
      L2: l2,
    });
  }

  public async GetL2ObjectiveFromL1(l1ObjectiveId: string): Promise<string> {
    return this.sendRequest("get_l2_objective_from_l1", {
      L1ObjectiveId: l1ObjectiveId,
    });
  }

  public async GetPendingBridgeTxs(channelId: string): Promise<string> {
    return this.sendRequest("get_pending_bridge_txs", {
      ChannelId: channelId,
    });
  }

  private async getAuthToken(): Promise<string> {
    return this.sendRequest("get_auth_token", {});
  }

  private async sendRequest<K extends RequestMethod>(
    method: K,
    payload: RPCRequestAndResponses[K][0]["params"]["payload"]
  ): Promise<RPCRequestAndResponses[K][1]["result"]> {
    const request = generateRequest(method, payload, this.authToken || "");
    const res = await this.transport.sendRequest<K>(request);
    return getAndValidateResult(res, method);
  }

  public async Close(): Promise<void> {
    return this.transport.Close();
  }

  private constructor(transport: Transport) {
    this.transport = transport;
  }

  /**
   * Creates an RPC client that uses HTTP/WS as the transport.
   *
   * @param url - The URL of the HTTP/WS server
   * @returns A NitroRpcClient that uses WS as the transport
   */
  public static async CreateHttpNitroClient(
    url: string,
    isSecure: boolean
  ): Promise<NitroRpcClient> {
    const transport = await HttpTransport.createTransport(url, isSecure);
    const rpcClient = new NitroRpcClient(transport);
    rpcClient.authToken = await rpcClient.getAuthToken();
    return rpcClient;
  }
}
