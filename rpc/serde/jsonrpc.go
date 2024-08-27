package serde

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/payments"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/bridgeddefund"
	"github.com/statechannels/go-nitro/protocols/directdefund"
	"github.com/statechannels/go-nitro/protocols/directfund"
	"github.com/statechannels/go-nitro/protocols/virtualdefund"
	"github.com/statechannels/go-nitro/protocols/virtualfund"
	"github.com/statechannels/go-nitro/types"
)

type RequestMethod string

const (
	GetAuthTokenMethod                RequestMethod = "get_auth_token"
	GetAddressMethod                  RequestMethod = "get_address"
	VersionMethod                     RequestMethod = "version"
	CreateLedgerChannelRequestMethod  RequestMethod = "create_ledger_channel"
	CloseLedgerChannelRequestMethod   RequestMethod = "close_ledger_channel"
	CloseBridgeChannelRequestMethod   RequestMethod = "close_bridge_channel"
	MirrorBridgedDefundRequestMethod  RequestMethod = "mirror_bridged_defund"
	CreatePaymentChannelRequestMethod RequestMethod = "create_payment_channel"
	ClosePaymentChannelRequestMethod  RequestMethod = "close_payment_channel"
	PayRequestMethod                  RequestMethod = "pay"
	GetPaymentChannelRequestMethod    RequestMethod = "get_payment_channel"
	GetVoucherRequestMethod           RequestMethod = "get_voucher"
	GetLedgerChannelRequestMethod     RequestMethod = "get_ledger_channel"
	GetPaymentChannelsByLedgerMethod  RequestMethod = "get_payment_channels_by_ledger"
	GetAllLedgerChannelsMethod        RequestMethod = "get_all_ledger_channels"
	CreateVoucherRequestMethod        RequestMethod = "create_voucher"
	ReceiveVoucherRequestMethod       RequestMethod = "receive_voucher"
	CounterChallengeRequestMethod     RequestMethod = "counter_challenge"
	ValidateVoucherRequestMethod      RequestMethod = "validate_voucher"

	// Bridge methods
	GetAllL2ChannelsRequestMethod RequestMethod = "get_all_l2_channels"
	GetL2ObjectiveFromL1Method    RequestMethod = "get_l2_objective_from_l1"

	GetSignedStateMethod RequestMethod = "get_signed_state"

	// Chain reorgs workaround methods
	GetObjectiveMethod RequestMethod = "get_objective"
	RetryTxMethod      RequestMethod = "retry_tx"
)

type NotificationMethod string

const (
	ObjectiveCompleted    NotificationMethod = "objective_completed"
	LedgerChannelUpdated  NotificationMethod = "ledger_channel_updated"
	PaymentChannelUpdated NotificationMethod = "payment_channel_updated"
)

type NotificationOrRequest interface {
	RequestMethod | NotificationMethod
}

const JsonRpcVersion = "2.0"

type AuthRequest struct {
	Id string
}
type PaymentRequest struct {
	Amount  uint64
	Channel types.Destination
}

type MirrorBridgedDefundRequest struct {
	ChannelId                types.Destination
	StringifiedL2SignedState string
	IsChallenge              bool
}

type CounterChallengeRequest struct {
	ChannelId                types.Destination
	Action                   types.CounterChallengeAction
	StringifiedL2SignedState string
}

type GetPaymentChannelRequest struct {
	Id types.Destination
}
type GetVoucherRequest struct {
	Id types.Destination
}
type GetLedgerChannelRequest struct {
	Id types.Destination
}
type GetPaymentChannelsByLedgerRequest struct {
	LedgerId types.Destination
}

type ValidateVoucherRequest struct {
	VoucherHash common.Hash
	Signer      common.Address
	Value       uint64
}
type RetryTxRequest struct {
	ObjectiveId protocols.ObjectiveId
}

type GetObjectiveRequest struct {
	ObjectiveId protocols.ObjectiveId
	L2          bool
}

type GetL2ObjectiveFromL1Request struct {
	L1ObjectiveId protocols.ObjectiveId
}

type (
	NoPayloadRequest = struct{}
)

type GetSignedStateRequest struct {
	Id types.Destination
}

type RequestPayload interface {
	directfund.ObjectiveRequest |
		directdefund.ObjectiveRequest |
		MirrorBridgedDefundRequest |
		virtualfund.ObjectiveRequest |
		virtualdefund.ObjectiveRequest |
		AuthRequest |
		PaymentRequest |
		GetLedgerChannelRequest |
		GetPaymentChannelRequest |
		GetPaymentChannelsByLedgerRequest |
		GetSignedStateRequest |
		GetVoucherRequest |
		NoPayloadRequest |
		payments.Voucher |
		CounterChallengeRequest |
		ValidateVoucherRequest |
		bridgeddefund.ObjectiveRequest |
		RetryTxRequest |
		GetObjectiveRequest |
		GetL2ObjectiveFromL1Request
}

type NotificationPayload interface {
	protocols.ObjectiveId |
		query.PaymentChannelInfo |
		query.LedgerChannelInfo
}

type Params[T RequestPayload | NotificationPayload] struct {
	AuthToken string `json:"authtoken"`
	Payload   T      `json:"payload"`
}

type JsonRpcSpecificRequest[T RequestPayload | NotificationPayload] struct {
	Jsonrpc string    `json:"jsonrpc"`
	Id      uint64    `json:"id"`
	Method  string    `json:"method"`
	Params  Params[T] `json:"params"`
}

type (
	GetAllLedgersResponse              = []query.LedgerChannelInfo
	GetPaymentChannelsByLedgerResponse = []query.PaymentChannelInfo
)

type ValidateVoucherResponse struct {
	Success   bool
	ErrorCode string
}

type ResponsePayload interface {
	directfund.ObjectiveResponse |
		protocols.ObjectiveId |
		virtualfund.ObjectiveResponse |
		PaymentRequest |
		query.PaymentChannelInfo |
		query.LedgerChannelInfo |
		GetAllLedgersResponse |
		GetPaymentChannelsByLedgerResponse |
		payments.Voucher |
		common.Address |
		string |
		payments.ReceiveVoucherSummary |
		CounterChallengeRequest |
		ValidateVoucherResponse
}

type JsonRpcSuccessResponse[T ResponsePayload] struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      uint64 `json:"id"`
	Result  T      `json:"result"`
}

func NewJsonRpcSpecificRequest[T RequestPayload | NotificationPayload, U RequestMethod | NotificationMethod](requestId uint64, method U, objectiveRequest T, authToken string) *JsonRpcSpecificRequest[T] {
	return &JsonRpcSpecificRequest[T]{
		Jsonrpc: JsonRpcVersion,
		Id:      requestId,
		Method:  string(method),
		Params:  Params[T]{AuthToken: authToken, Payload: objectiveRequest},
	}
}

func NewJsonRpcResponse[T ResponsePayload](requestId uint64, objectiveResponse T) *JsonRpcSuccessResponse[T] {
	return &JsonRpcSuccessResponse[T]{
		Jsonrpc: JsonRpcVersion,
		Id:      requestId,
		Result:  objectiveResponse,
	}
}

type JsonRpcGeneralRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	Id      uint64      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type JsonRpcGeneralResponse struct {
	Jsonrpc string       `json:"jsonrpc"`
	Id      uint64       `json:"id"`
	Error   JsonRpcError `json:"error"`
	Result  interface{}  `json:"result"`
}

type JsonRpcErrorResponse struct {
	Jsonrpc string       `json:"jsonrpc"`
	Id      uint64       `json:"id"`
	Error   JsonRpcError `json:"error"`
}

type JsonRpcError struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func (e JsonRpcError) Error() string {
	return e.Message
}

func NewJsonRpcErrorResponse(requestId uint64, error JsonRpcError) *JsonRpcErrorResponse {
	return &JsonRpcErrorResponse{
		Jsonrpc: JsonRpcVersion,
		Id:      requestId,
		Error:   error,
	}
}

var (
	ParseError            = JsonRpcError{Code: -32700, Message: "Parse error"}
	InvalidRequestError   = JsonRpcError{Code: -32600, Message: "Invalid Request"}
	MethodNotFoundError   = JsonRpcError{Code: -32601, Message: "Method not found"}
	InvalidParamsError    = JsonRpcError{Code: -32602, Message: "Invalid params"}
	InternalServerError   = JsonRpcError{Code: -32603, Message: "Internal error"}
	RequestUnmarshalError = JsonRpcError{Code: -32010, Message: "Could not unmarshal request object"}
	ParamsUnmarshalError  = JsonRpcError{Code: -32009, Message: "Could not unmarshal params object"}
	InvalidAuthTokenError = JsonRpcError{Code: -32008, Message: "Invalid auth token"}
)
