package rpc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/bridge"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/internal/logging"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/bridgeddefund"
	"github.com/statechannels/go-nitro/rpc/serde"
	"github.com/statechannels/go-nitro/rpc/transport"
	"github.com/statechannels/go-nitro/types"
)

type BridgeRpcServer struct {
	*BaseRpcServer
	bridge *bridge.Bridge
}

func NewBridgeRpcServer(bridge *bridge.Bridge, trans transport.Responder) (*BridgeRpcServer, error) {
	baseRpcServer := NewBaseRpcServer(trans)

	brs := &BridgeRpcServer{
		baseRpcServer,
		bridge,
	}

	brs.logger = logging.LoggerWithAddress(slog.Default(), bridge.GetBridgeAddress())

	err := brs.registerHandlers()
	if err != nil {
		return nil, err
	}

	return brs, nil
}

func (brs *BridgeRpcServer) Close() error {
	err := brs.BaseRpcServer.Close()
	if err != nil {
		return err
	}

	return brs.bridge.Close()
}

func (brs *BridgeRpcServer) registerHandlers() (err error) {
	handlerV1 := func(requestData []byte) []byte {
		if !json.Valid(requestData) {
			brs.logger.Error("request is not valid json")
			errRes := serde.NewJsonRpcErrorResponse(0, serde.ParseError)
			return marshalResponse(errRes)
		}

		jsonrpcReq, errRes := validateJsonrpcRequest(requestData)
		brs.logger.Debug("Rpc server received request", "request", jsonrpcReq)
		if errRes != nil {
			brs.logger.Error("could not validate jsonrpc request")

			return errRes
		}

		switch serde.RequestMethod(jsonrpcReq.Method) {
		case serde.GetAuthTokenMethod:
			return processRequest(brs.BaseRpcServer, permNone, requestData, func(req serde.AuthRequest) (string, error) {
				return generateAuthToken(req.Id, allPermissions)
			})
		case serde.GetAllL2ChannelsRequestMethod:
			return processRequest(brs.BaseRpcServer, permSign, requestData, func(req serde.NoPayloadRequest) ([]query.LedgerChannelInfo, error) {
				return brs.bridge.GetAllL2Channels()
			})
		case serde.CounterChallengeRequestMethod:
			return processRequest(brs.BaseRpcServer, permSign, requestData, func(req serde.CounterChallengeRequest) (serde.CounterChallengeRequest, error) {
				var l2SignedState state.SignedState
				if len(req.StringifiedL2SignedState) > 0 {
					err := json.Unmarshal([]byte(req.StringifiedL2SignedState), &l2SignedState)
					if err != nil {
						return serde.CounterChallengeRequest{}, fmt.Errorf("error in unmarshalling signed state payload %w", err)
					}
				}

				brs.bridge.CounterChallenge(req.ChannelId, req.Action, l2SignedState)
				return req, nil
			})
		case serde.GetSignedStateMethod:
			return processRequest(brs.BaseRpcServer, permRead, requestData, func(req serde.GetSignedStateRequest) (string, error) {
				if err := serde.ValidateGetSignedStateRequest(req); err != nil {
					return "", err
				}

				latestState, err := brs.bridge.GetL2SupportedSignedState(req.Id)
				if err != nil {
					return "", err
				}

				marshalledState, err := json.Marshal(latestState)
				if err != nil {
					return "", err
				}

				return string(marshalledState), nil
			})
		case serde.MirrorBridgedDefundRequestMethod:
			return processRequest(brs.BaseRpcServer, permSign, requestData, func(req serde.MirrorBridgedDefundRequest) (protocols.ObjectiveId, error) {
				if DISABLE_BRIDGE_DEFUND {
					return protocols.ObjectiveId(bridgeddefund.ObjectivePrefix + req.ChannelId.String()), fmt.Errorf("bridged defund is currently disabled")
				}

				var l2SignedState state.SignedState
				err := json.Unmarshal([]byte(req.StringifiedL2SignedState), &l2SignedState)
				if err != nil {
					return "", err
				}

				return brs.bridge.MirrorBridgedDefund(req.ChannelId, l2SignedState, req.IsChallenge)
			})
		case serde.RetryTxMethod:
			return processRequest(brs.BaseRpcServer, permSign, requestData, func(req serde.RetryTxRequest) (protocols.ObjectiveId, error) {
				err := brs.bridge.RetryTx(req.ObjectiveId)
				return req.ObjectiveId, err
			})
		case serde.GetObjectiveMethod:
			return processRequest(brs.BaseRpcServer, permSign, requestData, func(req serde.GetObjectiveRequest) (string, error) {
				objective, err := brs.bridge.GetObjectiveById(req.ObjectiveId, req.L2)
				if err != nil {
					return "", err
				}

				marshalledObjective, err := objective.MarshalJSON()
				if err != nil {
					return "", err
				}

				return string(marshalledObjective), nil
			})
		case serde.GetL2ObjectiveFromL1Method:
			return processRequest(brs.BaseRpcServer, permSign, requestData, func(req serde.GetL2ObjectiveFromL1Request) (string, error) {
				l1ChannelId := strings.Split(string(req.L1ObjectiveId), "-")[1]
				l2ChannelId, _ := brs.bridge.GetL2ChannelIdByL1ChannelId(types.Destination(common.HexToHash(l1ChannelId)))

				if l2ChannelId.IsZero() {
					return "", fmt.Errorf("Given objective ID is incorrect")
				}

				l2Objective, ok := brs.bridge.GetL2ObjectiveByChannelId(l2ChannelId)
				l2Objective.Id()

				if !ok {
					return "", fmt.Errorf("Corresponding L2 objective is either complete or does not exist")
				}

				marshalledObjective, err := l2Objective.MarshalJSON()
				if err != nil {
					return "", err
				}

				return string(marshalledObjective), nil
			})
		default:
			errRes := serde.NewJsonRpcErrorResponse(jsonrpcReq.Id, serde.MethodNotFoundError)
			return marshalResponse(errRes)
		}
	}

	err = brs.transport.RegisterRequestHandler("v1", handlerV1)
	return err
}
