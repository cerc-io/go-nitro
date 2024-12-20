// Package engine contains the types and imperative code for the business logic of a go-nitro Node.
package engine // import "github.com/statechannels/go-nitro/node/engine"

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/consensus_channel"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/internal/logging"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	"github.com/statechannels/go-nitro/node/engine/messageservice"
	p2pms "github.com/statechannels/go-nitro/node/engine/messageservice/p2p-message-service"
	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/payments"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/bridgeddefund"
	"github.com/statechannels/go-nitro/protocols/bridgedfund"
	"github.com/statechannels/go-nitro/protocols/directdefund"
	"github.com/statechannels/go-nitro/protocols/directfund"
	"github.com/statechannels/go-nitro/protocols/mirrorbridgeddefund"
	"github.com/statechannels/go-nitro/protocols/swap"
	"github.com/statechannels/go-nitro/protocols/swapdefund"
	"github.com/statechannels/go-nitro/protocols/swapfund"
	"github.com/statechannels/go-nitro/protocols/virtualdefund"
	"github.com/statechannels/go-nitro/protocols/virtualfund"
	"github.com/statechannels/go-nitro/types"
)

var (
	errEmptyDroppedEvent   error = errors.New("no dropped events yet")
	errSwapObjectiveExists error = errors.New("swap objective already exists")
)

// ErrUnhandledChainEvent is an engine error when the the engine cannot process a chain event
type ErrUnhandledChainEvent struct {
	event   chainservice.Event
	channel channel.Channel
	reason  string
}

func (uce *ErrUnhandledChainEvent) Error() string {
	return fmt.Sprintf("chain event %#v could not be handled by channel %#v due to: %s", uce.event, uce.channel, uce.reason)
}

type ErrGetObjective struct {
	wrappedError error
	objectiveId  protocols.ObjectiveId
}

func (e *ErrGetObjective) Error() string {
	return fmt.Sprintf("unexpected error getting/creating objective %s: %v", e.objectiveId, e.wrappedError)
}

// nonFatalErrors is a list of errors for which the engine should not panic
var nonFatalErrors = []error{
	&ErrGetObjective{},
	store.ErrLoadVouchers,
	directfund.ErrLedgerChannelExists,
	virtualfund.ErrUpdatingLedgerFunding,
	swapfund.ErrUpdatingLedgerFunding,
	swapfund.ErrZeroFunds,
	errEmptyDroppedEvent,
	errSwapObjectiveExists,
	swap.ErrInvalidSwap,
	types.ErrLeftLedgerChannelNotFound,
	types.ErrRightLedgerChannelNotFound,
}

// Engine is the imperative part of the core business logic of a go-nitro Node
type Engine struct {
	// inbound go channels

	// From API
	ObjectiveRequestsFromAPI        chan protocols.ObjectiveRequest
	PaymentRequestsFromAPI          chan PaymentRequest
	CounterChallengeRequestsFromAPI chan CounterChallengeRequest
	RetryObjectiveTxRequestFromAPI  chan types.RetryObjectiveTxRequest
	ConfirmSwapRequestFromAPI       chan types.ConfirmSwapRequest

	fromChain             <-chan chainservice.Event
	droppedEventFromChain <-chan protocols.DroppedEventInfo
	fromMsg               <-chan protocols.Message
	fromLedger            chan consensus_channel.Proposal
	signRequests          <-chan p2pms.SignatureRequest

	eventHandler func(EngineEvent)

	msg   messageservice.MessageService
	chain chainservice.ChainService

	store       store.Store // A Store for persisting and restoring important data
	policymaker PolicyMaker // A PolicyMaker decides whether to approve or reject objectives
	logger      *slog.Logger
	vm          *payments.VoucherManager

	wg     *sync.WaitGroup
	cancel context.CancelFunc
}

// PaymentRequest represents a request from the API to make a payment using a channel
type PaymentRequest struct {
	ChannelId types.Destination
	Amount    *big.Int
}

// CounterChallengeRequest represents a request from the API to initiate a counter challenge against registered challenge
type CounterChallengeRequest struct {
	ChannelId types.Destination
	Action    types.CounterChallengeAction
	Payload   state.SignedState
}

// EngineEvent is a struct that contains a list of changes caused by handling a message/chain event/api event
type EngineEvent struct {
	// These are objectives that are now completed
	CompletedObjectives []protocols.Objective
	// These are objectives that have failed
	FailedObjectives []protocols.ObjectiveId
	// ReceivedVouchers are vouchers we've received from other participants
	ReceivedVouchers []payments.Voucher

	// LedgerChannelUpdates contains channel info for ledger channels that have been updated
	LedgerChannelUpdates []query.LedgerChannelInfo
	// PaymentChannelUpdates contains channel info for payment channels that have been updated
	PaymentChannelUpdates []query.PaymentChannelInfo
	// SwapUpdates contains info for updates in swap
	SwapUpdates []query.SwapInfo
}

// IsEmpty returns true if the EngineEvent contains no changes
func (ee *EngineEvent) IsEmpty() bool {
	return len(ee.CompletedObjectives) == 0 &&
		len(ee.FailedObjectives) == 0 &&
		len(ee.ReceivedVouchers) == 0 &&
		len(ee.LedgerChannelUpdates) == 0 &&
		len(ee.PaymentChannelUpdates) == 0 &&
		len(ee.SwapUpdates) == 0
}

func (ee *EngineEvent) Merge(other EngineEvent) {
	ee.CompletedObjectives = append(ee.CompletedObjectives, other.CompletedObjectives...)
	ee.FailedObjectives = append(ee.FailedObjectives, other.FailedObjectives...)
	ee.ReceivedVouchers = append(ee.ReceivedVouchers, other.ReceivedVouchers...)
	ee.LedgerChannelUpdates = append(ee.LedgerChannelUpdates, other.LedgerChannelUpdates...)
	ee.PaymentChannelUpdates = append(ee.PaymentChannelUpdates, other.PaymentChannelUpdates...)
	ee.SwapUpdates = append(ee.SwapUpdates, other.SwapUpdates...)
}

type CompletedObjectiveEvent struct {
	Id protocols.ObjectiveId
}

// Response is the return type that asynchronous API calls "resolve to". Such a call returns a go channel of type Response.
type Response struct{}

// NewEngine is the constructor for an Engine
func New(vm *payments.VoucherManager, msg messageservice.MessageService, chain chainservice.ChainService, store store.Store, policymaker PolicyMaker, eventHandler func(EngineEvent)) Engine {
	e := Engine{}
	e.logger = logging.LoggerWithAddress(slog.Default(), *store.GetAddress())
	e.store = store

	e.fromLedger = make(chan consensus_channel.Proposal, 100)
	// bind to inbound chans
	e.ObjectiveRequestsFromAPI = make(chan protocols.ObjectiveRequest)
	e.PaymentRequestsFromAPI = make(chan PaymentRequest)
	e.CounterChallengeRequestsFromAPI = make(chan CounterChallengeRequest)
	e.RetryObjectiveTxRequestFromAPI = make(chan types.RetryObjectiveTxRequest)
	e.ConfirmSwapRequestFromAPI = make(chan types.ConfirmSwapRequest)

	e.fromChain = chain.EventEngineFeed()
	e.droppedEventFromChain = chain.DroppedEventEngineFeed()
	e.fromMsg = msg.P2PMessages()
	e.signRequests = msg.SignRequests()

	e.chain = chain
	e.msg = msg

	e.eventHandler = eventHandler

	e.policymaker = policymaker

	e.vm = vm

	e.logger.Info("Constructed Engine")

	e.wg = &sync.WaitGroup{}

	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel

	e.wg.Add(1)
	go e.run(ctx)

	return e
}

func (e *Engine) Close() error {
	e.cancel()
	e.wg.Wait()
	if err := e.msg.Close(); err != nil {
		return err
	}

	return e.chain.Close()
}

// run kicks of an infinite loop that waits for communications on the supplied channels, and handles them accordingly
// The loop exits when the context is cancelled.
func (e *Engine) run(ctx context.Context) {
	for {
		var res EngineEvent
		var err error

		var blockTicker <-chan time.Time
		var ticker *time.Ticker
		_, isEthChainService := e.chain.(*chainservice.EthChainService)
		if isEthChainService {
			ticker = time.NewTicker(5 * time.Second)
			blockTicker = ticker.C
		}

		select {

		case or := <-e.ObjectiveRequestsFromAPI:
			res, err = e.handleObjectiveRequest(or)
		case pr := <-e.PaymentRequestsFromAPI:
			res, err = e.handlePaymentRequest(pr)
		case chainEvent := <-e.fromChain:
			res, err = e.handleChainEvent(chainEvent)
		case droppedEventTxInfo := <-e.droppedEventFromChain:
			err = e.handleDroppedChainEvent(droppedEventTxInfo)
		case message := <-e.fromMsg:
			res, err = e.handleMessage(message)
		case proposal := <-e.fromLedger:
			res, err = e.handleProposal(proposal)
		case signReq := <-e.signRequests:
			err = e.handleSignRequest(signReq)
		case counterChallengeReq := <-e.CounterChallengeRequestsFromAPI:
			err = e.handleCounterChallengeRequest(counterChallengeReq)
		case retryObjectiveTxReq := <-e.RetryObjectiveTxRequestFromAPI:
			err = e.handleRetryObjectiveTxRequest(retryObjectiveTxReq)
		case confirmSwapReq := <-e.ConfirmSwapRequestFromAPI:
			res, err = e.handleConfirmSwapRequest(confirmSwapReq)
		case <-blockTicker:
			blockNum := e.chain.GetLastConfirmedBlockNum()
			err = e.store.SetLastBlockNumSeen(blockNum)
			e.checkError(err)

			var block *ethTypes.Block

			block, err = e.chain.GetBlockByNumber(big.NewInt(int64(blockNum)))
			if err != nil {
				e.logger.Error(err.Error())
				err = nil
			}

			if block != nil {
				chainServiceBlock := chainservice.Block{
					BlockNum:  block.NumberU64(),
					Timestamp: block.Time(),
				}

				err = e.processStoreChannels(chainServiceBlock)
			}
		case <-ctx.Done():
			if ticker != nil {
				ticker.Stop()
			}

			e.wg.Done()
			return
		}

		// Handle errors
		e.checkError(err)

		// Only send out an event if there are changes
		if !res.IsEmpty() {

			for _, obj := range res.CompletedObjectives {
				e.logger.Info("Objective is complete & returned to API", logging.WithObjectiveIdAttribute(obj.Id()))
			}
			e.eventHandler(res)
		}

	}
}

// handleProposal handles a Proposal returned to the engine from
// a running ledger channel by pulling its corresponding objective
// from the store and attempting progress.
func (e *Engine) handleProposal(proposal consensus_channel.Proposal) (EngineEvent, error) {
	c, _ := e.store.GetChannelById(proposal.Target())
	id := getProposalObjectiveId(proposal, c.Type)

	obj, err := e.store.GetObjectiveById(id)
	if err != nil {
		return EngineEvent{}, err
	}
	if obj.GetStatus() == protocols.Completed {
		e.logger.Info("Ignoring proposal for completed objective", logging.WithObjectiveIdAttribute(id))
		return EngineEvent{}, nil
	}
	return e.attemptProgress(obj)
}

func (e *Engine) handleSignRequest(sigReq p2pms.SignatureRequest) error {
	recordDataBytes, err := json.Marshal(sigReq.Data)
	if err != nil {
		return err
	}

	hash := sha256.Sum256(recordDataBytes) // Hash the data before signing it
	secretKey := e.store.GetChannelSecretKey()
	signature, err := secp256k1.Sign(hash[:], *secretKey)
	if err != nil {
		return err
	}

	sigReq.ResponseChan <- signature
	return nil
}

// handleMessage handles a Message from a peer go-nitro Wallet.
// It:
//   - reads an objective from the store,
//   - generates an updated objective,
//   - attempts progress on the target Objective,
//   - attempts progress on related objectives which may have become unblocked.
func (e *Engine) handleMessage(message protocols.Message) (EngineEvent, error) {
	e.logMessage(message, Incoming)
	allCompleted := EngineEvent{}

	for _, payload := range message.ObjectivePayloads {

		objective, err := e.getOrCreateObjective(payload)
		if err != nil {
			return EngineEvent{}, err
		}

		if objective.GetStatus() == protocols.Unapproved {
			e.logger.Info("Policymaker for objective", "policy-maker", e.policymaker, logging.WithObjectiveIdAttribute(objective.Id()))
			if e.policymaker.ShouldApprove(objective) {
				objective = objective.Approve()

				ddfo, ok := objective.(*directdefund.Objective)
				if ok {
					// If we just approved a direct defund objective, destroy the consensus channel to prevent it being used (a Channel will now take over governance)
					err := e.store.DestroyConsensusChannel(ddfo.C.Id)
					if err != nil {
						return EngineEvent{}, err
					}
				}

				so, ok := objective.(*swap.Objective)
				if ok {
					rejectedObjective, err := e.rejectSwapIfPendingExists(so)
					if err != nil {
						return EngineEvent{}, err
					}

					if rejectedObjective != nil {
						allCompleted.CompletedObjectives = append(allCompleted.CompletedObjectives, rejectedObjective)

						if rejectedObjective.Id() == so.Id() {
							return allCompleted, nil
						}
					}
				}

			} else {
				objective, sideEffects := objective.Reject()
				err = e.store.SetObjective(objective)
				if err != nil {
					return EngineEvent{}, err
				}

				allCompleted.CompletedObjectives = append(allCompleted.CompletedObjectives, objective)

				err = e.executeSideEffects(sideEffects)
				// An error would mean we failed to send a message. But the objective is still "completed".
				// So, we should return allCompleted even if there was an error.
				return allCompleted, err
			}
		}

		if objective.GetStatus() == protocols.Completed {
			e.logger.Info("Ignoring payload for completed objective", logging.WithObjectiveIdAttribute(objective.Id()))

			continue
		}
		if objective.GetStatus() == protocols.Rejected {
			e.logger.Info("Ignoring payload for rejected objective", logging.WithObjectiveIdAttribute(objective.Id()))
			continue
		}

		updatedObjective, err := objective.Update(payload)
		if err != nil {
			return EngineEvent{}, err
		}

		progressEvent, err := e.attemptProgress(updatedObjective)
		if err != nil {
			return EngineEvent{}, err
		}

		allCompleted.Merge(progressEvent)

		if err != nil {
			return EngineEvent{}, err
		}

	}

	slog.Debug("DEBUG: engine.go-handleMessage length of ledger proposals via message", "length", len(message.LedgerProposals))

	for _, entry := range message.LedgerProposals { // The ledger protocol requires us to process these proposals in turnNum order.
		// Here we rely on the sender having packed them into the message in that order, and do not apply any checks or sorting of our own.
		marshalledProposal, er := json.Marshal(entry)
		if er != nil {
			slog.Debug("DEBUG: engine.go-handleMessage error marshalling proposal")
		} else {
			slog.Debug("DEBUG: engine.go-handleMessage proposal received from message", "proposal", string(marshalledProposal))
		}

		c, _ := e.store.GetChannelById(entry.Proposal.Target())
		id := getProposalObjectiveId(entry.Proposal, c.Type)

		o, err := e.store.GetObjectiveById(id)
		if err != nil {
			return EngineEvent{}, err
		}
		if o.GetStatus() == protocols.Completed {
			e.logger.Info("Ignoring proposal for completed objective", logging.WithObjectiveIdAttribute(id))

			continue
		}
		objective, isProposalReceiver := o.(protocols.ProposalReceiver)
		if !isProposalReceiver {
			return EngineEvent{}, fmt.Errorf("received a proposal for an objective which cannot receive proposals %s", objective.Id())
		}

		updatedObjective, err := objective.ReceiveProposal(entry)
		if err != nil {
			return EngineEvent{}, err
		}

		progressEvent, err := e.attemptProgress(updatedObjective)
		if err != nil {
			return EngineEvent{}, err
		}

		allCompleted.Merge(progressEvent)

	}

	for _, entry := range message.RejectedObjectives {
		objective, err := e.store.GetObjectiveById(entry)
		if err != nil {
			return EngineEvent{}, err
		}
		if objective.GetStatus() == protocols.Rejected {
			e.logger.Info("Ignoring payload for rejected objective", logging.WithObjectiveIdAttribute(objective.Id()))

			continue
		}

		// we are rejecting due to a counterparty message notifying us of their rejection. We
		// do not need to send a message back to that counterparty, and furthermore we assume that
		// counterparty has already notified all other interested parties. We can therefore ignore the side effects
		objective, _ = objective.Reject()
		err = e.store.SetObjective(objective)
		if err != nil {
			return EngineEvent{}, err
		}

		allCompleted.CompletedObjectives = append(allCompleted.CompletedObjectives, objective)
	}

	for _, voucher := range message.Payments {

		// TODO: return the amount we paid?
		_, _, err := e.vm.Receive(voucher)

		allCompleted.ReceivedVouchers = append(allCompleted.ReceivedVouchers, voucher)
		if err != nil {
			return EngineEvent{}, fmt.Errorf("error accepting payment voucher: %w", err)
		}
		c, ok := e.store.GetChannelById(voucher.ChannelId)
		if !ok {
			return EngineEvent{}, fmt.Errorf("could not fetch channel for voucher %+v", voucher)
		}

		// Vouchers only count as payment channel updates if the channel is open.
		if !c.FinalCompleted() {

			paid, remaining, err := query.GetVoucherBalance(c.Id, e.vm)
			if err != nil {
				return EngineEvent{}, err
			}
			info, err := query.ConstructPaymentInfo(c, paid, remaining)
			if err != nil {
				return EngineEvent{}, err
			}
			allCompleted.PaymentChannelUpdates = append(allCompleted.PaymentChannelUpdates, info)
		}

	}
	return allCompleted, nil
}

// handleChainEvent handles a Chain Event from the blockchain.
// It:
//   - reads an objective from the store,
//   - generates an updated objective, and
//   - attempts progress.
func (e *Engine) handleChainEvent(chainEvent chainservice.Event) (EngineEvent, error) {
	e.logger.Info("Handling chain event", "blockNum", chainEvent.Block().BlockNum, "event", chainEvent)
	err := e.store.SetLastBlockNumSeen(chainEvent.Block().BlockNum)
	if err != nil {
		return EngineEvent{}, err
	}

	channelId := chainEvent.ChannelID()

	_, isChallengeRegistered := chainEvent.(chainservice.ChallengeRegisteredEvent)
	_, isChallengeCleared := chainEvent.(chainservice.ChallengeClearedEvent)

	if isChallengeRegistered || isChallengeCleared {
		l1ChannelId, err := e.checkAndProcessL2Channel(chainEvent, isChallengeRegistered)
		if err != nil {
			if errors.Is(err, mirrorbridgeddefund.ErrChannelNotExist) {
				return EngineEvent{}, nil
			}

			return EngineEvent{}, err
		}

		if !l1ChannelId.IsZero() {
			channelId = l1ChannelId
		}
	}

	c, ok := e.store.GetChannelById(channelId)
	if !ok {
		// If channel doesn't exist and chain event is ChallengeRegistered then create a new direct defund objective
		// This doesn't occur for actor who registered the challenge
		_, isChallengeRegistered := chainEvent.(chainservice.ChallengeRegisteredEvent)

		if isChallengeRegistered {
			ddfo, err := directdefund.NewObjective(directdefund.NewObjectiveRequest(chainEvent.ChannelID(), false), true, e.store.GetConsensusChannelById, e.store.GetChannelById, e.vm.GetVoucherIfAmountPresent, true)
			if err != nil {
				// Node should not panic if it is unable to find the required consensus channel before creating objective
				if errors.Is(err, directdefund.ErrChannelNotExist) {
					return EngineEvent{}, nil
				}

				return EngineEvent{}, err
			}
			// If ddfo creation was successful, destroy the consensus channel to prevent it being used (a Channel will now take over governance)
			err = e.store.DestroyConsensusChannel(chainEvent.ChannelID())
			if err != nil {
				return EngineEvent{}, err
			}
			c = ddfo.C
			err = e.store.SetObjective(&ddfo)
			if err != nil {
				return EngineEvent{}, err
			}
		} else {
			// TODO: Right now the chain service returns chain events for ALL channels even those we aren't involved in
			// for now we can ignore channels we aren't involved in
			// in the future the chain service should allow us to register for specific channels
			return EngineEvent{}, nil
		}
	}

	updatedChannel, err := c.UpdateWithChainEvent(chainEvent)
	if err != nil {
		return EngineEvent{}, err
	}
	updatedChannel.UpdateChannelMode(chainEvent.Block().Timestamp)

	err = e.store.SetChannel(updatedChannel)
	if err != nil {
		return EngineEvent{}, err
	}

	objective, ok := e.store.GetObjectiveByChannelId(updatedChannel.Id)

	if ok {
		return e.attemptProgress(objective)
	}

	// When a challenge is registered on a virtual channel, identify the related ledger channel and its associated objective, and then process it.
	// Challenge registered on virtual channels are handled only by the one who raised the challenge
	_, isChallengeRegisteredEvent := chainEvent.(chainservice.ChallengeRegisteredEvent)
	if isChallengeRegisteredEvent && c.Type == types.Virtual && c.OnChain.IsChallengeInitiatedByMe {
		myAddress := *e.store.GetAddress()
		counterParty := common.Address{}

		// Find counterparty address to get consensus channel between us
		if myAddress == c.Participants[0] {
			counterParty = c.Participants[len(c.Participants)-1]
		} else if myAddress == c.Participants[len(c.Participants)-1] {
			counterParty = c.Participants[0]
		}

		consensusChannel, ok := e.store.GetConsensusChannel(counterParty)
		if ok {
			obj, ok := e.store.GetObjectiveByChannelId(consensusChannel.Id)
			if ok {
				return e.attemptProgress(obj)
			}
		}
	}

	return EngineEvent{}, nil
}

func (e *Engine) handleDroppedChainEvent(droppedEventInfo protocols.DroppedEventInfo) error {
	obj, ok := e.store.GetObjectiveByChannelId(droppedEventInfo.ChannelId)

	if !ok {
		slog.Info("Could not find objective with given channel ID", "channelId", droppedEventInfo.ChannelId)
		return nil
	}

	switch objective := obj.(type) {
	case *directfund.Objective:
		objective.SetDroppedEvent(droppedEventInfo)
		err := e.store.SetObjective(objective)
		if err != nil {
			return err
		}
	case *directdefund.Objective:
		objective.SetDroppedEvent(droppedEventInfo)
		err := e.store.SetObjective(objective)
		if err != nil {
			return err
		}
	}

	return nil
}

// checkAndProcessL2Channel checks if the chain event corresponds to an L2 channel and retrieves its L1 channel ID.
// If the L1 channel doesn't exist, it creates a mirror bridged defund objective.
func (e *Engine) checkAndProcessL2Channel(chainEvent chainservice.Event, isChallengeRegistered bool) (types.Destination, error) {
	// Check whether a challenge has been registered / cleared for the L2 channel, and then retrieve its L1 channel using an eth call to NitroAdjudicator contract
	l1ChannelId, err := e.chain.GetL1ChannelFromL2(chainEvent.ChannelID())
	if err != nil {
		return types.Destination{}, err
	}

	if l1ChannelId.IsZero() {
		return types.Destination{}, nil
	}

	_, ok := e.store.GetChannelById(l1ChannelId)
	if ok {
		return l1ChannelId, nil
	}

	// If channel doesn't exist and chain event is ChallengeRegistered on L2 then create a new mirror bridged defund objective
	// This doesn't occur for actor who registered the challenge on L2
	if isChallengeRegistered && !ok {
		mbdfo, err := mirrorbridgeddefund.NewObjective(mirrorbridgeddefund.NewObjectiveRequest(l1ChannelId, state.SignedState{}, false), true, e.store.GetConsensusChannelById, true)
		if err != nil {
			return types.Destination{}, err
		}

		// Destroy the consensus channel to prevent it being used (Channel will now take over governance)
		err = e.store.DestroyConsensusChannel(mbdfo.C.Id)
		if err != nil {
			return types.Destination{}, err
		}

		err = e.store.SetObjective(&mbdfo)
		if err != nil {
			return types.Destination{}, err
		}

		return l1ChannelId, nil
	}

	return types.Destination{}, nil
}

// handleObjectiveRequest handles an ObjectiveRequest (triggered by a client API call).
// It will attempt to spawn a new, approved objective.
func (e *Engine) handleObjectiveRequest(or protocols.ObjectiveRequest) (EngineEvent, error) {
	myAddress := *e.store.GetAddress()

	chainId, err := e.chain.GetChainId()
	if err != nil {
		return EngineEvent{}, fmt.Errorf("could not get chain id from chain service: %w", err)
	}

	objectiveId := or.Id(myAddress, chainId)
	failedEngineEvent := EngineEvent{FailedObjectives: []protocols.ObjectiveId{objectiveId}}
	e.logger.Info("handling new objective request", logging.WithObjectiveIdAttribute(objectiveId))
	defer or.SignalObjectiveStarted()
	switch request := or.(type) {

	case virtualfund.ObjectiveRequest:
		vfo, err := virtualfund.NewObjective(request, true, myAddress, chainId, e.store.GetConsensusChannel)
		if err != nil {
			return failedEngineEvent, fmt.Errorf("handleAPIEvent: Could not create virtualfund objective for %+v: %w", request, err)
		}
		// Only Alice or Bob care about registering the objective and keeping track of vouchers
		lastParticipant := uint(len(vfo.V.Participants) - 1)
		if vfo.MyRole == lastParticipant || vfo.MyRole == payments.PAYER_INDEX {
			err = e.registerPaymentChannel(vfo)
			if err != nil {
				return failedEngineEvent, fmt.Errorf("could not register channel with payment/receipt manager: %w", err)
			}
		}

		if err != nil {
			return failedEngineEvent, fmt.Errorf("could not register channel with payment/receipt manager: %w", err)
		}
		return e.attemptProgress(&vfo)

	case swap.ObjectiveRequest:
		so, err := swap.NewObjective(request, true, true, e.store.GetChannelById)
		if err != nil {
			return failedEngineEvent, fmt.Errorf("handleAPIEvent: Could not create swap objective for %+v: %w", request, err)
		}

		pendingSwap, err := e.store.GetPendingSwapByChannelId(so.C.Id)
		if err != nil {
			return EngineEvent{}, err
		}

		if pendingSwap != nil {
			return failedEngineEvent, errSwapObjectiveExists
		}
		return e.attemptProgress(&so)

	case swapfund.ObjectiveRequest:
		sfo, err := swapfund.NewObjective(request, true, myAddress, chainId, e.store.GetConsensusChannel)
		if err != nil {
			return failedEngineEvent, fmt.Errorf("handleAPIEvent: Could not create swapfund objective for %+v: %w", request, err)
		}

		if err != nil {
			return failedEngineEvent, fmt.Errorf("could not register channel with swap manager: %w", err)
		}
		return e.attemptProgress(&sfo)

	case virtualdefund.ObjectiveRequest:
		minAmount := big.NewInt(0)
		if e.vm.ChannelRegistered(request.ChannelId) {
			paid, err := e.vm.Paid(request.ChannelId)
			if err != nil {
				return failedEngineEvent, fmt.Errorf("handleAPIEvent: Could not create virtualdefund objective for %+v: %w", request, err)
			}
			minAmount = paid
		}
		vdfo, err := virtualdefund.NewObjective(request, true, myAddress, minAmount, e.store.GetChannelById, e.store.GetConsensusChannel)
		if err != nil {
			return failedEngineEvent, fmt.Errorf("handleAPIEvent: Could not create virtualdefund objective for %+v: %w", request, err)
		}
		return e.attemptProgress(&vdfo)

	case swapdefund.ObjectiveRequest:
		sdfo, err := swapdefund.NewObjective(request, true, myAddress, e.store.GetChannelById, e.store.GetConsensusChannel)
		if err != nil {
			return failedEngineEvent, fmt.Errorf("handleAPIEvent: Could not create swapdefund objective for %+v: %w", request, err)
		}
		return e.attemptProgress(&sdfo)

	case directfund.ObjectiveRequest:
		dfo, err := directfund.NewObjective(request, true, myAddress, chainId, e.store.GetChannelsByParticipant, e.store.GetConsensusChannel)
		if err != nil {
			return failedEngineEvent, fmt.Errorf("handleAPIEvent: Could not create directfund objective for %+v: %w", request, err)
		}
		return e.attemptProgress(&dfo)

	case directdefund.ObjectiveRequest:
		ddfo, err := directdefund.NewObjective(request, true, e.store.GetConsensusChannelById, e.store.GetChannelById, e.vm.GetVoucherIfAmountPresent, false)
		if err != nil {
			return failedEngineEvent, fmt.Errorf("handleAPIEvent: Could not create directdefund objective for %+v: %w", request, err)
		}

		// Retaining the consensus channel only if ddfo is with challenge since it's needed to process a challenge-registered event for a virtual channel and obtain the ledger channel ID.
		if !ddfo.IsChallenge {
			err = e.store.DestroyConsensusChannel(request.ChannelId)
			if err != nil {
				return failedEngineEvent, fmt.Errorf("handleAPIEvent: Could not destroy consensus channel for %+v: %w", request, err)
			}
		}

		return e.attemptProgress(&ddfo)
	case bridgedfund.ObjectiveRequest:
		bfo, err := bridgedfund.NewObjective(request, true, myAddress, chainId, e.store.GetChannelsByParticipant, e.store.GetConsensusChannel)
		if err != nil {
			return failedEngineEvent, fmt.Errorf("handleAPIEvent: Could not create bridgedfund objective for %+v: %w", request, err)
		}
		return e.attemptProgress(&bfo)

	case bridgeddefund.ObjectiveRequest:
		bdfo, err := bridgeddefund.NewObjective(request, true, e.store.GetConsensusChannelById)
		if err != nil {
			return failedEngineEvent, fmt.Errorf("handleAPIEvent: Could not create bridgeddefund objective for %+v: %w", request, err)
		}

		// Destroy the consensus channel to prevent it being used (Channel will now take over governance)
		err = e.store.DestroyConsensusChannel(bdfo.C.Id)
		if err != nil {
			return failedEngineEvent, err
		}

		return e.attemptProgress(&bdfo)
	case mirrorbridgeddefund.ObjectiveRequest:
		mbdfo, err := mirrorbridgeddefund.NewObjective(request, true, e.store.GetConsensusChannelById, true)
		if err != nil {
			return failedEngineEvent, fmt.Errorf("handleAPIEvent: Could not create mirrorbridgeddefund objective for %+v: %w", request, err)
		}

		// Destroy the consensus channel to prevent it being used (Channel will now take over governance)
		err = e.store.DestroyConsensusChannel(mbdfo.C.Id)
		if err != nil {
			return failedEngineEvent, err
		}

		return e.attemptProgress(&mbdfo)

	default:
		return failedEngineEvent, fmt.Errorf("handleAPIEvent: Unknown objective type %T", request)
	}
}

// handlePaymentRequest handles an PaymentRequest (triggered by a client API call).
// It prepares and dispatches a payment message to the counterparty.
func (e *Engine) handlePaymentRequest(request PaymentRequest) (EngineEvent, error) {
	ee := EngineEvent{}
	if (request == PaymentRequest{}) {
		return ee, fmt.Errorf("handleAPIEvent: Empty payment request")
	}
	cId := request.ChannelId
	voucher, err := e.vm.Pay(
		cId,
		request.Amount,
		*e.store.GetChannelSecretKey())
	if err != nil {
		return ee, fmt.Errorf("handleAPIEvent: Error making payment: %w", err)
	}
	c, ok := e.store.GetChannelById(cId)
	if !ok {
		return ee, fmt.Errorf("handleAPIEvent: Could not get channel from the store %s", cId)
	}
	payer, payee := payments.GetPayer(c.Participants), payments.GetPayee(c.Participants)
	if payer != *e.store.GetAddress() {
		return ee, fmt.Errorf("handleAPIEvent: Not the sender in channel %s", cId)
	}
	info, err := query.GetPaymentChannelInfo(cId, e.store, e.vm)
	if err != nil {
		return ee, fmt.Errorf("handleAPIEvent: Error querying channel info: %w", err)
	}
	ee.PaymentChannelUpdates = append(ee.PaymentChannelUpdates, info)

	se := protocols.SideEffects{MessagesToSend: protocols.CreateVoucherMessage(voucher, payee)}
	return ee, e.executeSideEffects(se)
}

// handleCounterChallengeRequest handles a counter challenge request for the given channel.
func (e *Engine) handleCounterChallengeRequest(request CounterChallengeRequest) error {
	channelId := request.ChannelId
	isCheckPoint := false
	isChallenge := false

	switch request.Action {
	case types.Checkpoint:
		isCheckPoint = true
	case types.Challenge:
		isChallenge = true
	default:
		return fmt.Errorf("unknown counter challenge action")
	}

	obj, ok := e.store.GetObjectiveByChannelId(channelId)
	if !ok {
		return fmt.Errorf("objective to process the counter challenge request for channel Id %s could not be found", channelId.String())
	}

	switch objective := obj.(type) {
	case *directdefund.Objective:
		objective.IsCheckpoint = isCheckPoint
		objective.IsChallenge = isChallenge

	case *mirrorbridgeddefund.Objective:
		if request.Payload.State().ChannelId().IsZero() {
			return fmt.Errorf("invalid L2 signed state")
		}

		objective.L2SignedState = request.Payload
		objective.IsCheckPoint = isCheckPoint
		objective.IsChallenge = isChallenge

	default:
		return fmt.Errorf("unknown objective type %T", objective)
	}

	_, err := e.attemptProgress(obj)
	if err != nil {
		return err
	}

	return nil
}

func (e *Engine) handleConfirmSwapRequest(request types.ConfirmSwapRequest) (EngineEvent, error) {
	objective, err := e.store.GetObjectiveById(protocols.ObjectiveId(swap.ObjectivePrefix + request.SwapId.String()))
	if err != nil {
		return EngineEvent{}, err
	}
	o, ok := objective.(*swap.Objective)
	if !ok {
		return EngineEvent{}, fmt.Errorf("not a swap objective")
	}

	o.SwapStatus = request.Action

	return e.attemptProgress(o)
}

func (e *Engine) handleRetryObjectiveTxRequest(request types.RetryObjectiveTxRequest) error {
	// Get objective from objective id
	obj, err := e.store.GetObjectiveById(protocols.ObjectiveId(request.ObjectiveId))
	if err != nil {
		return &ErrGetObjective{wrappedError: err, objectiveId: protocols.ObjectiveId(request.ObjectiveId)}
	}

	// Based on objective type, send appropriate tx
	switch objective := obj.(type) {
	case *directfund.Objective:
		droppedEvent := objective.GetDroppedEvent()
		if droppedEvent.ChannelId.IsZero() {
			return errEmptyDroppedEvent
		}

		objective.ResetTxSubmitted()
		_, err = e.attemptProgress(objective)

	case *directdefund.Objective:
		droppedEvent := objective.GetDroppedEvent()
		if droppedEvent.ChannelId.IsZero() {
			return errEmptyDroppedEvent
		}
		objective.ResetWithDrawAllTxSubmitted()
		_, err = e.attemptProgress(objective)
	}

	if err != nil {
		return err
	}

	return nil
}

// sendMessages sends out the messages and records the metrics.
func (e *Engine) sendMessages(msgs []protocols.Message) {
	defer e.wg.Done()
	for _, message := range msgs {
		message.From = *e.store.GetAddress()
		err := e.msg.Send(message)
		if err != nil {
			e.logger.Error("could not send message", "message", message.Summarize(e.getChannelTypeById))
			e.logger.Error(err.Error())

			return
		}
		e.logMessage(message, Outgoing)
	}
}

// executeSideEffects executes the SideEffects declared by cranking an Objective or handling a payment request.
func (e *Engine) executeSideEffects(sideEffects protocols.SideEffects) error {
	e.wg.Add(1)
	// Send messages in a go routine so that we don't block on message delivery
	go e.sendMessages(sideEffects.MessagesToSend)

	for _, tx := range sideEffects.TransactionsToSubmit {
		e.logger.Info("Sending chain transaction", "channel", tx.ChannelId().String())

		_, err := e.chain.SendTransaction(tx)
		if err != nil {
			return err
		}
	}
	for _, proposal := range sideEffects.ProposalsToProcess {
		e.fromLedger <- proposal
	}

	return nil
}

// attemptProgress takes a "live" objective in memory and performs the following actions:
//
//  1. It pulls the secret key from the store
//  2. It cranks the objective with that key
//  3. It commits the cranked objective to the store
//  4. It executes any side effects that were declared during cranking
//  5. It updates progress metadata in the store
func (e *Engine) attemptProgress(objective protocols.Objective) (outgoing EngineEvent, err error) {
	secretKey := e.store.GetChannelSecretKey()
	var crankedObjective protocols.Objective
	var sideEffects protocols.SideEffects
	var waitingFor protocols.WaitingFor

	crankedObjective, sideEffects, waitingFor, err = objective.Crank(secretKey)
	if err != nil {
		return
	}

	err = e.store.SetObjective(crankedObjective)
	if err != nil {
		return EngineEvent{}, err
	}

	notifEvents, err := e.generateNotifications(crankedObjective)
	if err != nil {
		return EngineEvent{}, err
	}
	outgoing.Merge(notifEvents)

	e.logger.Info("Objective cranked", logging.WithObjectiveIdAttribute(objective.Id()), "waiting-for", string(waitingFor))

	// If our protocol is waiting for nothing then we know the objective is complete
	// TODO: If attemptProgress is called on a completed objective CompletedObjectives would include that objective id
	// Probably should have a better check that only adds it to CompletedObjectives if it was completed in this crank
	if waitingFor == "WaitingForNothing" {
		outgoing.CompletedObjectives = append(outgoing.CompletedObjectives, crankedObjective)

		// Only release the channel if the objective owns one
		// Swap objective does not own a channel
		channel := crankedObjective.OwnsChannel()
		if !channel.IsZero() {
			err = e.store.ReleaseChannelFromOwnership(crankedObjective.OwnsChannel())
			if err != nil {
				return
			}
		}
		err = e.spawnConsensusChannelIfDirectFundObjective(crankedObjective) // Here we assume that every directfund.Objective is for a ledger channel.
		if err != nil {
			return
		}

		err = e.spawnConsensusChannelIfBridgedFundObjective(crankedObjective) // Here we assume that every bridgedfund.Objective is for a ledger channel.
		if err != nil {
			return
		}

		err = e.destroyObjectiveAndChannelIfChallengeCleared(crankedObjective)
		if err != nil {
			return
		}
	}
	err = e.executeSideEffects(sideEffects)
	return
}

// generateNotifications takes an objective and constructs notifications for any related channels for that objective.
func (e *Engine) generateNotifications(o protocols.Objective) (EngineEvent, error) {
	outgoing := EngineEvent{}

	for _, rel := range o.Related() {
		switch c := rel.(type) {
		case *channel.VirtualChannel:
			var paid, remaining *big.Int

			if !c.FinalCompleted() {
				// If the channel is open, we inspect vouchers for that channel to get the future resolvable balance
				var err error
				paid, remaining, err = query.GetVoucherBalance(c.Id, e.vm)
				if err != nil {
					return outgoing, err
				}
			} else {
				// If the channel is closed, vouchers have already been resolved.
				// Note that when virtual defunding, this information may in fact be more up to date than
				// the voucher balance due to a race condition https://github.com/statechannels/go-nitro/issues/1323
				paid, remaining = c.GetPaidAndRemaining()
			}
			info, err := query.ConstructPaymentInfo(&c.Channel, paid, remaining)
			if err != nil {
				return outgoing, err
			}

			slog.Debug("DEBUG: engine.go-generateNotifications generating notification for payment_channel_updated")
			outgoing.PaymentChannelUpdates = append(outgoing.PaymentChannelUpdates, info)
		case *channel.Channel:
			l, err := query.ConstructLedgerInfoFromChannel(c, *e.store.GetAddress())
			if err != nil {
				return outgoing, err
			}
			outgoing.LedgerChannelUpdates = append(outgoing.LedgerChannelUpdates, l)
		case *consensus_channel.ConsensusChannel:
			l, err := query.ConstructLedgerInfoFromConsensus(c, *e.store.GetAddress())
			if err != nil {
				return outgoing, err
			}
			outgoing.LedgerChannelUpdates = append(outgoing.LedgerChannelUpdates, l)
		case *payments.Swap:
			swapInfo := query.SwapInfo{
				Id:        c.Id,
				ChannelId: c.ChannelId,
			}

			outgoing.SwapUpdates = append(outgoing.SwapUpdates, swapInfo)
		case *channel.SwapChannel:
			// TODO: Add notification for swap channel
		default:
			return outgoing, fmt.Errorf("handleNotifications: Unknown related type %T", c)
		}
	}
	return outgoing, nil
}

func (e Engine) registerPaymentChannel(vfo virtualfund.Objective) error {
	postfund := vfo.V.PostFundState()
	startingBalance := big.NewInt(0)
	// TODO: Assumes one asset for now
	startingBalance.Set(postfund.Outcome[0].Allocations[0].Amount)

	return e.vm.Register(vfo.V.Id, payments.GetPayer(postfund.Participants), payments.GetPayee(postfund.Participants), startingBalance)
}

// spawnConsensusChannel will attempt to create and store a ConsensusChannel derived from the supplied Objective if it is a directfund.Objective or bridgedfund.Objective.
// The associated Channel will remain in the store.
func (e Engine) spawnConsensusChannel(crankedObjective protocols.Objective, createChannelFunc func() (*consensus_channel.ConsensusChannel, error)) error {
	c, err := createChannelFunc()
	if err != nil {
		return fmt.Errorf("could not create consensus channel for objective %s: %w", crankedObjective.Id(), err)
	}
	err = e.store.SetConsensusChannel(c)
	if err != nil {
		return fmt.Errorf("could not store consensus channel for objective %s: %w", crankedObjective.Id(), err)
	}
	// Destroy the channel since the consensus channel takes over governance:
	err = e.store.DestroyChannel(c.Id)
	if err != nil {
		return fmt.Errorf("could not destroy consensus channel for objective %s: %w", crankedObjective.Id(), err)
	}
	return nil
}

func (e Engine) spawnConsensusChannelIfDirectFundObjective(crankedObjective protocols.Objective) error {
	if dfo, isDfo := crankedObjective.(*directfund.Objective); isDfo {
		return e.spawnConsensusChannel(crankedObjective, dfo.CreateConsensusChannel)
	}
	return nil
}

func (e Engine) spawnConsensusChannelIfBridgedFundObjective(crankedObjective protocols.Objective) error {
	if bfo, isBfo := crankedObjective.(*bridgedfund.Objective); isBfo {
		return e.spawnConsensusChannel(crankedObjective, bfo.CreateConsensusChannel)
	}
	return nil
}

// destroyObjectiveAndChannelIfChallengeCleared attempts to create and store a ConsensusChannel
// derived from the supplied Objective if it is a directdefund.Objective or mirrorbridgeddefund.Objective and its challenge has been cleared.
// If successful, the associated objective and channel will be destroyed.
func (e Engine) destroyObjectiveAndChannelIfChallengeCleared(crankedObjective protocols.Objective) error {
	var objectiveToDelete protocols.Objective
	var consensusChannel *consensus_channel.ConsensusChannel
	var isObjectiveFullyWithdrawn bool

	// TODO: Create interface for defund objectives
	switch objective := crankedObjective.(type) {
	case *directdefund.Objective:
		if !objective.FullyWithdrawn() {
			cc, err := objective.CreateConsensusChannelFromChannel()
			if err != nil {
				return fmt.Errorf("could not create consensus channel for objective %s: %w", crankedObjective.Id(), err)
			}

			objectiveToDelete = objective
			consensusChannel = cc
			isObjectiveFullyWithdrawn = true
		}

	case *mirrorbridgeddefund.Objective:
		if !objective.FullyWithdrawn() {
			cc, err := objective.CreateConsensusChannelFromChannel()
			if err != nil {
				return fmt.Errorf("could not create consensus channel for objective %s: %w", crankedObjective.Id(), err)
			}

			objectiveToDelete = objective
			consensusChannel = cc
			isObjectiveFullyWithdrawn = true
		}
	}

	if isObjectiveFullyWithdrawn {
		err := e.store.SetConsensusChannel(consensusChannel)
		if err != nil {
			return fmt.Errorf("could not store consensus channel for objective %s: %w", crankedObjective.Id(), err)
		}

		err = e.store.DestroyObjective(objectiveToDelete.Id())
		if err != nil {
			return fmt.Errorf("could not destroy objective %s: %w", crankedObjective.Id(), err)
		}

		// Destroy the channel since the consensus channel takes over governance:
		err = e.store.DestroyChannel(consensusChannel.Id)
		if err != nil {
			return fmt.Errorf("could not destroy consensus channel for objective %s: %w", crankedObjective.Id(), err)
		}
	}

	return nil
}

// getOrCreateObjective retrieves the objective from the store.
// If the objective does not exist, it creates the objective using the supplied payload and stores it in the store
func (e *Engine) getOrCreateObjective(p protocols.ObjectivePayload) (protocols.Objective, error) {
	id := p.ObjectiveId
	objective, err := e.store.GetObjectiveById(id)

	if err == nil {
		return objective, nil
	} else if errors.Is(err, store.ErrNoSuchObjective) {

		newObj, err := e.constructObjectiveFromMessage(id, p)
		if err != nil {
			return nil, fmt.Errorf("error constructing objective from message: %w", err)
		}

		err = e.store.SetObjective(newObj)
		if err != nil {
			return nil, fmt.Errorf("error setting objective in store: %w", err)
		}
		e.logger.Info("Created new objective from message", "id", id)

		return newObj, nil

	} else {
		return nil, &ErrGetObjective{err, id}
	}
}

// constructObjectiveFromMessage Constructs a new objective (of the appropriate concrete type) from the supplied payload.
func (e *Engine) constructObjectiveFromMessage(id protocols.ObjectiveId, p protocols.ObjectivePayload) (protocols.Objective, error) {
	e.logger.Info("Constructing objective from message", logging.WithObjectiveIdAttribute(id))
	switch {
	case directfund.IsDirectFundObjective(id):

		dfo, err := directfund.ConstructFromPayload(false, p, *e.store.GetAddress())
		return &dfo, err
	case virtualfund.IsVirtualFundObjective(id):
		vfo, err := virtualfund.ConstructObjectiveFromPayload(p, false, *e.store.GetAddress(), e.store.GetConsensusChannel)
		if err != nil {
			return &virtualfund.Objective{}, fromMsgErr(id, err)
		}
		err = e.registerPaymentChannel(vfo)
		if err != nil {
			return &virtualfund.Objective{}, fmt.Errorf("could not register channel with payment/receipt manager.\n\ttarget channel: %s\n\terr: %w", id, err)
		}
		return &vfo, nil
	case swapfund.IsSwapFundObjective(id):
		sfo, err := swapfund.ConstructObjectiveFromPayload(p, false, *e.store.GetAddress(), e.store.GetConsensusChannel)
		if err != nil {
			return &swapfund.Objective{}, fromMsgErr(id, err)
		}

		return &sfo, nil
	case swap.IsSwapObjective(id):
		so, err := swap.ConstructObjectiveFromPayload(p, false, e.store.GetChannelById, *e.store.GetAddress())
		return &so, err
	case virtualdefund.IsVirtualDefundObjective(id):
		vId, err := virtualdefund.GetVirtualChannelFromObjectiveId(id)
		if err != nil {
			return &virtualdefund.Objective{}, fmt.Errorf("could not determine virtual channel id from objective %s: %w", id, err)
		}
		minAmount := big.NewInt(0)
		if e.vm.ChannelRegistered(vId) {
			paid, err := e.vm.Paid(vId)
			if err != nil {
				return &virtualdefund.Objective{}, fmt.Errorf("could not determine virtual channel id from objective %s: %w", id, err)
			}
			minAmount = paid
		}

		vdfo, err := virtualdefund.ConstructObjectiveFromPayload(p, false, *e.store.GetAddress(), e.store.GetChannelById, e.store.GetConsensusChannel, minAmount)
		if err != nil {
			return &virtualfund.Objective{}, fromMsgErr(id, err)
		}
		return &vdfo, nil
	case swapdefund.IsSwapDefundObjective(id):
		sdfo, err := swapdefund.ConstructObjectiveFromPayload(p, false, *e.store.GetAddress(), e.store.GetChannelById, e.store.GetConsensusChannel)
		if err != nil {
			return &virtualfund.Objective{}, fromMsgErr(id, err)
		}
		return &sdfo, nil
	case directdefund.IsDirectDefundObjective(id):
		ddfo, err := directdefund.ConstructObjectiveFromPayload(p, false, e.store.GetConsensusChannelById, e.store.GetChannelById, e.vm.GetVoucherIfAmountPresent)
		if err != nil {
			return &directdefund.Objective{}, fromMsgErr(id, err)
		}
		return &ddfo, nil

	case bridgedfund.IsBridgedFundObjective(id):
		bfo, err := bridgedfund.ConstructFromPayload(false, p, *e.store.GetAddress())
		if err != nil {
			return &bridgedfund.Objective{}, fromMsgErr(id, err)
		}
		return &bfo, err

	case bridgeddefund.IsBridgedDefundObjective(id):
		bdfo, err := bridgeddefund.ConstructObjectiveFromPayload(p, false, e.store.GetConsensusChannelById)
		if err != nil {
			return &bridgeddefund.Objective{}, fromMsgErr(id, err)
		}
		return &bdfo, nil

	case mirrorbridgeddefund.IsMirrorBridgedDefundObjective(id):
		mbdfo, err := mirrorbridgeddefund.ConstructObjectiveFromPayload(p, false, e.store.GetConsensusChannelById)
		if err != nil {
			return &mirrorbridgeddefund.Objective{}, fromMsgErr(id, err)
		}
		return &mbdfo, nil

	default:
		return &directfund.Objective{}, errors.New("cannot handle unimplemented objective type")
	}
}

// fromMsgErr wraps errors from objective construction functions and
// returns an error bundled with the objectiveID
func fromMsgErr(id protocols.ObjectiveId, err error) error {
	return fmt.Errorf("could not create objective from message.\n\ttarget objective: %s\n\terr: %w", id, err)
}

// getProposalObjectiveId returns the objectiveId for a proposal.
func getProposalObjectiveId(p consensus_channel.Proposal, channelType types.ChannelType) protocols.ObjectiveId {
	switch p.Type() {
	case consensus_channel.AddProposal:
		{
			var prefix string

			if channelType == types.Swap {
				prefix = swapfund.ObjectivePrefix
			} else {
				prefix = virtualfund.ObjectivePrefix
			}

			channelId := p.ToAdd.Guarantee.Target().String()
			return protocols.ObjectiveId(prefix + channelId)

		}
	case consensus_channel.RemoveProposal:
		{
			var prefix string

			if channelType == types.Swap {
				prefix = swapdefund.ObjectivePrefix
			} else {
				prefix = virtualdefund.ObjectivePrefix
			}

			channelId := p.ToRemove.Target.String()
			return protocols.ObjectiveId(prefix + channelId)

		}
	default:
		{
			panic("invalid proposal type")
		}
	}
}

// GetConsensusAppAddress returns the address of a deployed ConsensusApp (for ledger channels)
func (e *Engine) GetConsensusAppAddress() types.Address {
	return e.chain.GetConsensusAppAddress()
}

// GetVirtualPaymentAppAddress returns the address of a deployed VirtualPaymentApp
func (e *Engine) GetVirtualPaymentAppAddress() types.Address {
	return e.chain.GetVirtualPaymentAppAddress()
}

type messageDirection string

const (
	Incoming messageDirection = "Incoming"
	Outgoing messageDirection = "Outgoing"
)

// logMessage logs a message to the engine's logger
func (e *Engine) logMessage(msg protocols.Message, direction messageDirection) {
	if direction == Incoming {
		e.logger.Debug("Received message", "msg", msg.Summarize(e.getChannelTypeById))
	} else {
		e.logger.Debug("Sent message", "msg", msg.Summarize(e.getChannelTypeById))
	}
}

// processStoreChannels perform necessary actions for all channels in store
func (e *Engine) processStoreChannels(latestblock chainservice.Block) error {
	channels, err := e.store.GetAllChannels()
	if err != nil {
		return err
	}

	for _, ch := range channels {
		// Update on chain channel mode and store the channel
		prevChannelMode := ch.OnChain.ChannelMode
		ch.UpdateChannelMode(latestblock.Timestamp)
		if ch.OnChain.ChannelMode != prevChannelMode {
			err := e.store.SetChannel(ch)
			if err != nil {
				return err
			}
		}

		// Liquidate assets for finalized ledger channels
		if ch.Type == types.Ledger && ch.OnChain.ChannelMode == channel.Finalized {
			obj, ok := e.store.GetObjectiveByChannelId(ch.Id)

			if !ok {
				slog.Debug("Objective not found for liquidating the finalized ledger channel", "ledger channel", ch.Id)
				return nil
			}

			switch objective := obj.(type) {
			case *directdefund.Objective:
				if objective.C.OnChain.IsChallengeInitiatedByMe {
					_, err = e.attemptProgress(objective)
					if err != nil {
						return err
					}
				}

			case *mirrorbridgeddefund.Objective:
				if objective.C.OnChain.IsChallengeInitiatedByMe {
					_, err = e.attemptProgress(objective)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (e *Engine) checkError(err error) {
	if err != nil {
		e.logger.Error("error in run loop", "err", err)

		for _, nonFatalError := range nonFatalErrors {
			if errors.Is(err, nonFatalError) {
				return
			}
		}

		panic(err)
	}
}

func (e *Engine) GetNodeInfo() types.NodeInfo {
	return types.NodeInfo{
		SCAddress:            e.store.GetAddress().String(),
		MessageServicePeerId: e.msg.Id().String(),
	}
}

func (e *Engine) getChannelTypeById(channelId types.Destination) (types.ChannelType, error) {
	c, ok := e.store.GetChannelById(channelId)
	if ok {
		return c.Type, nil
	}

	return -1, fmt.Errorf("could not find channel for given channel ID: %v", channelId)
}

func (e *Engine) rejectSwapIfPendingExists(currentSwapObjective *swap.Objective) (protocols.Objective, error) {
	swapChannel := currentSwapObjective.C

	pendingSwap, err := e.store.GetPendingSwapByChannelId(swapChannel.Id)
	if err != nil {
		return nil, err
	}

	o, err := e.store.GetObjectiveById(protocols.ObjectiveId(swap.ObjectivePrefix + pendingSwap.Id.String()))
	if err != nil {
		return nil, err
	}

	pendingSwapObjective, ok := o.(*swap.Objective)
	if !ok {
		return nil, fmt.Errorf("expected swap objective")
	}

	currentSwapWithSender := payments.SwapWithSender{
		Swap:   currentSwapObjective.Swap,
		Sender: currentSwapObjective.C.Participants[currentSwapObjective.SwapSenderIndex],
	}

	pendingSwapWithSender := payments.SwapWithSender{
		Swap:   pendingSwapObjective.Swap,
		Sender: pendingSwapObjective.C.Participants[currentSwapObjective.SwapSenderIndex],
	}

	currentHash, err := currentSwapWithSender.Hash()
	if err != nil {
		return nil, err
	}

	slog.Debug("DEBUG: engine.go-rejectObjective", "currentHash", currentHash.String(), "myIndex", swapChannel.MyIndex)

	pendingHash, err := pendingSwapWithSender.Hash()
	if err != nil {
		return nil, err
	}

	slog.Debug("DEBUG: engine.go-rejectObjective", "pendingHash", pendingHash.String(), "myIndex", swapChannel.MyIndex)

	// Do not enter if pending swap and current swap are same and this node is not the swap channel leader
	if pendingSwap != nil && strings.Compare(pendingHash.String(), currentHash.String()) != 0 && swapChannel.MyIndex == 0 {
		var objectiveToReject protocols.Objective

		if strings.Compare(pendingHash.String(), currentHash.String()) < 0 {
			slog.Debug("DEBUG: engine.go-rejectObjective rejecting pending swap objective", "objectiveId", pendingSwapObjective.Id())
			objectiveToReject = pendingSwapObjective
		} else {
			slog.Debug("DEBUG: engine.go-rejectObjective rejecting current swap objective", "objectiveId", currentSwapObjective.Id())
			objectiveToReject = currentSwapObjective
		}

		objectiveToReject, sideEffects := objectiveToReject.Reject()
		err = e.store.SetObjective(objectiveToReject)
		if err != nil {
			return nil, err
		}

		err = e.executeSideEffects(sideEffects)
		if err != nil {
			return nil, err
		}

		return objectiveToReject, nil
	}

	return nil, nil
}
