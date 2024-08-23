package chainservice

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

// MockChainService adheres to the ChainService interface. The constructor accepts a MockChain, which allows multiple clients to share the same, in-memory chain.
type MockChainService struct {
	chain     *MockChain
	eventFeed <-chan Event
}

// NewMockChainService returns a new MockChainService.
func NewMockChainService(chain *MockChain, address common.Address) *MockChainService {
	mc := MockChainService{chain: chain}
	mc.eventFeed = chain.SubscribeToEvents(address)
	return &mc
}

// SendTransaction responds to the given tx.
func (mc *MockChainService) SendTransaction(tx protocols.ChainTransaction) error {
	return mc.chain.SubmitTransaction(tx)
}

// GetConsensusAppAddress returns the zero address, since the mock chain will not run any application logic.
func (mc *MockChainService) GetConsensusAppAddress() types.Address {
	return types.Address{}
}

// GetVirtualPaymentAppAddress returns the zero address, since the mock chain will not run any application logic.
func (mc *MockChainService) GetVirtualPaymentAppAddress() types.Address {
	return types.Address{}
}

func (mc *MockChainService) GetL1ChannelFromL2(l2Channel types.Destination) (types.Destination, error) {
	return types.Destination{}, nil
}

func (mc *MockChainService) GetL1AssetAddressFromL2(l2AssetAddress common.Address) (common.Address, error) {
	return common.Address{}, nil
}

func (mc *MockChainService) EventFeed() <-chan Event {
	return mc.eventFeed
}

func (mc *MockChainService) DroppedTxFeed() <-chan protocols.DroppedTxInfo {
	return make(<-chan protocols.DroppedTxInfo)
}

func (mc *MockChainService) GetChainId() (*big.Int, error) {
	return big.NewInt(TEST_CHAIN_ID), nil
}

func (mc *MockChainService) GetLastConfirmedBlockNum() uint64 {
	mc.chain.blockNumMu.Lock()
	blockNum := mc.chain.BlockNum
	mc.chain.blockNumMu.Unlock()

	return blockNum
}

func (mc *MockChainService) GetBlockByNumber(blockNum *big.Int) (*ethTypes.Block, error) {
	return &ethTypes.Block{}, nil
}

func (mc *MockChainService) Close() error {
	return nil
}
