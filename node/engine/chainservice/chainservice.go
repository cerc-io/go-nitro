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
	TxHash() common.Hash
}

// commonEvent declares fields shared by all chain events
type commonEvent struct {
	channelID types.Destination
	block     Block
	txIndex   uint
	txHash    common.Hash
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

func (ce commonEvent) TxHash() common.Hash {
	return ce.txHash
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
	txhash common.Hash,
) ChallengeRegisteredEvent {
	return ChallengeRegisteredEvent{
		commonEvent: commonEvent{channelID: channelId, block: block, txIndex: txIndex, txHash: txhash},
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

func NewDepositedEvent(channelId types.Destination, block Block, txIndex uint, assetAddress common.Address, nowHeld *big.Int, txhash common.Hash) DepositedEvent {
	return DepositedEvent{commonEvent{channelId, block, txIndex, txhash}, assetAddress, nowHeld}
}

func NewAllocationUpdatedEvent(channelId types.Destination, block Block, txIndex uint, assetAddress common.Address, assetAmount *big.Int, txhash common.Hash) AllocationUpdatedEvent {
	return AllocationUpdatedEvent{commonEvent{channelId, block, txIndex, txhash}, assetAndAmount{AssetAddress: assetAddress, AssetAmount: assetAmount}}
}

type ChallengeClearedEvent struct {
	commonEvent
	newTurnNumRecord *big.Int
}

func (cc ChallengeClearedEvent) String() string {
	return "Challenge cleared for Channel " + cc.channelID.String() + " at Block " + fmt.Sprint(cc.block.BlockNum)
}

func NewChallengeClearedEvent(channelId types.Destination, block Block, txIndex uint, newTurnNumRecord *big.Int, txhash common.Hash) ChallengeClearedEvent {
	return ChallengeClearedEvent{commonEvent: commonEvent{channelID: channelId, block: block, txIndex: txIndex, txHash: txhash}, newTurnNumRecord: newTurnNumRecord}
}

type ReclaimedEvent struct {
	// TODO: Check other fields of reclaimed event to store
	commonEvent
}

func (re ReclaimedEvent) String() string {
	return "Reclaim event for Channel " + re.channelID.String() + " at Block " + fmt.Sprint(re.block.BlockNum)
}

type AssetMapUpdatedEvent struct {
	commonEvent
	L1AssetAddress, L2AssetAddress common.Address
}

func (amue AssetMapUpdatedEvent) String() string {
	return "Asset map updated event at Block " + fmt.Sprint(amue.block.BlockNum)
}

type L2ToL1MapUpdated struct {
	commonEvent
	l1ChannelId, l2ChannelId types.Destination
}

func (l2l1mue L2ToL1MapUpdated) String() string {
	return "L2ToL1 map updated event at Block " + fmt.Sprint(l2l1mue.block.BlockNum)
}

// ChainEventHandler describes an objective that can handle chain events
type ChainEventHandler interface {
	UpdateWithChainEvent(event Event) (protocols.Objective, error)
}

type ChainService interface {
	// EventEngineFeed returns a chan for receiving events from the chain service
	EventEngineFeed() <-chan Event
	// EventFeed returns a chan for receiving bridge events from the chain service
	EventFeed() <-chan Event
	// Dropped event engine feed returns a chan for catching dropped events from chain service used by engine
	DroppedEventEngineFeed() <-chan protocols.DroppedEventInfo
	// TODO: Add comment
	DroppedEventFeed() <-chan protocols.DroppedEventInfo
	// SendTransaction is for sending transactions with the chain service
	SendTransaction(protocols.ChainTransaction) (*ethTypes.Transaction, error)
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
	// TODO: Implement method for eth calls
	// GetL1ChannelFromL2 returns the L1 ledger channel ID from the L2 ledger channel by making a contract call to the l2ToL1 map of the Nitro Adjudicator contract
	GetL1ChannelFromL2(l2Channel types.Destination) (types.Destination, error)
	// Close closes the ChainService
	Close() error
}
