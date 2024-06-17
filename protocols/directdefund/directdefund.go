// Package directdefund implements an off-chain protocol to defund a directly-funded channel.
package directdefund // import "github.com/statechannels/go-nitro/directfund"

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/consensus_channel"
	"github.com/statechannels/go-nitro/channel/state"
	NitroAdjudicator "github.com/statechannels/go-nitro/node/engine/chainservice/adjudicator"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

const (
	WaitingForFinalization     protocols.WaitingFor = "WaitingForFinalization"
	WaitingForWithdraw         protocols.WaitingFor = "WaitingForWithdraw"
	WaitingForChallenge        protocols.WaitingFor = "WaitingForChallenge"
	WaitingForChallengeCleared protocols.WaitingFor = "WaitingForChallengeCleared"
	WaitingForNothing          protocols.WaitingFor = "WaitingForNothing" // Finished
)

const (
	SignedStatePayload protocols.PayloadType = "SignedStatePayload"
)

var ErrChannelNotExist error = errors.New("could not find channel")

const ObjectivePrefix = "DirectDefunding-"

const (
	ErrChannelUpdateInProgress = types.ConstError("can only defund a channel when the latest state is supported or when the channel has a final state")
	ErrNoFinalState            = types.ConstError("cannot spawn direct defund objective without a final state")
	ErrNotEmpty                = types.ConstError("ledger channel has running guarantees")
)

// Objective is a cache of data computed by reading from the store. It stores (potentially) infinite data
type Objective struct {
	Status       protocols.ObjectiveStatus
	C            *channel.Channel
	finalTurnNum uint64

	// Whether a withdraw transaction has been declared as a side effect in a previous crank
	withdrawTransactionSubmitted bool

	IsChallenge                   bool
	challengeTransactionSubmitted bool

	IsCheckpoint                   bool
	checkpointTransactionSubmitted bool
}

// isInConsensusOrFinalState returns true if the channel has a final state or latest state that is supported
func isInConsensusOrFinalState(c *channel.Channel) (bool, error) {
	latestSS, err := c.LatestSignedState()
	// There are no signed states. We consider this as consensus
	if err != nil && err.Error() == "No states are signed" {
		return true, nil
	}
	if latestSS.State().IsFinal {
		return true, nil
	}

	latestSupportedState, err := c.LatestSupportedState()
	if err != nil {
		return false, err
	}

	return cmp.Equal(latestSS.State(), latestSupportedState), nil
}

// GetChannelByIdFunction specifies a function that can be used to retrieve channels from a store.
type GetChannelByIdFunction func(id types.Destination) (channel *channel.Channel, ok bool)

// GetConsensusChannel describes functions which return a ConsensusChannel ledger channel for a channel id.
type GetConsensusChannel func(channelId types.Destination) (ledger *consensus_channel.ConsensusChannel, err error)

// NewObjective initiates an Objective with the supplied channel
func NewObjective(
	request ObjectiveRequest,
	preApprove bool,
	getConsensusChannel GetConsensusChannel,
) (Objective, error) {
	cc, err := getConsensusChannel(request.ChannelId)
	if err != nil {
		return Objective{}, fmt.Errorf("%w %s: %w", ErrChannelNotExist, request.ChannelId, err)
	}

	if len(cc.FundingTargets()) != 0 {
		return Objective{}, ErrNotEmpty
	}

	c, err := CreateChannelFromConsensusChannel(*cc)
	if err != nil {
		return Objective{}, fmt.Errorf("could not create Channel from ConsensusChannel; %w", err)
	}

	// We choose to disallow creating an objective if the channel has an in-progress update.
	// We allow the creation of of an objective if the channel has some final states.
	// In the future, we can add a restriction that only defund objectives can add final states to the channel.
	canCreateObjective, err := isInConsensusOrFinalState(c)
	if err != nil {
		return Objective{}, err
	}
	if !canCreateObjective {
		return Objective{}, ErrChannelUpdateInProgress
	}

	init := Objective{}

	if preApprove {
		init.Status = protocols.Approved
	} else {
		init.Status = protocols.Unapproved
	}
	init.C = c.Clone()

	latestSS, err := c.LatestSupportedState()
	if err != nil {
		return init, err
	}

	if !latestSS.IsFinal {
		init.finalTurnNum = latestSS.TurnNum + 1
	} else {
		init.finalTurnNum = latestSS.TurnNum
	}

	init.IsChallenge = request.IsChallenge
	return init, nil
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
	request := NewObjectiveRequest(cId, false)
	return NewObjective(request, preapprove, getConsensusChannel)
}

// Public methods on the DirectDefundingObjective

// Id returns the unique id of the objective
func (o *Objective) Id() protocols.ObjectiveId {
	return protocols.ObjectiveId(ObjectivePrefix + o.C.Id.String())
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

// OwnsChannel returns the channel that the objective is funding.
func (o Objective) OwnsChannel() types.Destination {
	return o.C.Id
}

// GetStatus returns the status of the objective.
func (o Objective) GetStatus() protocols.ObjectiveStatus {
	return o.Status
}

func (o *Objective) Related() []protocols.Storable {
	return []protocols.Storable{o.C}
}

// Update receives an ObjectiveEvent, applies all applicable event data to the DirectDefundingObjective,
// and returns the updated objective
func (o *Objective) Update(p protocols.ObjectivePayload) (protocols.Objective, error) {
	if o.Id() != p.ObjectiveId {
		return o, fmt.Errorf("event and objective Ids do not match: %s and %s respectively", string(p.ObjectiveId), string(o.Id()))
	}
	ss, err := getSignedStatePayload(p.PayloadData)
	if err != nil {
		return o, fmt.Errorf("could not get signed state payload: %w", err)
	}
	if len(ss.Signatures()) != 0 {

		if !ss.State().IsFinal {
			return o, errors.New("direct defund objective can only be updated with final states")
		}
		if o.finalTurnNum != ss.State().TurnNum {
			return o, fmt.Errorf("expected state with turn number %d, received turn number %d", o.finalTurnNum, ss.State().TurnNum)
		}
	} else {
		return o, fmt.Errorf("event does not contain a signed state")
	}

	updated := o.clone()
	updated.C.AddSignedState(ss)

	return &updated, nil
}

// Crank inspects the extended state and declares a list of Effects to be executed
func (o *Objective) Crank(secretKey *[]byte) (protocols.Objective, protocols.SideEffects, protocols.WaitingFor, error) {
	updated := o.clone()

	sideEffects := protocols.SideEffects{}

	if updated.Status != protocols.Approved {
		return &updated, sideEffects, WaitingForNothing, protocols.ErrNotApproved
	}

	// Direct defund with challenge
	if updated.IsChallenge || updated.IsCheckpoint || updated.C.GetChannelMode() != channel.Open {
		return o.crankWithChallenge(updated, sideEffects, secretKey)
	}

	// Direct defund without challenge
	return o.crank(updated, sideEffects, secretKey)
}

func (o *Objective) crankWithChallenge(updated Objective, sideEffects protocols.SideEffects, secretKey *[]byte) (protocols.Objective, protocols.SideEffects, protocols.WaitingFor, error) {
	// Initiate challenge transaction
	if updated.IsChallenge && !updated.challengeTransactionSubmitted {
		latestSupportedSignedState, err := updated.C.LatestSupportedSignedState()
		if err != nil {
			return &updated, sideEffects, WaitingForNothing, err
		}

		challengerSig, _ := NitroAdjudicator.SignChallengeMessage(latestSupportedSignedState.State(), *secretKey)
		challengeTx := protocols.NewChallengeTransaction(updated.C.Id, latestSupportedSignedState, make([]state.SignedState, 0), challengerSig)
		sideEffects.TransactionsToSubmit = append(sideEffects.TransactionsToSubmit, challengeTx)
		updated.challengeTransactionSubmitted = true
		return &updated, sideEffects, WaitingForChallenge, nil
	}

	// Initiate checkpoint transaction
	if updated.IsCheckpoint && !updated.checkpointTransactionSubmitted {
		latestSupportedSignedState, err := updated.C.LatestSupportedSignedState()
		if err != nil {
			return &updated, sideEffects, WaitingForNothing, err
		}
		checkpointTx := protocols.NewCheckpointTransaction(updated.C.Id, latestSupportedSignedState, make([]state.SignedState, 0))
		sideEffects.TransactionsToSubmit = append(sideEffects.TransactionsToSubmit, checkpointTx)
		updated.checkpointTransactionSubmitted = true
		return &updated, sideEffects, WaitingForChallengeCleared, nil
	}

	// Wait for channel to finalize
	if updated.C.GetChannelMode() == channel.Challenge {
		return &updated, sideEffects, WaitingForFinalization, nil
	}

	// Liquidate the assets
	if updated.C.GetChannelMode() == channel.Finalized && !updated.withdrawTransactionSubmitted && !updated.FullyWithdrawn() {
		latestSupportedSignedState, _ := updated.C.LatestSupportedSignedState()
		transferTx := protocols.NewTransferAllTransaction(updated.C.Id, latestSupportedSignedState)
		sideEffects.TransactionsToSubmit = append(sideEffects.TransactionsToSubmit, transferTx)
		updated.withdrawTransactionSubmitted = true
		return &updated, sideEffects, WaitingForWithdraw, nil
	}

	// Direct defund with challenge objective is complete after asset liquidation
	if updated.C.GetChannelMode() == channel.Finalized && updated.FullyWithdrawn() {
		updated.Status = protocols.Completed
		return &updated, sideEffects, WaitingForNothing, nil
	}

	// Direct defund with challenge objective is complete after challenge is cleared
	if updated.C.GetChannelMode() == channel.Open {
		updated.Status = protocols.Completed
		return &updated, sideEffects, WaitingForNothing, nil
	}

	return &updated, sideEffects, WaitingForNothing, fmt.Errorf("objective %s in invalid state", string(updated.Id()))
}

func (o *Objective) crank(updated Objective, sideEffects protocols.SideEffects, secretKey *[]byte) (protocols.Objective, protocols.SideEffects, protocols.WaitingFor, error) {
	latestSignedState, err := updated.C.LatestSignedState()
	if err != nil {
		return &updated, sideEffects, WaitingForNothing, errors.New("the channel must contain at least one signed state to crank the defund objective")
	}

	// Sign a final state if no supported, final state exists
	if !latestSignedState.State().IsFinal || !latestSignedState.HasSignatureForParticipant(updated.C.MyIndex) {
		stateToSign := latestSignedState.State().Clone()
		if !stateToSign.IsFinal {
			stateToSign.TurnNum += 1
			stateToSign.IsFinal = true
		}
		ss, err := updated.C.SignAndAddState(stateToSign, secretKey)
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForFinalization, fmt.Errorf("could not sign final state %w", err)
		}
		messages, err := protocols.CreateObjectivePayloadMessage(updated.Id(), ss, SignedStatePayload, o.otherParticipants()...)
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForFinalization, fmt.Errorf("could not create payload message %w", err)
		}
		sideEffects.MessagesToSend = append(sideEffects.MessagesToSend, messages...)
	}

	latestSupportedState, err := updated.C.LatestSupportedState()
	if err != nil {
		return &updated, sideEffects, WaitingForFinalization, fmt.Errorf("error finding a supported state: %w", err)
	}
	if !latestSupportedState.IsFinal {
		return &updated, sideEffects, WaitingForFinalization, nil
	}

	// Withdrawal of funds
	if !updated.FullyWithdrawn() {
		// The first participant in the channel submits the withdrawAll transaction
		if updated.C.MyIndex == 0 && !updated.withdrawTransactionSubmitted {
			withdrawAll := protocols.NewWithdrawAllTransaction(updated.C.Id, latestSignedState)
			sideEffects.TransactionsToSubmit = append(sideEffects.TransactionsToSubmit, withdrawAll)
			updated.withdrawTransactionSubmitted = true
		}
		// Every participant waits for all channel funds to be distributed, even if the participant has no funds in the channel
		return &updated, sideEffects, WaitingForWithdraw, nil
	}

	updated.Status = protocols.Completed
	return &updated, sideEffects, WaitingForNothing, nil
}

// IsDirectDefundObjective inspects a objective id and returns true if the objective id is for a direct defund objective.
func IsDirectDefundObjective(id protocols.ObjectiveId) bool {
	return strings.HasPrefix(string(id), ObjectivePrefix)
}

//  Private methods on the DirectDefundingObjective

// CreateChannelFromConsensusChannel creates a Channel with (an appropriate latest supported state) from the supplied ConsensusChannel.
func CreateChannelFromConsensusChannel(cc consensus_channel.ConsensusChannel) (*channel.Channel, error) {
	c, err := channel.New(cc.ConsensusVars().AsState(cc.SupportedSignedState().State().FixedPart()), uint(cc.MyIndex))
	if err != nil {
		return &channel.Channel{}, err
	}
	c.AddSignedState(cc.SupportedSignedState())
	c.OnChain.Holdings = cc.OnChainFunding

	return c, nil
}

// FullyWithdrawn returns true if the channel contains no assets on chain
func (o *Objective) FullyWithdrawn() bool {
	return !o.C.OnChain.Holdings.IsNonZero()
}

// clone returns a deep copy of the receiver.
func (o *Objective) clone() Objective {
	clone := Objective{}
	clone.Status = o.Status

	cClone := o.C.Clone()
	clone.C = cClone
	clone.finalTurnNum = o.finalTurnNum
	clone.withdrawTransactionSubmitted = o.withdrawTransactionSubmitted

	clone.IsChallenge = o.IsChallenge
	clone.challengeTransactionSubmitted = o.challengeTransactionSubmitted

	clone.IsCheckpoint = o.IsCheckpoint
	clone.checkpointTransactionSubmitted = o.checkpointTransactionSubmitted

	return clone
}

// ObjectiveRequest represents a request to create a new direct defund objective.
type ObjectiveRequest struct {
	ChannelId        types.Destination
	objectiveStarted chan struct{}
	IsChallenge      bool
}

// NewObjectiveRequest creates a new ObjectiveRequest.
func NewObjectiveRequest(channelId types.Destination, isChallenge bool) ObjectiveRequest {
	return ObjectiveRequest{
		ChannelId:        channelId,
		objectiveStarted: make(chan struct{}),
		IsChallenge:      isChallenge,
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
	return protocols.ObjectiveId(ObjectivePrefix + r.ChannelId.String())
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

// otherParticipants returns the participants in the channel that are not the current participant.
func (o *Objective) otherParticipants() []types.Address {
	others := make([]types.Address, 0)
	for i, p := range o.C.Participants {
		if i != int(o.C.MyIndex) {
			others = append(others, p)
		}
	}
	return others
}

// CreateConsensusChannelFromChannel creates a ConsensusChannel from the Objective by extracting signatures and a single asset outcome from the latest supported signed state.
func (o *Objective) CreateConsensusChannelFromChannel() (*consensus_channel.ConsensusChannel, error) {
	ledger := o.C

	signedState, err := ledger.LatestSupportedSignedState()
	if err != nil {
		return nil, fmt.Errorf("could not get latest supported signed state")
	}
	leaderSig, err := signedState.GetParticipantSignature(uint(consensus_channel.Leader))
	if err != nil {
		return nil, fmt.Errorf("could not get leader signature: %w", err)
	}
	followerSig, err := signedState.GetParticipantSignature(uint(consensus_channel.Follower))
	if err != nil {
		return nil, fmt.Errorf("could not get follower signature: %w", err)
	}
	signatures := [2]state.Signature{leaderSig, followerSig}

	if len(signedState.State().Outcome) != 1 {
		return nil, fmt.Errorf("a consensus channel only supports a single asset")
	}
	assetExit := signedState.State().Outcome[0]
	turnNum := signedState.State().TurnNum
	outcome, err := consensus_channel.FromExit(assetExit)
	if err != nil {
		return nil, fmt.Errorf("could not create ledger outcome from channel exit: %w", err)
	}

	if ledger.MyIndex == uint(consensus_channel.Leader) {
		con, err := consensus_channel.NewLeaderChannel(ledger.FixedPart, turnNum, outcome, signatures)
		con.OnChainFunding = ledger.OnChain.Holdings.Clone() // Copy OnChain.Holdings so we don't lose this information
		if err != nil {
			return nil, fmt.Errorf("could not create consensus channel as leader: %w", err)
		}
		return &con, nil

	} else {
		con, err := consensus_channel.NewFollowerChannel(ledger.FixedPart, turnNum, outcome, signatures)
		con.OnChainFunding = ledger.OnChain.Holdings.Clone() // Copy OnChain.Holdings so we don't lose this information
		if err != nil {
			return nil, fmt.Errorf("could not create consensus channel as follower: %w", err)
		}
		return &con, nil
	}
}
