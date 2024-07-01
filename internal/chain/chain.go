package chain

import (
	"context"
	"os/exec"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	chainutils "github.com/statechannels/go-nitro/node/engine/chainservice/utils"
)

func StartAnvil(chainPort string) (*exec.Cmd, error) {
	anvilChain, err := chainservice.StartAnvil(chainPort)
	if err != nil {
		return nil, err
	}
	return anvilChain.AnvilCmd, nil
}

// DeployContracts deploys the NitroAdjudicator, VirtualPaymentApp and ConsensusApp contracts.
func DeployContracts(ctx context.Context, chainUrl, chainAuthToken, chainPk string) (chainutils.ContractAddresses, error) {
	ethClient, txSubmitter, err := chainutils.ConnectToChain(context.Background(), chainUrl, chainAuthToken, common.Hex2Bytes(chainPk))
	if err != nil {
		return chainutils.ContractAddresses{}, err
	}
	return chainutils.DeployContracts(ctx, ethClient, txSubmitter)
}

// DeployL2Contract deploys bridge contract.
func DeployL2Contract(ctx context.Context, chainUrl, chainAuthToken, chainPk string) (common.Address, error) {
	ethClient, txSubmitter, err := chainutils.ConnectToChain(context.Background(), chainUrl, chainAuthToken, common.Hex2Bytes(chainPk))
	if err != nil {
		return common.Address{}, err
	}
	return chainutils.DeployL2Contract(ctx, ethClient, txSubmitter)
}
