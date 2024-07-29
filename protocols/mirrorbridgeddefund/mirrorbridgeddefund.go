package mirrorbridgeddefund

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/consensus_channel"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

const ObjectivePrefix = "mirrorbridgeddefunding-"

const (
	WaitingForFinalization protocols.WaitingFor = "WaitingForFinalization"
	WaitingForNothing      protocols.WaitingFor = "WaitingForNothing" // Finished
	WaitingForWithdraw     protocols.WaitingFor = "WaitingForWithdraw"
)

const (
	SignedStatePayload protocols.PayloadType = "SignedStatePayload"
)

var ErrChannelNotExist error = errors.New("could not find channel")

const (
	ErrNoFinalState = types.ConstError("cannot spawn direct defund objective without a final state")
)

// Objective is a cache of data computed by reading from the store. It stores (potentially) infinite data
type Objective struct {
	Status                     protocols.ObjectiveStatus
	C                          *channel.Channel
	L2SignedState              state.SignedState
	MirrorTransactionSubmitted bool
}

// GetConsensusChannel describes functions which return a ConsensusChannel ledger channel for a channel id.
type GetConsensusChannel func(channelId types.Destination) (ledger *consensus_channel.ConsensusChannel, err error)

// NewObjective initiates an Objective with the supplied channel
func NewObjective(
	request ObjectiveRequest,
	preApprove bool,
	getConsensusChannel GetConsensusChannel,
	isObjectiveInitiator bool,
) (Objective, error) {
	cc, err := getConsensusChannel(request.l1ChannelId)
	if err != nil {
		return Objective{}, fmt.Errorf("%w %s: %w", ErrChannelNotExist, request.l1ChannelId, err)
	}

	c, err := CreateChannelFromConsensusChannel(*cc)
	if err != nil {
		return Objective{}, fmt.Errorf("could not create Channel from ConsensusChannel; %w", err)
	}

	init := Objective{}

	if preApprove {
		init.Status = protocols.Approved
	} else {
		init.Status = protocols.Unapproved
	}
	init.C = c.Clone()

	init.L2SignedState = request.l2SignedState

	return init, nil
}

// GetStatus returns the status of the objective.
func (o *Objective) GetStatus() protocols.ObjectiveStatus {
	return o.Status
}

// OwnsChannel returns the channel that the objective is funding.
func (o *Objective) OwnsChannel() types.Destination {
	return o.C.Id
}

func (o *Objective) Related() []protocols.Storable {
	return []protocols.Storable{o.C}
}

func (o *Objective) Id() protocols.ObjectiveId {
	return protocols.ObjectiveId(ObjectivePrefix + o.C.Id.String())
}

// Crank inspects the extended state and declares a list of Effects to be executed
// It's like a state machine transition function where the finite / enumerable state is returned (computed from the extended state)
// rather than being independent of the extended state; and where there is only one type of event ("the crank") with no data on it at all
func (o *Objective) Crank(secretKey *[]byte) (protocols.Objective, protocols.SideEffects, protocols.WaitingFor, error) {
	updated := o.clone()
	sideEffects := protocols.SideEffects{}

	if updated.Status != protocols.Approved {
		return &updated, sideEffects, WaitingForNothing, protocols.ErrNotApproved
	}

	if len(updated.L2SignedState.Signatures()) != 0 && !updated.MirrorTransactionSubmitted {
		// Create updated L1 state based on the variable part of the L2 state
		updatedL1State, err := o.CreateL1StateBasedOnL2()
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForFinalization, err
		}

		// Sign the updated L1 state
		updatedL1SignedState, err := updated.C.SignAndAddState(updatedL1State, secretKey)
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForFinalization, err
		}

		// Send updated L1 signed state to couterparty
		messages, err := protocols.CreateObjectivePayloadMessage(updated.Id(), updatedL1SignedState, SignedStatePayload, updated.otherParticipants()...)
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForFinalization, fmt.Errorf("could not create payload message %w", err)
		}
		sideEffects.MessagesToSend = append(sideEffects.MessagesToSend, messages...)

		// Send MirrorWithdrawAll transaction
		mirrorWithdrawAllTx := protocols.NewMirrorWithdrawAllTransaction(updated.OwnsChannel(), updated.L2SignedState)
		updated.MirrorTransactionSubmitted = true
		sideEffects.TransactionsToSubmit = append(sideEffects.TransactionsToSubmit, mirrorWithdrawAllTx)
		return &updated, sideEffects, WaitingForFinalization, nil
	}

	if len(updated.L2SignedState.Signatures()) == 0 && !updated.C.LatestSignedStateSignedByMe() {
		// Sign received signed state and send it back
		latestSignedState, err := updated.C.LatestSignedState()
		if err != nil {
			return &updated, sideEffects, WaitingForNothing, errors.New("the channel must contain at least one signed state to crank the defund objective")
		}

		ss, err := updated.C.SignAndAddState(latestSignedState.State(), secretKey)
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForFinalization, fmt.Errorf("could not sign final state %w", err)
		}

		messages, err := protocols.CreateObjectivePayloadMessage(updated.Id(), ss, SignedStatePayload, o.otherParticipants()...)
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForFinalization, fmt.Errorf("could not create payload message %w", err)
		}
		sideEffects.MessagesToSend = append(sideEffects.MessagesToSend, messages...)
		return &updated, sideEffects, WaitingForWithdraw, nil
	}

	if !updated.FullyWithdrawn() {
		// Wait until the channel no longer holds any assets on the chain
		return &updated, sideEffects, WaitingForWithdraw, nil
	}

	return &updated, sideEffects, WaitingForNothing, nil
}

func (o *Objective) Approve() protocols.Objective {
	updated := o.clone()
	// todo: consider case of o.Status == Rejected
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

// Update receives an ObjectivePayload, applies all applicable data to the MirrorBridgedDeFundingObjectiveState,
// and returns the updated state
func (o *Objective) Update(p protocols.ObjectivePayload) (protocols.Objective, error) {
	if o.Id() != p.ObjectiveId {
		return o, fmt.Errorf("event and objective Ids do not match: %s and %s respectively", string(p.ObjectiveId), string(o.Id()))
	}

	updated := o.clone()
	ss, err := getSignedStatePayload(p.PayloadData)
	if err != nil {
		return o, fmt.Errorf("could not get signed state payload: %w", err)
	}
	updated.C.AddSignedState(ss)
	return &updated, nil
}

// clone returns a deep copy of the receiver.
func (o *Objective) clone() Objective {
	clone := Objective{}
	clone.Status = o.Status

	clone.C = o.C.Clone()
	clone.L2SignedState = o.L2SignedState
	clone.MirrorTransactionSubmitted = o.MirrorTransactionSubmitted

	return clone
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

// Create updated L1 state based on the variable part of the L2 state
func (o *Objective) CreateL1StateBasedOnL2() (state.State, error) {
	// Get the latest L1 supported state
	l1State, err := o.C.LatestSupportedState()
	if err != nil {
		return state.State{}, fmt.Errorf("could not retrieve latest signed state %w", err)
	}

	l1VariablePartBasedOnL2 := o.L2SignedState.State().VariablePart()

	// Swap the L2 outcome: since Alice creates a ledger channel in L1, the 0th position in L1's state allocations corresponds to Alice. Similarly, since Bridge Prime creates a ledger channel in L2, the 0th position in L2's state allocations corresponds to Bridge Prime.
	l1OutcomeBasedOnL2 := l1VariablePartBasedOnL2.Outcome.Clone()
	tempAllocation := l1OutcomeBasedOnL2[0].Allocations[0]
	l1OutcomeBasedOnL2[0].Allocations[0] = l1OutcomeBasedOnL2[0].Allocations[1]
	l1OutcomeBasedOnL2[0].Allocations[1] = tempAllocation
	l1VariablePartBasedOnL2.Outcome = l1OutcomeBasedOnL2

	return state.StateFromFixedAndVariablePart(l1State.FixedPart(), l1VariablePartBasedOnL2), nil
}

// FullyWithdrawn returns true if the channel contains no assets on chain
func (o *Objective) FullyWithdrawn() bool {
	return !o.C.OnChain.Holdings.IsNonZero()
}

// ConstructObjectiveFromPayload takes in a state and constructs an objective from it.
func ConstructObjectiveFromPayload(
	p protocols.ObjectivePayload,
	preapprove bool,
	getConsensusChannel GetConsensusChannel,
) (Objective, error) {
	ss, err := getSignedStatePayload(p.PayloadData)
	if err != nil {
		return Objective{}, fmt.Errorf("could not get signed state payload: %w", err)
	}
	s := ss.State()

	// Implicit in the wire protocol is that the message signalling
	// closure of a channel includes an isFinal state (in the 0 slot of the message)
	//
	if !s.IsFinal {
		return Objective{}, ErrNoFinalState
	}

	err = s.FixedPart().Validate()
	if err != nil {
		return Objective{}, err
	}

	cId := s.ChannelId()
	request := NewObjectiveRequest(cId, state.SignedState{})
	return NewObjective(request, preapprove, getConsensusChannel, false)
}

// IsMirrorBridgedDefundObjective inspects a objective id and returns true if the objective id is for a bridged defund objective.
func IsMirrorBridgedDefundObjective(id protocols.ObjectiveId) bool {
	return strings.HasPrefix(string(id), ObjectivePrefix)
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

// CreateChannelFromConsensusChannel creates a Channel with (an appropriate latest supported state) from the supplied ConsensusChannel.
func CreateChannelFromConsensusChannel(cc consensus_channel.ConsensusChannel) (*channel.Channel, error) {
	c, err := channel.New(cc.ConsensusVars().AsState(cc.SupportedSignedState().State().FixedPart()), uint(cc.MyIndex), channel.Ledger)
	if err != nil {
		return &channel.Channel{}, err
	}
	c.AddSignedState(cc.SupportedSignedState())
	c.OnChain.Holdings = cc.OnChainFunding

	return c, nil
}

// ObjectiveRequest represents a request to create a mirror bridged defund objective.
type ObjectiveRequest struct {
	l1ChannelId      types.Destination
	l2SignedState    state.SignedState
	objectiveStarted chan struct{}
}

// NewObjectiveRequest creates a new ObjectiveRequest.
func NewObjectiveRequest(l1ChannelId types.Destination, l2SignedState state.SignedState) ObjectiveRequest {
	return ObjectiveRequest{
		l1ChannelId:      l1ChannelId,
		l2SignedState:    l2SignedState,
		objectiveStarted: make(chan struct{}),
	}
}

func (r ObjectiveRequest) SignalObjectiveStarted() {
	close(r.objectiveStarted)
}

// WaitForObjectiveToStart blocks until the objective starts
func (r ObjectiveRequest) WaitForObjectiveToStart() {
	<-r.objectiveStarted
}

// Id returns the objective id for the request.
func (r ObjectiveRequest) Id(myAddress types.Address, chainId *big.Int) protocols.ObjectiveId {
	return protocols.ObjectiveId(ObjectivePrefix + r.l1ChannelId.String())
}
