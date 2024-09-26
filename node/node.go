// Package node contains imperative library code for running a go-nitro node inside another application.
package node // import "github.com/statechannels/go-nitro/node"

import (
	"fmt"
	"log/slog"
	"math/big"
	"runtime/debug"
	"time"

	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/internal/safesync"
	"github.com/statechannels/go-nitro/node/engine"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	"github.com/statechannels/go-nitro/node/engine/messageservice"
	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/node/notifier"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/payments"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/bridgeddefund"
	"github.com/statechannels/go-nitro/protocols/bridgedfund"
	"github.com/statechannels/go-nitro/protocols/directdefund"
	"github.com/statechannels/go-nitro/protocols/directfund"
	"github.com/statechannels/go-nitro/protocols/mirrorbridgeddefund"
	"github.com/statechannels/go-nitro/protocols/swapfund"
	"github.com/statechannels/go-nitro/protocols/virtualdefund"
	"github.com/statechannels/go-nitro/protocols/virtualfund"
	"github.com/statechannels/go-nitro/rand"
	"github.com/statechannels/go-nitro/types"
)

// Node provides the interface for the consuming application
type Node struct {
	engine                      engine.Engine // The core business logic of the node
	Address                     *types.Address
	channelNotifier             *notifier.ChannelNotifier
	completedObjectivesNotifier *notifier.CompletedObjetivesNotifier

	completedObjectives *safesync.Map[chan struct{}]
	failedObjectives    chan protocols.ObjectiveId
	receivedVouchers    chan payments.Voucher
	chainId             *big.Int
	store               store.Store
	vm                  *payments.VoucherManager
}

// New is the constructor for a Node. It accepts a messaging service, a chain service, and a store as injected dependencies.
func New(messageService messageservice.MessageService, chainservice chainservice.ChainService, store store.Store, policymaker engine.PolicyMaker) Node {
	n := Node{}
	n.Address = store.GetAddress()

	chainId, err := chainservice.GetChainId()
	if err != nil {
		panic(err)
	}
	n.chainId = chainId
	n.store = store
	n.vm = payments.NewVoucherManager(*store.GetAddress(), store)

	n.engine = engine.New(n.vm, messageService, chainservice, store, policymaker, n.handleEngineEvent)
	n.completedObjectives = &safesync.Map[chan struct{}]{}

	n.failedObjectives = make(chan protocols.ObjectiveId, 100)
	// Using a larger buffer since payments can be sent frequently.
	n.receivedVouchers = make(chan payments.Voucher, 1000)

	n.channelNotifier = notifier.NewChannelNotifier(store, n.vm)
	n.completedObjectivesNotifier = notifier.NewCompletedObjectivesNotifier()

	return n
}

// handleEngineEvents dispatches events to the necessary node chan.
func (n *Node) handleEngineEvent(update engine.EngineEvent) {
	for _, completed := range update.CompletedObjectives {
		d, _ := n.completedObjectives.LoadOrStore(string(completed.Id()), make(chan struct{}))
		close(d)
		n.completedObjectives.Delete(string(completed.Id()))

		// Broadcast completed objective ID to all listeners
		n.completedObjectivesNotifier.BroadcastCompletedObjective(completed.Id())
	}

	for _, erred := range update.FailedObjectives {
		n.failedObjectives <- erred
	}

	for _, payment := range update.ReceivedVouchers {
		n.receivedVouchers <- payment
	}

	for _, updated := range update.LedgerChannelUpdates {

		err := n.channelNotifier.NotifyLedgerUpdated(updated)
		n.handleError(err)
	}
	for _, updated := range update.PaymentChannelUpdates {

		err := n.channelNotifier.NotifyPaymentUpdated(updated)
		n.handleError(err)
	}
}

// Begin API

// Version returns the go-nitro version
func (n *Node) Version() string {
	info, _ := debug.ReadBuildInfo()

	version := info.Main.Version
	// Depending on how the binary was built we may get back no version info.
	// In this case we default to "(devel)".
	// See https://github.com/golang/go/issues/51831#issuecomment-1074188363 for more details.
	if version == "" {
		version = "(devel)"
	}

	// If the binary was built with the -buildvcs flag we can get the git commit hash and use that as the version.
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			version = s.Value
			break
		}
	}

	return version
}

// LedgerUpdates returns a chan that receives ledger channel info whenever that ledger channel is updated. Not suitable for multiple subscribers.
func (n *Node) LedgerUpdates() <-chan query.LedgerChannelInfo {
	return n.channelNotifier.RegisterForAllLedgerUpdates()
}

// PaymentUpdates returns a chan that receives payment channel info whenever that payment channel is updated. Not suitable fo multiple subscribers.
func (n *Node) PaymentUpdates() <-chan query.PaymentChannelInfo {
	return n.channelNotifier.RegisterForAllPaymentUpdates()
}

// CompletedObjectives returns a chan that receives a objective ID whenever that objective is completed
func (n *Node) CompletedObjectives() <-chan protocols.ObjectiveId {
	return n.completedObjectivesNotifier.RegisterForAllCompletedObjectives()
}

// ObjectiveCompleteChan returns a chan that is closed when the objective with given id is completed
func (n *Node) ObjectiveCompleteChan(id protocols.ObjectiveId) <-chan struct{} {
	d, _ := n.completedObjectives.LoadOrStore(string(id), make(chan struct{}))
	return d
}

// LedgerUpdatedChan returns a chan that receives a ledger channel info whenever the ledger with given id is updated
func (n *Node) LedgerUpdatedChan(ledgerId types.Destination) <-chan query.LedgerChannelInfo {
	return n.channelNotifier.RegisterForLedgerUpdates(ledgerId)
}

// PaymentChannelUpdatedChan returns a chan that receives a payment channel info whenever the payment channel with given id is updated
func (n *Node) PaymentChannelUpdatedChan(ledgerId types.Destination) <-chan query.PaymentChannelInfo {
	return n.channelNotifier.RegisterForPaymentChannelUpdates(ledgerId)
}

// FailedObjectives returns a chan that receives an objective id whenever that objective has failed
func (n *Node) FailedObjectives() <-chan protocols.ObjectiveId {
	return n.failedObjectives
}

// ReceivedVouchers returns a chan that receives a voucher every time we receive a payment voucher
func (n *Node) ReceivedVouchers() <-chan payments.Voucher {
	return n.receivedVouchers
}

// CreateVoucher creates and returns a voucher for the given channelId which increments the redeemable balance by amount.
// It is the responsibility of the caller to send the voucher to the payee.
func (n *Node) CreateVoucher(channelId types.Destination, amount *big.Int) (payments.Voucher, error) {
	voucher, err := n.vm.Pay(channelId, amount, *n.store.GetChannelSecretKey())
	if err != nil {
		return payments.Voucher{}, err
	}
	info, err := n.GetPaymentChannel(channelId)
	if err != nil {
		return voucher, err
	}
	err = n.channelNotifier.NotifyPaymentUpdated(info)
	if err != nil {
		return voucher, err
	}
	return voucher, nil
}

// ReceiveVoucher receives a voucher and returns the amount that was paid.
// It can be used to add a voucher that was sent outside of the go-nitro system.
func (c *Node) ReceiveVoucher(v payments.Voucher) (payments.ReceiveVoucherSummary, error) {
	total, delta, err := c.vm.Receive(v)
	return payments.ReceiveVoucherSummary{Total: total, Delta: delta}, err
}

func (n *Node) CreateSwapChannel(Intermediaries []types.Address, CounterParty types.Address, ChallengeDuration uint32, Outcome outcome.Exit) (swapfund.ObjectiveResponse, error) {
	objectiveRequest := swapfund.NewObjectiveRequest(
		Intermediaries,
		CounterParty,
		ChallengeDuration,
		Outcome,
		rand.Uint64(),
		// Since no contract present for swap channels yet
		// TODO: Handle sad path for swap channels
		n.engine.GetConsensusAppAddress(),
	)

	// Send the event to the engine
	n.engine.ObjectiveRequestsFromAPI <- objectiveRequest

	objectiveRequest.WaitForObjectiveToStart()
	return objectiveRequest.Response(*n.Address), nil
}

// CreatePaymentChannel creates a virtual channel with the counterParty using ledger channels
// with the supplied intermediaries.
func (n *Node) CreatePaymentChannel(Intermediaries []types.Address, CounterParty types.Address, ChallengeDuration uint32, Outcome outcome.Exit) (virtualfund.ObjectiveResponse, error) {
	objectiveRequest := virtualfund.NewObjectiveRequest(
		Intermediaries,
		CounterParty,
		ChallengeDuration,
		Outcome,
		rand.Uint64(),
		n.engine.GetVirtualPaymentAppAddress(),
	)

	// Send the event to the engine
	n.engine.ObjectiveRequestsFromAPI <- objectiveRequest

	objectiveRequest.WaitForObjectiveToStart()
	return objectiveRequest.Response(*n.Address), nil
}

// ClosePaymentChannel attempts to close and defund the given virtually funded channel.
func (n *Node) ClosePaymentChannel(channelId types.Destination) (protocols.ObjectiveId, error) {
	objectiveRequest := virtualdefund.NewObjectiveRequest(channelId)

	// Send the event to the engine
	n.engine.ObjectiveRequestsFromAPI <- objectiveRequest
	objectiveRequest.WaitForObjectiveToStart()
	return objectiveRequest.Id(*n.Address, n.chainId), nil
}

// CreateLedgerChannel creates a directly funded ledger channel with the given counterparty.
// The channel will run under full consensus rules (it is not possible to provide a custom AppDefinition or AppData).
func (n *Node) CreateLedgerChannel(Counterparty types.Address, ChallengeDuration uint32, outcome outcome.Exit) (directfund.ObjectiveResponse, error) {
	objectiveRequest := directfund.NewObjectiveRequest(
		Counterparty,
		ChallengeDuration,
		outcome,
		rand.Uint64(),
		n.engine.GetConsensusAppAddress(),
		// Appdata implicitly zero
	)

	// Check store to see if there is an existing channel with this counterparty
	channelExists, err := directfund.ChannelsExistWithCounterparty(Counterparty, n.store.GetChannelsByParticipant, n.store.GetConsensusChannel)
	if err != nil {
		slog.Error("direct fund error", "error", err)
		return directfund.ObjectiveResponse{}, fmt.Errorf("counterparty check failed: %w", err)
	}
	if channelExists {
		slog.Error("directfund: channel already exists", "error", directfund.ErrLedgerChannelExists)

		return directfund.ObjectiveResponse{}, fmt.Errorf("counterparty %s: %w", Counterparty, directfund.ErrLedgerChannelExists)
	}

	// Send the event to the engine
	n.engine.ObjectiveRequestsFromAPI <- objectiveRequest
	objectiveRequest.WaitForObjectiveToStart()
	return objectiveRequest.Response(*n.Address, n.chainId), nil
}

// Uses bridgedfund protocol to create a bridge channel (to be called by L2 nodes)
// No chain interactions are involved while creating this channel
func (n *Node) CreateBridgeChannel(Counterparty types.Address, ChallengeDuration uint32, outcome outcome.Exit) (bridgedfund.ObjectiveResponse, error) {
	objectiveRequest := bridgedfund.NewObjectiveRequest(
		Counterparty,
		ChallengeDuration,
		outcome,
		rand.Uint64(),
		n.engine.GetConsensusAppAddress(),
		// Appdata implicitly zero
	)

	// Check store to see if there is an existing channel with this counterparty
	channelExists, err := bridgedfund.ChannelsExistWithCounterparty(Counterparty, n.store.GetChannelsByParticipant, n.store.GetConsensusChannel)
	if err != nil {
		slog.Error("bridge fund error", "error", err)
		return bridgedfund.ObjectiveResponse{}, fmt.Errorf("counterparty check failed: %w", err)
	}
	if channelExists {
		slog.Error("bridgefund: channel already exists", "error", bridgedfund.ErrLedgerChannelExists)

		return bridgedfund.ObjectiveResponse{}, fmt.Errorf("counterparty %s: %w", Counterparty, bridgedfund.ErrLedgerChannelExists)
	}

	// Send the event to the engine
	n.engine.ObjectiveRequestsFromAPI <- objectiveRequest
	objectiveRequest.WaitForObjectiveToStart()
	return objectiveRequest.Response(*n.Address, n.chainId), nil
}

// CloseLedgerChannel attempts to close and defund the given directly funded channel.
func (n *Node) CloseLedgerChannel(channelId types.Destination, isChallenge bool) (protocols.ObjectiveId, error) {
	objectiveRequest := directdefund.NewObjectiveRequest(channelId, isChallenge)

	// Send the event to the engine
	n.engine.ObjectiveRequestsFromAPI <- objectiveRequest
	objectiveRequest.WaitForObjectiveToStart()
	return objectiveRequest.Id(*n.Address, n.chainId), nil
}

func (n *Node) CloseBridgeChannel(channelId types.Destination) (protocols.ObjectiveId, error) {
	objectiveRequest := bridgeddefund.NewObjectiveRequest(channelId)

	// Send the event to the engine
	n.engine.ObjectiveRequestsFromAPI <- objectiveRequest
	objectiveRequest.WaitForObjectiveToStart()
	return objectiveRequest.Id(*n.Address, n.chainId), nil
}

func (n *Node) MirrorBridgedDefund(l1ChannelId types.Destination, l2SignedState state.SignedState, isChallenge bool) (protocols.ObjectiveId, error) {
	objectiveRequest := mirrorbridgeddefund.NewObjectiveRequest(l1ChannelId, l2SignedState, isChallenge)

	// Send the event to the engine
	n.engine.ObjectiveRequestsFromAPI <- objectiveRequest
	objectiveRequest.WaitForObjectiveToStart()
	return objectiveRequest.Id(*n.Address, n.chainId), nil
}

// Pay will send a signed voucher to the payee that they can redeem for the given amount.
func (n *Node) Pay(channelId types.Destination, amount *big.Int) error {
	paymentChannel, err := n.GetPaymentChannel(channelId)
	if err != nil {
		return err
	}

	if types.Gt(amount, (*big.Int)(paymentChannel.Balance.RemainingFunds)) {
		return fmt.Errorf("error making payment request: insufficient funds")
	}

	// Send the event to the engine
	n.engine.PaymentRequestsFromAPI <- engine.PaymentRequest{ChannelId: channelId, Amount: amount}
	return nil
}

// GetPaymentChannel returns the payment channel with the given id.
// If no ledger channel exists with the given id an error is returned.
func (n *Node) GetPaymentChannel(id types.Destination) (query.PaymentChannelInfo, error) {
	return query.GetPaymentChannelInfo(id, n.store, n.vm)
}

func (n *Node) GetSwapChannel(id types.Destination) (string, error) {
	return query.GetSwapChannelInfo(id, n.store)
}

// GetPaymentChannelsByLedger returns all active payment channels that are funded by the given ledger channel.
func (n *Node) GetPaymentChannelsByLedger(ledgerId types.Destination) ([]query.PaymentChannelInfo, error) {
	return query.GetPaymentChannelsByLedger(ledgerId, n.store, n.vm)
}

func (n *Node) GetVoucher(id types.Destination) payments.Voucher {
	var voucher payments.Voucher
	voucherInfo, voucherFound := n.vm.GetVoucherIfAmountPresent(id)

	if voucherFound {
		voucher = voucherInfo.LargestVoucher
	}

	return voucher
}

// GetAllLedgerChannels returns all ledger channels.
func (n *Node) GetAllLedgerChannels() ([]query.LedgerChannelInfo, error) {
	return query.GetAllLedgerChannels(n.store, n.engine.GetConsensusAppAddress())
}

// GetLastBlockNum returns last confirmed blockNum read from store
func (n *Node) GetLastBlockNum() (uint64, error) {
	return n.store.GetLastBlockNumSeen()
}

// GetLedgerChannel returns the ledger channel with the given id.
// If no ledger channel exists with the given id an error is returned.
func (n *Node) GetLedgerChannel(id types.Destination) (query.LedgerChannelInfo, error) {
	return query.GetLedgerChannelInfo(id, n.store)
}

func (n *Node) GetSignedState(id types.Destination) (state.SignedState, error) {
	consensusChannel, err := n.store.GetConsensusChannelById(id)
	if err != nil {
		return state.SignedState{}, err
	}

	return consensusChannel.SupportedSignedState(), nil
}

func (n *Node) GetObjectiveById(objectiveId protocols.ObjectiveId) (protocols.Objective, error) {
	return n.store.GetObjectiveById(objectiveId)
}

// Close stops the node from responding to any input.
func (n *Node) Close() error {
	if err := n.engine.Close(); err != nil {
		return err
	}
	if err := n.channelNotifier.Close(); err != nil {
		return err
	}

	if err := n.completedObjectivesNotifier.Close(); err != nil {
		return err
	}

	return n.store.Close()
}

// handleError logs the error and panics
// Eventually it should return the error to the caller
func (n *Node) handleError(err error) {
	if err != nil {

		slog.Error("Error in nitro node", "error", err)

		<-time.After(1000 * time.Millisecond) // We wait for a bit so the previous log line has time to complete

		// TODO instead of a panic, errors should be returned to the caller.
		panic(err)

	}
}

func (n *Node) CounterChallenge(id types.Destination, action types.CounterChallengeAction, payload state.SignedState) {
	n.engine.CounterChallengeRequestsFromAPI <- engine.CounterChallengeRequest{ChannelId: id, Action: action, Payload: payload}
}

func (n *Node) RetryObjectiveTx(objectiveId protocols.ObjectiveId) {
	n.engine.RetryObjectiveTxRequestFromAPI <- types.RetryObjectiveTxRequest{ObjectiveId: string(objectiveId)}
}

func (n *Node) GetNodeInfo() types.NodeInfo {
	return n.engine.GetNodeInfo()
}
