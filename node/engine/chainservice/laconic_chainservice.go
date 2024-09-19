package chainservice

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

type LaconicChainService struct{}

func NewLaconicChainService() (*LaconicChainService, error) {
	return &LaconicChainService{}, nil
}

func (lcs *LaconicChainService) SendTransaction(tx protocols.ChainTransaction) (*ethTypes.Transaction, error) {
	return nil, nil
}

func (lcs *LaconicChainService) DroppedEventEngineFeed() <-chan protocols.DroppedEventInfo {
	return nil
}

func (lcs *LaconicChainService) DroppedEventFeed() <-chan protocols.DroppedEventInfo {
	return nil
}

func (lcs *LaconicChainService) EventEngineFeed() <-chan Event {
	return nil
}

func (lcs *LaconicChainService) EventFeed() <-chan Event {
	return nil
}

func (lcs *LaconicChainService) GetConsensusAppAddress() common.Address {
	return common.Address{}
}

func (lcs *LaconicChainService) GetVirtualPaymentAppAddress() common.Address {
	return common.Address{}
}

func (lcs *LaconicChainService) GetChainId() (*big.Int, error) {
	return nil, nil
}

func (lcs *LaconicChainService) GetLastConfirmedBlockNum() uint64 {
	return 0
}

func (lcs *LaconicChainService) GetBlockByNumber(blockNum *big.Int) (*ethTypes.Block, error) {
	return &ethTypes.Block{}, nil
}

// TODO: Check is this method really required
func (lcs *LaconicChainService) GetL1ChannelFromL2(l2Channel types.Destination) (types.Destination, error) {
	return types.Destination{}, nil
}

func (lcs *LaconicChainService) GetL1AssetAddressFromL2(l2AssetAddress common.Address) (common.Address, error) {
	return common.Address{}, nil
}

func (lcs *LaconicChainService) Close() error {
	return nil
}
