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
} from "./types";
import { Transport } from "./transport";
import { createOutcome, generateRequest } from "./utils";
import { HttpTransport } from "./transport/http";
import { getAndValidateResult } from "./serde";
import { RpcClientApi } from "./interface";

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
    assetAddress: string,
    alphaAmount: number,
    betaAmount: number
  ): Promise<ObjectiveResponse> {
    const payload: DirectFundPayload = {
      CounterParty: counterParty,
      ChallengeDuration: 0,
      Outcome: createOutcome(
        assetAddress,
        await this.GetAddress(),
        counterParty,
        alphaAmount,
        betaAmount
      ),
      AppDefinition: assetAddress,
      AppData: "0x00",
      Nonce: Date.now(),
    };
    return this.sendRequest("create_ledger_channel", payload);
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
      Outcome: createOutcome(
        asset,
        await this.GetAddress(),
        counterParty,
        amount,
        // As payment channel is simplex, only alpha node can pay beta node and not vice-versa hence beta node's allocation amount is 0
        0
      ),
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
    action: CounterChallengeAction
  ): Promise<CounterChallengeResult> {
    const payload = {
      ChannelId: channelId,
      Action: action,
    };
    return this.sendRequest("counter_challenge", payload);
  }

  public async ClosePaymentChannel(channelId: string): Promise<string> {
    const payload: DefundObjectiveRequest = { ChannelId: channelId };
    return this.sendRequest("close_payment_channel", payload);
  }

  public async GetVersion(): Promise<string> {
    return this.sendRequest("version", {});
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

  public async GetPaymentChannelsByLedger(
    ledgerId: string
  ): Promise<PaymentChannelInfo[]> {
    return this.sendRequest("get_payment_channels_by_ledger", {
      LedgerId: ledgerId,
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
    url: string
  ): Promise<NitroRpcClient> {
    const transport = await HttpTransport.createTransport(url);
    const rpcClient = new NitroRpcClient(transport);
    rpcClient.authToken = await rpcClient.getAuthToken();
    return rpcClient;
  }
}
