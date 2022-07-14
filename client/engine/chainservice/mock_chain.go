package chainservice

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/client/engine/store/safesync"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

// MockChain is an in-memory "chain" that accepts transactions and broadcasts events
type MockChain interface {
	SubmitTransaction(protocols.ChainTransaction) error // unlike an ethereum blockchain, Mockhain accepts go-nitro protocols.ChainTransaction
	SubscribeToEvents(a types.Address) <-chan Event     // returns a channel that produces all chain Events
}

// MockChainImpl mimicks the Ethereum blockchain by keeping track of block numbers and account balances.
type MockChainImpl struct {
	blockNum uint64
	// holdings tracks funds for each channel.
	holdings map[types.Destination]types.Funds
	// out maps addresses to an Event channel. Given that MockChainServices only subscribe
	// (and never unsubscribe) to events, this can be converted to a list.
	out safesync.Map[chan Event]
}

// NewMockChainImpl creates a new MockChainImpl
func NewMockChainImpl() *MockChainImpl {
	chain := MockChainImpl{}
	chain.blockNum = 1
	chain.holdings = make(map[types.Destination]types.Funds)
	chain.out = safesync.Map[chan Event]{}
	return &chain
}

// SubmitTransaction updates internal state and brodcasts events
func (mc *MockChainImpl) SubmitTransaction(tx protocols.ChainTransaction) error {
	mc.blockNum++
	switch tx := tx.(type) {
	case protocols.DepositTransaction:
		if tx.Deposit.IsNonZero() {
			mc.holdings[tx.ChannelId()] = mc.holdings[tx.ChannelId()].Add(tx.Deposit)
		}
		for address, amount := range tx.Deposit {
			event := NewDepositedEvent(tx.ChannelId(), mc.blockNum, address, amount, mc.holdings[tx.ChannelId()][address])
			mc.broadcastEvent(event)
		}
	case protocols.WithdrawAllTransaction:
		for assetAddress := range mc.holdings[tx.ChannelId()] {
			event := NewAllocationUpdatedEvent(tx.ChannelId(), mc.blockNum, assetAddress, common.Big0)
			mc.broadcastEvent(event)
		}
		mc.holdings[tx.ChannelId()] = types.Funds{}
	default:
		return fmt.Errorf("unexpected transaction type %T", tx)
	}
	return nil
}

func (mc *MockChainImpl) broadcastEvent(event Event) {
	mc.out.Range(func(_ string, channel chan Event) bool {
		channel <- event
		return true
	})
}

// SubscribeToEvents creates, stores, and returns a new Event channel
func (mc *MockChainImpl) SubscribeToEvents(a types.Address) <-chan Event {
	// Use a buffered channel so we don't have to worry about blocking on writing to the channel.
	c := make(chan Event, 10)
	mc.out.Store(a.String(), c)
	return c
}