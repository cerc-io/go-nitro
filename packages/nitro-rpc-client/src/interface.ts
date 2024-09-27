import {
  AssetData,
  ChannelStatus,
  CounterChallengeAction,
  CounterChallengeResult,
  LedgerChannelInfo,
  ObjectiveResponse,
  PaymentChannelInfo,
  PaymentPayload,
  ReceiveVoucherResult,
  Voucher,
} from "./types";

interface ledgerChannelApi {
  /**
   * CreateLedgerChannel creates a directly funded ledger channel with the counterparty.
   *
   * @param counterParty - The counterparty to create the channel with
   * @returns A promise that resolves to an objective response, containing the ID of the objective and the channel id.
   */
  CreateLedgerChannel(
    counterParty: string,
    assetsData: AssetData[],
    challengeDuration: number
  ): Promise<ObjectiveResponse>;
  /**
   * CloseLedgerChannel defunds a directly funded ledger channel.
   *
   * @param channelId - The ID of the channel to defund
   * @returns The ID of the objective that was created
   */
  CloseLedgerChannel(channelId: string, isChallenge: boolean): Promise<string>;
  /**
   * GetLedgerChannel queries the RPC server for a payment channel.
   *
   * @param channelId - The ID of the channel to query for
   * @returns A `LedgerChannelInfo` object containing the channel's information
   */
  GetLedgerChannel(channelId: string): Promise<LedgerChannelInfo>;
  /**
   * GetAllLedgerChannels queries the RPC server for all ledger channels.
   * @returns A `LedgerChannelInfo` object containing the channel's information for each ledger channel
   */
  GetAllLedgerChannels(): Promise<LedgerChannelInfo[]>;
}
interface paymentChannelApi {
  /**
   * CreatePaymentChannel creates a virtually funded payment channel with the counterparty, using the given intermediaries.
   *
   * @param counterParty - The counterparty to create the channel with
   * @param intermediaries - The intermerdiaries to use
   * @returns A promise that resolves to an objective response, containing the ID of the objective and the channel id.
   */
  CreatePaymentChannel(
    counterParty: string,
    intermediaries: string[],
    amount: number
  ): Promise<ObjectiveResponse>;
  /**
   * ClosePaymentChannel defunds a virtually funded payment channel.
   *
   * @param channelId - The ID of the channel to defund
   * @returns The ID of the objective that was created
   */
  ClosePaymentChannel(channelId: string): Promise<string>;
  /**
   * GetPaymentChannel queries the RPC server for a payment channel.
   *
   * @param channelId - The ID of the channel to query for
   * @returns A `PaymentChannelInfo` object containing the channel's information
   */
  GetPaymentChannel(channelId: string): Promise<PaymentChannelInfo>;
  /**
   * GetPaymentChannelsByLedger queries the RPC server for any payment channels that are actively funded by the given ledger.
   *
   * @param ledgerId - The ID of the ledger to find payment channels for
   * @returns A `PaymentChannelInfo` object containing the channel's information for each payment channel
   */
  GetPaymentChannelsByLedger(ledgerId: string): Promise<PaymentChannelInfo[]>;
}

interface paymentApi {
  /**
   * Creates a payment voucher for the given channel and amount.
   * The voucher does not get sent to the other party automatically.
   * @param channelId The payment channel to use for the voucher
   * @param amount The amount for the voucher
   * @returns A signed voucher
   */
  CreateVoucher(channelId: string, amount: number): Promise<Voucher>;
  /**
   * Adds a voucher to the go-nitro node that was received from the other party to the channel.
   * @param voucher The voucher to add
   * @returns The total amount of the channel and the delta of the voucher
   */
  ReceiveVoucher(voucher: Voucher): Promise<ReceiveVoucherResult>;
  /**
   * Pay sends a payment on a virtual payment chanel.
   *
   * @param channelId - The ID of the payment channel to use
   * @param amount - The amount to pay
   */
  Pay(channelId: string, amount: number): Promise<PaymentPayload>;
}

interface bridgeAPI {
  /**
   * CloseBridgeChannel defunds a mirrored ledger channel.
   *
   * @param channelId - The ID of the channel to defund
   * @returns The ID of the objective that was created
   */
  CloseBridgeChannel(channelId: string): Promise<string>;
  /**
   * MirrorBridgedDefund defunds ledger channel from which mirrored channel was created.
   *
   * @param channelId - The ID of the channel to defund
   * @param stringifiedL2SignedState - The stringified state of mirrored channel
   * @param isChallenge - Boolean flag to defund with challenge
   * @returns The ID of the objective that was created
   */
  MirrorBridgedDefund(
    channelId: string,
    stringifiedL2SignedState: string,
    isChallenge: boolean
  ): Promise<string>;
  /**
   * GetAllL2Channels gets all mirrored ledger channels.
   *
   * @returns A `LedgerChannelInfo` object containing the channel's information for each ledger channel
   */
  GetAllL2Channels(): Promise<LedgerChannelInfo[]>;
  /**
   * GetL2ObjectiveFromL1 gets corresponding L2 objective based on l1 objective ID.
   *
   * @param l1ObjectiveId - ID of Objective on L1
   * @returns L2 Objective info
   */
  GetL2ObjectiveFromL1(l1ObjectiveId: string): Promise<string>;
  /**
   * GetPendingBridgeTxs gets array of transactions that are not yet confirmed.
   *
   * @param channelId - ID of channel to get pending txs
   * @returns Array of pending txs
   */
  GetPendingBridgeTxs(channelId: string): Promise<string>;
}

interface swapAPI {
  /**
   * CreateSwapChannel creates a swap channel with the counterparty.
   *
   * @param counterParty - The counterparty to create the channel with
   * @param intermediaries - The intermerdiaries to use
   * @returns A promise that resolves to an objective response, containing the ID of the objective and the channel id.
   */
  CreateSwapChannel(
    counterParty: string,
    intermediaries: string[],
    assetsData: AssetData[]
  ): Promise<ObjectiveResponse>;
  /**
   * GetSwapChannel gets swap channel info.
   *
   * @param channelId - The ID of the channel to query for
   * @returns A JSON object containing the channel's information
   */
  GetSwapChannel(channelId: string): Promise<string>;
}

interface syncAPI {
  /**
   * WaitForLedgerChannelStatus blocks until the ledger channel with the given ID to have the given status.
   *
   * @param objectiveId - The channel id to wait for
   * @param status - The channel id to wait for (e.g. Ready or Closing)
   */
  WaitForLedgerChannelStatus(
    objectiveId: string,
    status: ChannelStatus
  ): Promise<void>;
  /**
   * WaitForPaymentChannelStatus blocks until the payment channel with the given ID to have the given status.
   *
   * @param objectiveId - The channel id to wait for
   * @param status - The channel id to wait for (e.g. Ready or Closing)
   */
  WaitForPaymentChannelStatus(
    objectiveId: string,
    status: ChannelStatus
  ): Promise<void>;
  /**
   * PaymentChannelUpdated attaches a callback which is triggered when the channel with supplied ID is updated.
   * Returns a cleanup function which can be used to remove the subscription.
   *
   * @param objectiveId - The id objective to wait for
   */
  onPaymentChannelUpdated(
    channelId: string,
    callback: (info: PaymentChannelInfo) => void
  ): () => void;

  /**
   * WaitForObjectiveToComplete blocks until the objective with the given ID is complete.
   */
  WaitForObjectiveToComplete(objectiveId: string): Promise<void>;
}

export interface RpcClientApi
  extends ledgerChannelApi,
    paymentChannelApi,
    paymentApi,
    syncAPI,
    bridgeAPI,
    swapAPI {
  /**
   * GetVersion queries the API server for it's version.
   *
   * @returns The version of the RPC server
   */
  GetVersion(): Promise<string>;
  /**
   * GetAddress queries the RPC server for it's state channel address.
   *
   * @returns The address of the wallet connected to the RPC server
   */
  GetAddress(): Promise<string>;
  /**
   * Close closes the RPC client and stops listening for notifications.
   */
  Close(): Promise<void>;

  /**
   * RetryObjectiveTx retries failed txs related to given objective.
   *
   * @param objectiveId - The objective to retry failed txs
   * @returns objectiveId of the objective for which tx was retried
   */
  RetryObjectiveTx(objectiveId: string): Promise<string>;
  /**
   * RetryTx retries tx with given tx hash if it is failed.
   *
   * @param txHash - Hash of the tx to retry
   * @returns objectiveId of the objective for which tx was retried
   */
  RetryTx(txHash: string): Promise<string>;
  /**
   * CounterChallenge responds to the ongoing challenge with either `challenge` or `checkpoint`.
   *
   * @param channelId - Channel Id of the channel to counter challenge
   * @param action - The action to respond with (either checkpoint or challenge)
   * @param signedState - Optional param required to counter challenge a mirrored channel on L1
   * @returns A `CounterChallengeResult` object
   */
  CounterChallenge(
    channelId: string,
    action: CounterChallengeAction,
    signedState?: string
  ): Promise<CounterChallengeResult>;
}
