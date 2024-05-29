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
	return anvilChain.AnvilCmd, err
}

// DeployContracts deploys the NitroAdjudicator, VirtualPaymentApp and ConsensusApp contracts.
func DeployContracts(ctx context.Context, chainUrl, chainAuthToken, chainPk string) (chainservice.ContractAddresses, error) {
	ethClient, txSubmitter, err := chainutils.ConnectToChain(context.Background(), chainUrl, chainAuthToken, common.Hex2Bytes(chainPk))
	if err != nil {
		return chainservice.ContractAddresses{}, err
	}
	return chainservice.DeployContracts(ctx, ethClient, txSubmitter)
}
