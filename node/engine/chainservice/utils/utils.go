package chainutils

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	NitroAdjudicator "github.com/statechannels/go-nitro/node/engine/chainservice/adjudicator"
	ConsensusApp "github.com/statechannels/go-nitro/node/engine/chainservice/consensusapp"
	VirtualPaymentApp "github.com/statechannels/go-nitro/node/engine/chainservice/virtualpaymentapp"
	"github.com/statechannels/go-nitro/types"
)

type ContractAddresses struct {
	NaAddress  common.Address
	VpaAddress common.Address
	CaAddress  common.Address
}

// ConnectToChain connects to the chain at the given url and returns a client and a transactor.
func ConnectToChain(ctx context.Context, chainUrl, chainAuthToken string, chainPK []byte) (*ethclient.Client, *bind.TransactOpts, error) {
	var rpcClient *rpc.Client
	var err error

	if chainAuthToken != "" {
		slog.Info("Adding bearer token authorization header to chain service")
		options := rpc.WithHeader("Authorization", "Bearer "+chainAuthToken)
		rpcClient, err = rpc.DialOptions(ctx, chainUrl, options)
	} else {
		rpcClient, err = rpc.DialContext(ctx, chainUrl)
	}
	if err != nil {
		return nil, nil, err
	}

	client := ethclient.NewClient(rpcClient)
	slog.Info("Connected to ethclient", "url", chainUrl)

	foundChainId, err := client.ChainID(context.Background())
	if err != nil {
		return nil, nil, fmt.Errorf("could not get chain id: %w", err)
	}
	slog.Info("Found chain id", "chainId", foundChainId)

	key, err := ethcrypto.ToECDSA(chainPK)
	if err != nil {
		return nil, nil, err
	}
	txSubmitter, err := bind.NewKeyedTransactorWithChainID(key, foundChainId)
	if err != nil {
		return nil, nil, err
	}

	return client, txSubmitter, nil
}

// DeployContracts deploys the NitroAdjudicator, VirtualPaymentApp and ConsensusApp contracts.
func DeployContracts(ctx context.Context, ethClient *ethclient.Client, txSubmitter *bind.TransactOpts) (ContractAddresses, error) {
	na, err := deployContract(ctx, "NitroAdjudicator", ethClient, txSubmitter, NitroAdjudicator.DeployNitroAdjudicator)
	if err != nil {
		return ContractAddresses{}, err
	}

	vpa, err := deployContract(ctx, "VirtualPaymentApp", ethClient, txSubmitter, VirtualPaymentApp.DeployVirtualPaymentApp)
	if err != nil {
		return ContractAddresses{}, err
	}

	ca, err := deployContract(ctx, "ConsensusApp", ethClient, txSubmitter, ConsensusApp.DeployConsensusApp)
	if err != nil {
		return ContractAddresses{}, err
	}

	return ContractAddresses{
		NaAddress:  na,
		VpaAddress: vpa,
		CaAddress:  ca,
	}, nil
}

type contractBackend interface {
	NitroAdjudicator.NitroAdjudicator | VirtualPaymentApp.VirtualPaymentApp | ConsensusApp.ConsensusApp
}

// deployFunc is a function that deploys a contract and returns the contract address, backend, and transaction.
type deployFunc[T contractBackend] func(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *ethTypes.Transaction, *T, error)

// deployContract deploys a contract and waits for the transaction to be mined.
func deployContract[T contractBackend](ctx context.Context, name string, ethClient *ethclient.Client, txSubmitter *bind.TransactOpts, deploy deployFunc[T]) (types.Address, error) {
	a, tx, _, err := deploy(txSubmitter, ethClient)
	if err != nil {
		return types.Address{}, err
	}

	fmt.Printf("Waiting for %s deployment confirmation\n", name)
	_, err = bind.WaitMined(ctx, ethClient, tx)
	if err != nil {
		return types.Address{}, err
	}
	fmt.Printf("%s successfully deployed to %s\n", name, a.String())
	return a, nil
}
