package chainservice

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

type LaconicdChainOpts struct {
	VpaAddress common.Address
	CaAddress  common.Address
}
type LaconicdChainService struct {
	consensusAppAddress      common.Address
	virtualPaymentAppAddress common.Address
}

func NewLaconicdChainService(chainOpts LaconicdChainOpts) (*LaconicdChainService, error) {
	return &LaconicdChainService{
		chainOpts.CaAddress,
		chainOpts.VpaAddress,
	}, nil
}

func (lcs *LaconicdChainService) SendTransaction(tx protocols.ChainTransaction) (*ethTypes.Transaction, error) {
	return nil, nil
}

func (lcs *LaconicdChainService) DroppedEventEngineFeed() <-chan protocols.DroppedEventInfo {
	return nil
}

func (lcs *LaconicdChainService) DroppedEventFeed() <-chan protocols.DroppedEventInfo {
	return nil
}

func (lcs *LaconicdChainService) EventEngineFeed() <-chan Event {
	return nil
}

func (lcs *LaconicdChainService) EventFeed() <-chan Event {
	return nil
}

func (lcs *LaconicdChainService) GetConsensusAppAddress() common.Address {
	return lcs.consensusAppAddress
}

func (lcs *LaconicdChainService) GetVirtualPaymentAppAddress() common.Address {
	return lcs.virtualPaymentAppAddress
}

func (lcs *LaconicdChainService) GetChainId() (*big.Int, error) {
	return nil, nil
}

func (lcs *LaconicdChainService) GetLastConfirmedBlockNum() uint64 {
	return 0
}

func (lcs *LaconicdChainService) GetBlockByNumber(blockNum *big.Int) (*ethTypes.Block, error) {
	return &ethTypes.Block{}, nil
}

func (lcs *LaconicdChainService) GetL1ChannelFromL2(l2Channel types.Destination) (types.Destination, error) {
	return types.Destination{}, nil
}

func (lcs *LaconicdChainService) Close() error {
	return nil
}
