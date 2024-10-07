package bridgedfund // import "github.com/statechannels/go-nitro/bridgedfund"

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/consensus_channel"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

var ErrLedgerChannelExists error = errors.New("bridgedfund: ledger channel already exists")

const (
	WaitingForCompletePrefund  protocols.WaitingFor = "WaitingForCompletePrefund"
	WaitingForCompletePostFund protocols.WaitingFor = "WaitingForCompletePostFund"
	WaitingForNothing          protocols.WaitingFor = "WaitingForNothing" // Finished
)

const (
	SignedStatePayload protocols.PayloadType = "SignedStatePayload"
)

const ObjectivePrefix = "BridgedFunding-"

func FundOnChainEffect(cId types.Destination, asset string, amount types.Funds) string {
	return "deposit" + amount.String() + "into" + cId.String()
}

// Objective is a cache of data computed by reading from the store. It stores (potentially) infinite data
type Objective struct {
	Status protocols.ObjectiveStatus
	C      *channel.Channel
}

// GetChannelByIdFunction specifies a function that can be used to retrieve channels from a store.
type GetChannelsByParticipantFunction func(participant types.Address) ([]*channel.Channel, error)

// GetTwoPartyConsensusLedgerFuncion describes functions which return a ConsensusChannel ledger channel between
// the calling client and the given counterparty, if such a channel exists.
type GetTwoPartyConsensusLedgerFunction func(counterparty types.Address) (ledger *consensus_channel.ConsensusChannel, ok bool)

// NewObjective creates a new bridged funding objective from a given request.
func NewObjective(request ObjectiveRequest, preApprove bool, myAddress types.Address, chainId *big.Int, getChannels GetChannelsByParticipantFunction, getTwoPartyConsensusLedger GetTwoPartyConsensusLedgerFunction) (Objective, error) {
	channelExists, err := ChannelsExistWithCounterparty(request.CounterParty, getChannels, getTwoPartyConsensusLedger)
	if err != nil {
		return Objective{}, fmt.Errorf("counterparty check failed: %w", err)
	}
	if channelExists {
		return Objective{}, fmt.Errorf("counterparty %s: %w", request.CounterParty, ErrLedgerChannelExists)
	}

	initialState := state.State{
		Participants:      []types.Address{myAddress, request.CounterParty},
		ChannelNonce:      request.Nonce,
		AppDefinition:     request.AppDefinition,
		ChallengeDuration: request.ChallengeDuration,
		AppData:           request.AppData,
		Outcome:           request.Outcome,
		TurnNum:           0,
		IsFinal:           false,
	}

	// TODO: Refactor so the main logic is contained in NewObjective and have ConstructFromPayload call that
	signedInitial := state.NewSignedState(initialState)
	b, err := json.Marshal(signedInitial)
	if err != nil {
		return Objective{}, fmt.Errorf("could not create new objective: %w", err)
	}
	objective, err := ConstructFromPayload(preApprove,
		protocols.ObjectivePayload{ObjectiveId: request.Id(myAddress, chainId), PayloadData: b, Type: SignedStatePayload},
		myAddress,
	)
	if err != nil {
		return Objective{}, fmt.Errorf("could not create new objective: %w", err)
	}
	return objective, nil
}

// ChannelsExistWithCounterparty returns true if a channel or consensus_channel exists with the counterparty
func ChannelsExistWithCounterparty(counterparty types.Address, getChannels GetChannelsByParticipantFunction, getTwoPartyConsensusLedger GetTwoPartyConsensusLedgerFunction) (bool, error) {
	// check for any channels that may be in the process of bridged funding
	channels, err := getChannels(counterparty)
	if err != nil {
		return false, err
	}
	for _, c := range channels {
		// We only want to find bridged funded channels that would have two participants
		if len(c.Participants) == 2 {
			return true, nil
		}
	}

	_, ok := getTwoPartyConsensusLedger(counterparty)

	return ok, nil
}

// ConstructFromPayload initiates a Objective with data calculated from
// the supplied initialState and client address
func ConstructFromPayload(
	preApprove bool,
	op protocols.ObjectivePayload,
	myAddress types.Address,
) (Objective, error) {
	var err error
	initialSignedState, err := getSignedStatePayload(op.PayloadData)
	if err != nil {
		return Objective{}, fmt.Errorf("could not get signed state payload: %w", err)
	}

	initialState := initialSignedState.State()
	err = initialState.FixedPart().Validate()
	if err != nil {
		return Objective{}, err
	}
	if initialState.TurnNum != 0 {
		return Objective{}, errors.New("cannot construct bridged fund objective without prefund state")
	}
	if initialState.IsFinal {
		return Objective{}, errors.New("attempted to initiate new bridged-funding objective with IsFinal == true")
	}
	init := Objective{}

	if preApprove {
		init.Status = protocols.Approved
	} else {
		init.Status = protocols.Unapproved
	}
	var myIndex uint
	foundMyAddress := false
	for i, v := range initialState.Participants {
		if v == myAddress {
			myIndex = uint(i)
			foundMyAddress = true
			break
		}
	}
	if !foundMyAddress {
		return Objective{}, errors.New("my address not found in participants")
	}

	init.C = &channel.Channel{}
	init.C, err = channel.New(initialState, myIndex, types.Ledger)
	if err != nil {
		return Objective{}, fmt.Errorf("failed to initialize channel for bridged-fund objective: %w", err)
	}

	return init, nil
}

// OwnsChannel returns the channel that the objective is funding.
func (dfo *Objective) OwnsChannel() types.Destination {
	return dfo.C.Id
}

// GetStatus returns the status of the objective.
func (dfo *Objective) GetStatus() protocols.ObjectiveStatus {
	return dfo.Status
}

// CreateConsensusChannel creates a ConsensusChannel from the Objective by extracting signatures and a single asset outcome from the post fund state.
func (dfo *Objective) CreateConsensusChannel() (*consensus_channel.ConsensusChannel, error) {
	ledger := dfo.C

	if !ledger.PostFundComplete() {
		return nil, fmt.Errorf("expected funding for channel %s to be complete", dfo.C.Id)
	}
	signedPostFund := ledger.SignedPostFundState()
	leaderSig, err := signedPostFund.GetParticipantSignature(uint(consensus_channel.Leader))
	if err != nil {
		return nil, fmt.Errorf("could not get leader signature: %w", err)
	}
	followerSig, err := signedPostFund.GetParticipantSignature(uint(consensus_channel.Follower))
	if err != nil {
		return nil, fmt.Errorf("could not get follower signature: %w", err)
	}
	signatures := [2]state.Signature{leaderSig, followerSig}

	assetsExit := signedPostFund.State().Outcome
	turnNum := signedPostFund.State().TurnNum
	outcomes, err := consensus_channel.FromExit(assetsExit)
	if err != nil {
		return nil, fmt.Errorf("could not create ledger outcome from channel exit: %w", err)
	}

	if ledger.MyIndex == uint(consensus_channel.Leader) {
		con, err := consensus_channel.NewLeaderChannel(ledger.FixedPart, turnNum, outcomes, signatures)
		con.OnChainFunding = ledger.OnChain.Holdings.Clone() // Copy OnChain.Holdings so we don't lose this information
		if err != nil {
			return nil, fmt.Errorf("could not create consensus channel as leader: %w", err)
		}
		return &con, nil

	} else {
		con, err := consensus_channel.NewFollowerChannel(ledger.FixedPart, turnNum, outcomes, signatures)
		con.OnChainFunding = ledger.OnChain.Holdings.Clone() // Copy OnChain.Holdings so we don't lose this information
		if err != nil {
			return nil, fmt.Errorf("could not create consensus channel as follower: %w", err)
		}
		return &con, nil
	}
}

// Public methods on the BridgedFundingObjectiveState

func (o *Objective) Id() protocols.ObjectiveId {
	return protocols.ObjectiveId(ObjectivePrefix + o.C.Id.String())
}

func (o *Objective) Approve() protocols.Objective {
	updated := o.clone()
	// todo: consider case of s.Status == Rejected
	updated.Status = protocols.Approved

	return &updated
}

func (o *Objective) Reject() (protocols.Objective, protocols.SideEffects) {
	updated := o.clone()

	updated.Status = protocols.Rejected
	peer := o.C.Participants[1-o.C.MyIndex]

	sideEffects := protocols.SideEffects{MessagesToSend: protocols.CreateRejectionNoticeMessage(o.Id(), peer)}
	return &updated, sideEffects
}

// Update receives an ObjectivePayload, applies all applicable data to the BridgedFundingObjectiveState,
// and returns the updated state
func (o *Objective) Update(p protocols.ObjectivePayload) (protocols.Objective, error) {
	if o.Id() != p.ObjectiveId {
		return o, fmt.Errorf("event and objective Ids do not match: %s and %s respectively", string(p.ObjectiveId), string(o.Id()))
	}

	updated := o.clone()
	ss, err := getSignedStatePayload(p.PayloadData)
	if err != nil {
		if err != nil {
			return o, fmt.Errorf("could not get signed state payload: %w", err)
		}
	}
	updated.C.AddSignedState(ss)
	return &updated, nil
}

func (o *Objective) otherParticipants() []types.Address {
	others := make([]types.Address, 0)
	for i, p := range o.C.Participants {
		if i != int(o.C.MyIndex) {
			others = append(others, p)
		}
	}
	return others
}

// Crank inspects the extended state and declares a list of Effects to be executed
// It's like a state machine transition function where the finite / enumerable state is returned (computed from the extended state)
// rather than being independent of the extended state; and where there is only one type of event ("the crank") with no data on it at all
func (o *Objective) Crank(secretKey *[]byte) (protocols.Objective, protocols.SideEffects, protocols.WaitingFor, error) {
	updated := o.clone()

	sideEffects := protocols.SideEffects{}
	// Input validation
	if updated.Status != protocols.Approved {
		return &updated, protocols.SideEffects{}, WaitingForNothing, protocols.ErrNotApproved
	}

	// Prefunding
	if !updated.C.PreFundSignedByMe() {
		ss, err := updated.C.SignAndAddPrefund(secretKey)
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForCompletePrefund, fmt.Errorf("could not sign prefund %w", err)
		}
		messages, err := protocols.CreateObjectivePayloadMessage(updated.Id(), ss, SignedStatePayload, updated.otherParticipants()...)
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForCompletePrefund, fmt.Errorf("could not create payload message %w", err)
		}
		sideEffects.MessagesToSend = append(sideEffects.MessagesToSend, messages...)
	}

	if !updated.C.PreFundComplete() {
		return &updated, sideEffects, WaitingForCompletePrefund, nil
	}

	// Postfunding
	if !updated.C.PostFundSignedByMe() {

		ss, err := updated.C.SignAndAddPostfund(secretKey)
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForCompletePostFund, fmt.Errorf("could not sign postfund %w", err)
		}
		messages, err := protocols.CreateObjectivePayloadMessage(updated.Id(), ss, SignedStatePayload, updated.otherParticipants()...)
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForCompletePostFund, fmt.Errorf("could not create paylaod message %w", err)
		}
		sideEffects.MessagesToSend = append(sideEffects.MessagesToSend, messages...)
	}

	if !updated.C.PostFundComplete() {
		return &updated, sideEffects, WaitingForCompletePostFund, nil
	}

	// Completion
	updated.Status = protocols.Completed

	return &updated, sideEffects, WaitingForNothing, nil
}

func (o *Objective) Related() []protocols.Storable {
	return []protocols.Storable{o.C}
}

//  Private methods on the BridgedFundingObjectiveState

// clone returns a deep copy of the receiver.
func (o *Objective) clone() Objective {
	clone := Objective{}
	clone.Status = o.Status

	cClone := o.C.Clone()
	clone.C = cClone

	return clone
}

// IsBridgedFundObjective inspects a objective id and returns true if the objective id is for a bridged fund objective.
func IsBridgedFundObjective(id protocols.ObjectiveId) bool {
	return strings.HasPrefix(string(id), ObjectivePrefix)
}

// ObjectiveRequest represents a request to create a new bridged funding objective.
type ObjectiveRequest struct {
	CounterParty      types.Address
	ChallengeDuration uint32
	Outcome           outcome.Exit
	AppDefinition     types.Address
	AppData           types.Bytes
	Nonce             uint64
	objectiveStarted  chan struct{}
}

// NewObjectiveRequest creates a new ObjectiveRequest.
func NewObjectiveRequest(
	counterparty types.Address,
	challengeDuration uint32,
	outcome outcome.Exit,
	nonce uint64,
	appDefinition types.Address,
) ObjectiveRequest {
	return ObjectiveRequest{
		CounterParty:      counterparty,
		ChallengeDuration: challengeDuration,
		Outcome:           outcome,
		Nonce:             nonce,
		AppDefinition:     appDefinition,
		objectiveStarted:  make(chan struct{}),
	}
}

// SignalObjectiveStarted is used by the engine to signal the objective has been started.
func (r ObjectiveRequest) SignalObjectiveStarted() {
	close(r.objectiveStarted)
}

// WaitForObjectiveToStart blocks until the objective starts
func (r ObjectiveRequest) WaitForObjectiveToStart() {
	<-r.objectiveStarted
}

// Id returns the objective id for the request.
func (r ObjectiveRequest) Id(myAddress types.Address, chainId *big.Int) protocols.ObjectiveId {
	fixedPart := state.FixedPart{
		Participants:      []types.Address{myAddress, r.CounterParty},
		ChannelNonce:      r.Nonce,
		ChallengeDuration: r.ChallengeDuration,
	}

	channelId := fixedPart.ChannelId()
	return protocols.ObjectiveId(ObjectivePrefix + channelId.String())
}

// ObjectiveResponse is the type returned across the API in response to the ObjectiveRequest.
type ObjectiveResponse struct {
	Id        protocols.ObjectiveId
	ChannelId types.Destination
}

// Response computes and returns the appropriate response from the request.
func (r ObjectiveRequest) Response(myAddress types.Address, chainId *big.Int) ObjectiveResponse {
	fixedPart := state.FixedPart{
		Participants:      []types.Address{myAddress, r.CounterParty},
		ChannelNonce:      r.Nonce,
		ChallengeDuration: r.ChallengeDuration,
		AppDefinition:     r.AppDefinition,
	}

	channelId := fixedPart.ChannelId()

	return ObjectiveResponse{
		Id:        protocols.ObjectiveId(ObjectivePrefix + channelId.String()),
		ChannelId: channelId,
	}
}

// getSignedStatePayload takes in a serialized signed state payload and returns the deserialized SignedState.
func getSignedStatePayload(b []byte) (state.SignedState, error) {
	ss := state.SignedState{}
	err := json.Unmarshal(b, &ss)
	if err != nil {
		return ss, fmt.Errorf("could not unmarshal signed state: %w", err)
	}
	return ss, nil
}

// TODO: Add mermaid diagram
