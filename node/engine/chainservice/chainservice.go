// Package chainservice is a chain service responsible for submitting blockchain transactions and relaying blockchain events.
package chainservice // import "github.com/statechannels/go-nitro/node/chainservice"

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

// Event dictates which methods all chain events must implement
type Event interface {
	ChannelID() types.Destination
	Block() Block
	TxIndex() uint
}

// commonEvent declares fields shared by all chain events
type commonEvent struct {
	channelID types.Destination
	block     Block
	txIndex   uint
}

func (ce commonEvent) ChannelID() types.Destination {
	return ce.channelID
}

func (ce commonEvent) Block() Block {
	return ce.block
}

func (ce commonEvent) TxIndex() uint {
	return ce.txIndex
}

type assetAndAmount struct {
	AssetAddress common.Address
	AssetAmount  *big.Int
}

func (aaa assetAndAmount) String() string {
	return aaa.AssetAmount.String() + " units of " + aaa.AssetAddress.Hex() + " token"
}

// DepositedEvent is an internal representation of the deposited blockchain event
type DepositedEvent struct {
	commonEvent
	Asset   types.Address
	NowHeld *big.Int
}

func (de DepositedEvent) String() string {
	return "Deposited " + de.Asset.String() + " leaving " + de.NowHeld.String() + " now held against channel " + de.channelID.String() + " at Block " + fmt.Sprint(de.block.BlockNum)
}

// AllocationUpdated is an internal representation of the AllocationUpdated blockchain event
// The event includes the token address and amount at the block that generated the event
type AllocationUpdatedEvent struct {
	commonEvent
	assetAndAmount
}

func (aue AllocationUpdatedEvent) String() string {
	return "Channel " + aue.channelID.String() + " has had allocation updated to " + aue.assetAndAmount.String() + " at Block " + fmt.Sprint(aue.block.BlockNum)
}

// ConcludedEvent is an internal representation of the Concluded blockchain event
type ConcludedEvent struct {
	commonEvent
}

func (ce ConcludedEvent) String() string {
	return "Channel " + ce.channelID.String() + " concluded at Block " + fmt.Sprint(ce.block.BlockNum)
}

type ChallengeRegisteredEvent struct {
	commonEvent
	candidate           state.VariablePart
	candidateSignatures []state.Signature
	FinalizesAt         *big.Int
	IsInitiatedByMe     bool
}

// NewChallengeRegisteredEvent constructs a ChallengeRegisteredEvent
func NewChallengeRegisteredEvent(
	channelId types.Destination,
	block Block,
	txIndex uint,
	variablePart state.VariablePart,
	sigs []state.Signature,
	finalizesAt *big.Int,
	isInitiatedByMe bool,
) ChallengeRegisteredEvent {
	return ChallengeRegisteredEvent{
		commonEvent: commonEvent{channelID: channelId, block: block, txIndex: txIndex},
		candidate: state.VariablePart{
			AppData: variablePart.AppData,
			Outcome: variablePart.Outcome,
			TurnNum: variablePart.TurnNum,
			IsFinal: variablePart.IsFinal,
		}, candidateSignatures: sigs,
		FinalizesAt:     finalizesAt,
		IsInitiatedByMe: isInitiatedByMe,
	}
}

type StatusUpdatedEvent struct {
	commonEvent
	StateHash types.Bytes32
}

func (sue StatusUpdatedEvent) String() string {
	return "Status updated event for Channel " + sue.channelID.String() + " concluded at Block " + fmt.Sprint(sue.block.BlockNum)
}

// StateHash returns the statehash stored on chain at the time of the ChallengeRegistered Event firing.
func (cr ChallengeRegisteredEvent) StateHash(fp state.FixedPart) (common.Hash, error) {
	return state.StateFromFixedAndVariablePart(fp, cr.candidate).Hash()
}

// Outcome returns the outcome which will have been stored on chain in the adjudicator after the ChallengeRegistered Event fires.
func (cr ChallengeRegisteredEvent) Outcome() outcome.Exit {
	return cr.candidate.Outcome
}

// SignedState returns the signed state which will have been stored on chain in the adjudicator after the ChallengeRegistered Event fires.
func (cr ChallengeRegisteredEvent) SignedState(fp state.FixedPart) (state.SignedState, error) {
	s := state.StateFromFixedAndVariablePart(fp, cr.candidate)
	ss := state.NewSignedState(s)
	for _, sig := range cr.candidateSignatures {
		err := ss.AddSignature(sig)
		if err != nil {
			return state.SignedState{}, err
		}
	}
	return ss, nil
}

func (cr ChallengeRegisteredEvent) String() string {
	return "Challenge registered for Channel " + cr.channelID.String() + " at Block " + fmt.Sprint(cr.block.BlockNum)
}

func NewDepositedEvent(channelId types.Destination, block Block, txIndex uint, assetAddress common.Address, nowHeld *big.Int) DepositedEvent {
	return DepositedEvent{commonEvent{channelId, block, txIndex}, assetAddress, nowHeld}
}

func NewAllocationUpdatedEvent(channelId types.Destination, block Block, txIndex uint, assetAddress common.Address, assetAmount *big.Int) AllocationUpdatedEvent {
	return AllocationUpdatedEvent{commonEvent{channelId, block, txIndex}, assetAndAmount{AssetAddress: assetAddress, AssetAmount: assetAmount}}
}

type ChallengeClearedEvent struct {
	commonEvent
	newTurnNumRecord *big.Int
}

func (cc ChallengeClearedEvent) String() string {
	return "Challenge cleared for Channel " + cc.channelID.String() + " at Block " + fmt.Sprint(cc.block.BlockNum)
}

func NewChallengeClearedEvent(channelId types.Destination, block Block, txIndex uint, newTurnNumRecord *big.Int) ChallengeClearedEvent {
	return ChallengeClearedEvent{commonEvent: commonEvent{channelID: channelId, block: block, txIndex: txIndex}, newTurnNumRecord: newTurnNumRecord}
}

type ReclaimedEvent struct {
	// TODO: Check other fields of reclaimed event to store
	commonEvent
}

func (re ReclaimedEvent) String() string {
	return "Reclaim event for Channel " + re.channelID.String() + " at Block " + fmt.Sprint(re.block.BlockNum)
}

// ChainEventHandler describes an objective that can handle chain events
type ChainEventHandler interface {
	UpdateWithChainEvent(event Event) (protocols.Objective, error)
}

type ChainService interface {
	// EventFeed returns a chan for receiving events from the chain service.
	EventFeed() <-chan Event
	// SendTransaction is for sending transactions with the chain service
	SendTransaction(protocols.ChainTransaction) error
	// GetConsensusAppAddress returns the address of a deployed ConsensusApp (for ledger channels)
	GetConsensusAppAddress() types.Address
	// GetVirtualPaymentAppAddress returns the address of a deployed VirtualPaymentApp
	GetVirtualPaymentAppAddress() types.Address
	// GetChainId returns the id of the chain the service is connected to
	GetChainId() (*big.Int, error)
	// GetLastConfirmedBlockNum returns the highest blockNum that satisfies the chainservice's REQUIRED_BLOCK_CONFIRMATIONS
	GetLastConfirmedBlockNum() uint64
	// GetBlockByNumber returns the block for given block number
	GetBlockByNumber(blockNum *big.Int) (*ethTypes.Block, error)
	// Close closes the ChainService
	Close() error
}
