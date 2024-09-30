package swap

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/internal/queue"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

type GetChannelByIdFunction func(id types.Destination) (channel *channel.Channel, ok bool)

const (
	SwapPayloadType protocols.PayloadType = "SwapPayload"
)

const (
	WaitingForConsensus    protocols.WaitingFor = "WaitingForConsensus"
	WaitingForConfirmation protocols.WaitingFor = "WaitingForConfirmation"
	WaitingForNothing      protocols.WaitingFor = "WaitingForNothing" // Finished
)

const ObjectivePrefix = "Swap-"

type SwapPayload struct {
	Swap      channel.Swap
	StateSigs map[uint]state.Signature
}

// Objective is a cache of data computed by reading from the store. It stores (potentially) infinite data.
type Objective struct {
	Status     protocols.ObjectiveStatus
	C          *channel.SwapChannel
	Swap       channel.Swap
	StateSigs  map[uint]state.Signature
	SwapStatus types.SwapStatus
	// Index of participant who initiated the swap in participants array
	SwapperIndex uint
}

// NewObjective creates a new swap objective from a given request.
func NewObjective(request ObjectiveRequest, preApprove bool, isSwapper bool, getChannelFunc GetChannelByIdFunction, address common.Address) (Objective, error) {
	obj := Objective{}

	swapChannel, ok := getChannelFunc(request.swap.ChannelId)
	if !ok {
		return obj, fmt.Errorf("swap objective creation failed, swap channel not found")
	}

	obj.C = &channel.SwapChannel{
		Channel: *swapChannel,
		Swaps:   *queue.NewFixedQueue[channel.Swap](channel.MAX_SWAP_STORAGE_LIMIT),
	}

	if preApprove {
		obj.Status = protocols.Approved
	} else {
		obj.Status = protocols.Unapproved
	}

	myIndex, err := obj.FindParticipantIndex(address)
	if err != nil {
		return obj, err
	}

	obj.Swap = request.swap

	if isSwapper {
		obj.SwapperIndex = uint(myIndex)
	} else {
		obj.SwapperIndex = 1 - uint(myIndex)
	}

	obj.StateSigs = make(map[uint]state.Signature, 2)

	return obj, nil
}

// Id returns the objective id.
func (o *Objective) Id() protocols.ObjectiveId {
	return protocols.ObjectiveId(ObjectivePrefix + o.Swap.Id.String())
}

// Approve returns an approved copy of the objective.
func (o *Objective) Approve() protocols.Objective {
	updated := o.clone()
	// todo: consider case of s.Status == Rejected
	updated.Status = protocols.Approved

	return &updated
}

// Reject returns a rejected copy of the objective.
func (o *Objective) Reject() (protocols.Objective, protocols.SideEffects) {
	updated := o.clone()
	updated.Status = protocols.Rejected

	peer := o.C.Participants[1-o.C.MyIndex]
	messages := protocols.CreateRejectionNoticeMessage(o.Id(), peer)
	sideEffects := protocols.SideEffects{MessagesToSend: messages}
	return &updated, sideEffects
}

// OwnsChannel returns the channel that the objective is funding.
func (o *Objective) OwnsChannel() types.Destination {
	// Swap objective doesnt owns any channel
	return types.Destination{}
}

// GetStatus returns the status of the objective.
func (o *Objective) GetStatus() protocols.ObjectiveStatus {
	return o.Status
}

// Update receives an protocols.ObjectiveEvent, applies all applicable event data to the VirtualFundObjective,
// and returns the updated state.
func (o *Objective) Update(raw protocols.ObjectivePayload) (protocols.Objective, error) {
	updated := o.clone()
	swapPayload, err := getSwapPayload(raw.PayloadData)
	if err != nil {
		return &updated, fmt.Errorf("could not get swap payload: %w", err)
	}

	// Ensure the incoming swap is valid
	ok := o.Swap.Equal(swapPayload.Swap)
	if !ok {
		return &updated, fmt.Errorf("swap does not match")
	}

	counterPartySig := swapPayload.Swap.Sigs[1-updated.C.MyIndex]
	counterPartyAddress, err := o.Swap.RecoverSigner(counterPartySig)
	if err != nil {
		return &updated, err
	}

	if counterPartyAddress != o.C.Participants[1-o.C.MyIndex] {
		return &updated, fmt.Errorf("swap lacks counterparty's signature")
	}

	updated.Swap = swapPayload.Swap
	// TODO: Validation for state sigs
	updated.StateSigs = swapPayload.StateSigs

	return &updated, nil
}

// Crank inspects the extended state and declares a list of Effects to be executed
// It's like a state machine transition function where the finite / enumerable state is returned (computed from the extended state)
// rather than being independent of the extended state; and where there is only one type of event ("the crank") with no data on it at all.
func (o *Objective) Crank(secretKey *[]byte) (protocols.Objective, protocols.SideEffects, protocols.WaitingFor, error) {
	updated := o.clone()

	sideEffects := protocols.SideEffects{}
	// Input validation
	if updated.Status != protocols.Approved {
		return &updated, sideEffects, WaitingForNothing, protocols.ErrNotApproved
	}
	// TODO: Both participant checks whether swap operation is valid

	// TODO: Swapee check whether to accept or reject
	if updated.SwapperIndex != o.C.MyIndex && updated.SwapStatus != types.Accepted {
		if updated.SwapStatus == types.PendingConfirmation {
			return &updated, sideEffects, WaitingForConfirmation, nil
		} else {
			// Rejected
			updated.Status = protocols.Completed
			fmt.Println("Swap rejected")
			return &updated, sideEffects, WaitingForNothing, nil
		}
	}

	// Verify if I have signed it
	// If not, sign it and send it to the counterparty
	if !updated.HasSignatureForParticipant() {
		sig, err := updated.Swap.Sign(*secretKey)
		if err != nil {
			return &updated, sideEffects, WaitingForConsensus, err
		}

		err = updated.Swap.AddSignature(sig, updated.C.MyIndex)
		if err != nil {
			return &updated, sideEffects, WaitingForConsensus, err
		}

		updatedState, err := updated.GetUpdatedSwapState()
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForConsensus, fmt.Errorf("error creating updated swap channel state %w", err)
		}

		stateSig, err := updatedState.Sign(*secretKey)
		if err != nil {
			return &updated, sideEffects, WaitingForConsensus, fmt.Errorf("error signing swap channel state %w", err)
		}

		updated.StateSigs[updated.C.MyIndex] = stateSig

		messages, err := protocols.CreateObjectivePayloadMessage(
			updated.Id(),
			SwapPayload{
				Swap:      updated.Swap,
				StateSigs: updated.StateSigs,
			},
			SwapPayloadType,
			o.C.Participants[1-o.C.MyIndex],
		)
		if err != nil {
			return &updated, protocols.SideEffects{}, WaitingForConsensus, fmt.Errorf("could not create payload message %w", err)
		}

		sideEffects.MessagesToSend = append(sideEffects.MessagesToSend, messages...)
	}

	// Wait if all signatures are not available
	if !updated.HasAllSignatures() {
		return &updated, sideEffects, WaitingForConsensus, nil
	}

	// If all signatures are available, update the swap channel according to the swap
	err := updated.UpdateSwapChannelState()
	if err != nil {
		return &updated, protocols.SideEffects{}, WaitingForConsensus, fmt.Errorf("error updating swap channel state %w", err)
	}

	// Add swap to swap channel
	updated.C.Swaps.Enqueue(updated.Swap)

	// Completion
	updated.Status = protocols.Completed
	return &updated, sideEffects, WaitingForNothing, nil
}

func (o *Objective) UpdateSwapChannelState() error {
	updatedState, err := o.GetUpdatedSwapState()
	if err != nil {
		return fmt.Errorf("error creating updated swap channel state %w", err)
	}

	updatedSignedState := state.NewSignedState(updatedState)
	for _, sig := range o.StateSigs {
		err := updatedSignedState.AddSignature(sig)
		if err != nil {
			return fmt.Errorf("error adding signature to signed swap channel state %w", err)
		}
	}

	ok := o.C.AddSignedState(updatedSignedState)
	if !ok {
		return fmt.Errorf("error adding signed state to swap channel %w", err)
	}

	return nil
}

func (o *Objective) GetUpdatedSwapState() (state.State, error) {
	tokenIn := o.Swap.Exchange.TokenIn
	tokenOut := o.Swap.Exchange.TokenOut

	s, err := o.C.LatestSupportedState()
	if err != nil {
		return state.State{}, fmt.Errorf("latest supported state not found: %w", err)
	}
	updateSupportedState := s.Clone()
	updateOutcome := updateSupportedState.Outcome.Clone()

	for _, assetOutcome := range updateOutcome {

		swapperAllocation := assetOutcome.Allocations[o.SwapperIndex]
		swappeAllocation := assetOutcome.Allocations[1-o.SwapperIndex]

		if assetOutcome.Asset == tokenIn {

			swapperAllocation.Amount.Sub(swapperAllocation.Amount, o.Swap.Exchange.AmountIn)
			swappeAllocation.Amount.Add(swappeAllocation.Amount, o.Swap.Exchange.AmountIn)
		}

		if assetOutcome.Asset == tokenOut {
			swapperAllocation.Amount.Add(swapperAllocation.Amount, o.Swap.Exchange.AmountOut)
			swappeAllocation.Amount.Sub(swappeAllocation.Amount, o.Swap.Exchange.AmountOut)
		}
	}

	updateSupportedState.Outcome = updateOutcome
	updateSupportedState.TurnNum++
	return updateSupportedState, nil
}

func (o *Objective) Related() []protocols.Storable {
	ret := []protocols.Storable{o.C}

	return ret
}

// Clone returns a deep copy of the receiver.
func (o *Objective) clone() Objective {
	clone := Objective{}
	clone.Status = o.Status
	clone.SwapperIndex = o.SwapperIndex
	clone.SwapStatus = o.SwapStatus
	clone.C = o.C.Clone()
	clone.Swap = o.Swap.Clone()

	clonedSigs := make(map[uint]state.Signature, len(o.StateSigs))
	for i, sig := range o.StateSigs {
		clonedSigs[i] = sig
	}
	clone.StateSigs = clonedSigs

	return clone
}

// HasAllSignatures returns true if every participant has a valid signature.
func (o *Objective) HasAllSignatures() bool {
	// Since signatures are validated
	if len(o.Swap.Sigs) == len(o.C.Participants) && len(o.StateSigs) == len(o.C.Participants) {
		return true
	} else {
		return false
	}
}

// HasSignatureForParticipant returns true if the participant (at participantIndex) has a valid signature.
func (o *Objective) HasSignatureForParticipant() bool {
	_, found := o.Swap.Sigs[o.C.MyIndex]
	return found
}

func (o *Objective) FindParticipantIndex(address common.Address) (int, error) {
	for index, participantAddress := range o.C.Participants {
		if participantAddress == address {
			return index, nil
		}
	}

	return -1, fmt.Errorf("participant not found")
}

func (o *Objective) AcceptSwap() {
	o.SwapStatus = types.Accepted
}

type jsonObjective struct {
	Status                      protocols.ObjectiveStatus
	C                           types.Destination
	Swap                        channel.Swap
	SwapperIndex                uint
	SwapStatus                  types.SwapStatus
	Nonce                       uint64
	StateSigs                   map[uint]state.Signature
	ProcessedSwapsInSwapChannel []types.Destination
}

func (o Objective) MarshalJSON() ([]byte, error) {
	processedSwapsInSwapChannel := o.C.Swaps.Values()
	swaps := make([]types.Destination, 0)
	for _, s := range processedSwapsInSwapChannel {
		swaps = append(swaps, s.Id)
	}

	jsonSO := jsonObjective{
		Status:                      o.Status,
		C:                           o.C.Id,
		Swap:                        o.Swap,
		SwapStatus:                  o.SwapStatus,
		SwapperIndex:                o.SwapperIndex,
		StateSigs:                   o.StateSigs,
		ProcessedSwapsInSwapChannel: swaps,
	}
	return json.Marshal(jsonSO)
}

func (o *Objective) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var jsonSo jsonObjective
	if err := json.Unmarshal(data, &jsonSo); err != nil {
		return fmt.Errorf("failed to unmarshal the Swap Objective: %w", err)
	}

	o.Status = jsonSo.Status
	o.SwapperIndex = jsonSo.SwapperIndex
	o.SwapStatus = jsonSo.SwapStatus
	o.Swap = jsonSo.Swap
	o.StateSigs = jsonSo.StateSigs

	o.C = &channel.SwapChannel{}
	o.C.Id = jsonSo.C
	swaps := queue.NewFixedQueue[channel.Swap](channel.MAX_SWAP_STORAGE_LIMIT)
	for _, sId := range jsonSo.ProcessedSwapsInSwapChannel {
		swap := channel.Swap{
			Id: sId,
		}

		swaps.Enqueue(swap)
	}
	o.C.Swaps = *swaps
	return nil
}

// ConstructObjectiveFromPayload takes in a message and constructs an objective from it.
// It accepts the message, myAddress, and a function to to retrieve ledgers from a store.
func ConstructObjectiveFromPayload(
	op protocols.ObjectivePayload,
	preApprove bool,
	getChannelFunc GetChannelByIdFunction,
	address common.Address,
) (Objective, error) {
	swapPayload, err := getSwapPayload(op.PayloadData)
	if err != nil {
		return Objective{}, fmt.Errorf("could not get swap payload: %w", err)
	}

	objectiveReq := ObjectiveRequest{
		swap: swapPayload.Swap,
	}

	obj, err := NewObjective(objectiveReq, preApprove, false, getChannelFunc, address)
	if err != nil {
		return Objective{}, fmt.Errorf("unable to construct swap objective from payload: %w", err)
	}

	return obj, nil
}

func getSwapPayload(b []byte) (SwapPayload, error) {
	payload := SwapPayload{}
	err := json.Unmarshal(b, &payload)
	if err != nil {
		return payload, fmt.Errorf("could not unmarshal swap: %w", err)
	}
	return payload, nil
}

// IsSwapObjective inspects a objective id and returns true if the objective id is for a swap objective.
func IsSwapObjective(id protocols.ObjectiveId) bool {
	return strings.HasPrefix(string(id), ObjectivePrefix)
}

// ObjectiveRequest represents a request to create a new swap objective.
type ObjectiveRequest struct {
	swap             channel.Swap
	objectiveStarted chan struct{}
}

// NewObjectiveRequest creates a new ObjectiveRequest.
func NewObjectiveRequest(channelId types.Destination, tokenIn common.Address, tokenOut common.Address, amountIn *big.Int, amountOut *big.Int, fixedPart state.FixedPart, nonce uint64) ObjectiveRequest {
	swap := channel.NewSwap(channelId, tokenIn, tokenOut, amountIn, amountOut, nonce)
	return ObjectiveRequest{
		swap:             swap,
		objectiveStarted: make(chan struct{}),
	}
}

// Id returns the objective id for the request.
func (r ObjectiveRequest) Id(myAddress types.Address, chainId *big.Int) protocols.ObjectiveId {
	return protocols.ObjectiveId(ObjectivePrefix + r.swap.SwapId().String())
}

// SignalObjectiveStarted is used by the engine to signal the objective has been started.
func (r ObjectiveRequest) SignalObjectiveStarted() {
	close(r.objectiveStarted)
}

// WaitForObjectiveToStart blocks until the objective starts
func (r ObjectiveRequest) WaitForObjectiveToStart() {
	<-r.objectiveStarted
}

// ObjectiveResponse is the type returned across the API in response to the ObjectiveRequest.
type ObjectiveResponse struct {
	Id        protocols.ObjectiveId
	ChannelId types.Destination
}

// Response computes and returns the appropriate response from the request.
func (r ObjectiveRequest) Response() ObjectiveResponse {
	return ObjectiveResponse{
		Id:        protocols.ObjectiveId(ObjectivePrefix + r.swap.Id.String()),
		ChannelId: r.swap.ChannelId,
	}
}
