package chain

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	b "github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	NitroAdjudicator "github.com/statechannels/go-nitro/node/engine/chainservice/adjudicator"
	ConsensusApp "github.com/statechannels/go-nitro/node/engine/chainservice/consensusapp"
	chainutils "github.com/statechannels/go-nitro/node/engine/chainservice/utils"
	VirtualPaymentApp "github.com/statechannels/go-nitro/node/engine/chainservice/virtualpaymentapp"
	"github.com/statechannels/go-nitro/types"
)

type ChainOpts struct {
	ChainUrl       string
	ChainAuthToken string
	ChainPk        string
	NaAddress      common.Address
	VpaAddress     common.Address
	CaAddress      common.Address
}

func InitializeEthChainService(chainOpts ChainOpts) (*chainservice.EthChainService, error) {
	if chainOpts.ChainPk == "" {
		return nil, fmt.Errorf("chainpk must be set")
	}

	fmt.Println("Initializing chain service and connecting to " + chainOpts.ChainUrl + "...")
	return chainservice.NewEthChainService(
		chainOpts.ChainUrl,
		chainOpts.ChainAuthToken,
		chainOpts.ChainPk,
		chainOpts.NaAddress,
		chainOpts.CaAddress,
		chainOpts.VpaAddress,
		os.Stdout)
}

func StartAnvil() (*exec.Cmd, error) {
	chainCmd := exec.Command("anvil", "--chain-id", "1337")
	chainCmd.Stdout = os.Stdout
	chainCmd.Stderr = os.Stderr
	err := chainCmd.Start()
	if err != nil {
		return &exec.Cmd{}, nil
	}
	// If Anvil start successfully, delay by 1 second for the chain to initialize
	time.Sleep(1 * time.Second)
	return chainCmd, nil
}

// DeployContracts deploys the NitroAdjudicator, VirtualPaymentApp and ConsensusApp contracts.
func DeployContracts(ctx context.Context, chainUrl, chainAuthToken, chainPk string) (na common.Address, vpa common.Address, ca common.Address, err error) {
	ethClient, txSubmitter, err := chainutils.ConnectToChain(context.Background(), chainUrl, chainAuthToken, common.Hex2Bytes(chainPk))
	if err != nil {
		return types.Address{}, types.Address{}, types.Address{}, err
	}
	var tx *ethTypes.Transaction

	na, tx, _, err = NitroAdjudicator.DeployNitroAdjudicator(txSubmitter, ethClient)
	if err != nil {
		return types.Address{}, types.Address{}, types.Address{}, err
	}

	fmt.Println("Waiting for NitroAdjudicator deployment confirmation")
	_, err = b.WaitMined(ctx, ethClient, tx)
	if err != nil {
		return types.Address{}, types.Address{}, types.Address{}, err
	}

	fmt.Printf("Deployed NitroAdjudicator at %s\n", na.String())

	vpa, tx, _, err = VirtualPaymentApp.DeployVirtualPaymentApp(txSubmitter, ethClient)
	if err != nil {
		return types.Address{}, types.Address{}, types.Address{}, err
	}

	fmt.Println("Waiting for VirtualPaymentApp deployment confirmation")
	_, err = b.WaitMined(ctx, ethClient, tx)
	if err != nil {
		return types.Address{}, types.Address{}, types.Address{}, err
	}
	fmt.Printf("Deployed VirtualPaymentApp at %s\n", vpa.String())

	ca, tx, _, err = ConsensusApp.DeployConsensusApp(txSubmitter, ethClient)
	if err != nil {
		return types.Address{}, types.Address{}, types.Address{}, err
	}

	fmt.Println("Waiting for ConsensusApp deployment confirmation")
	_, err = b.WaitMined(ctx, ethClient, tx)
	if err != nil {
		return types.Address{}, types.Address{}, types.Address{}, err
	}

	fmt.Printf("Deployed ConsensusApp at %s\n", ca.String())
	return
}
