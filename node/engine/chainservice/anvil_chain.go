package chainservice

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	NitroAdjudicator "github.com/statechannels/go-nitro/node/engine/chainservice/adjudicator"
	ConsensusApp "github.com/statechannels/go-nitro/node/engine/chainservice/consensusapp"
	chainutils "github.com/statechannels/go-nitro/node/engine/chainservice/utils"
	VirtualPaymentApp "github.com/statechannels/go-nitro/node/engine/chainservice/virtualpaymentapp"
	"github.com/statechannels/go-nitro/types"
)

const (
	CHAIN_AUTH_TOKEN = ""
	CHAIN_URL        = "ws://127.0.0.1:8545"
)

// Funded accounts in anvil chain
var ChainPks = []string{
	"ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	"59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d",
	"5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a",
	"7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6",
}

type ContractAddresses struct {
	NaAddress  common.Address
	VpaAddress common.Address
	CaAddress  common.Address
}

type AnvilChain struct {
	ChainUrl          string
	ChainAuthToken    string
	ChainPks          []string
	AnvilCmd          *exec.Cmd
	ethClient         *ethclient.Client
	ContractAddresses ContractAddresses
}

func NewAnvilChain() (*AnvilChain, error) {
	anvilChain, err := StartAnvil()
	if err != nil {
		return nil, err
	}

	ethClient, txSubmitter, err := chainutils.ConnectToChain(
		context.Background(),
		anvilChain.ChainUrl,
		anvilChain.ChainAuthToken,
		common.Hex2Bytes(ChainPks[0]),
	)
	if err != nil {
		return nil, err
	}
	anvilChain.ethClient = ethClient
	contractAddresses, _ := DeployContracts(context.Background(), ethClient, txSubmitter)
	anvilChain.ContractAddresses = contractAddresses
	return &anvilChain, nil
}

func NewAnvilChainWithChainUrlArg(chainUrl string) (*AnvilChain, error) {
	anvilChain, err := StartAnvilWithChainUrlArg(chainUrl)
	if err != nil {
		return nil, err
	}

	ethClient, txSubmitter, err := chainutils.ConnectToChain(
		context.Background(),
		anvilChain.ChainUrl,
		anvilChain.ChainAuthToken,
		common.Hex2Bytes(ChainPks[0]),
	)
	if err != nil {
		return nil, err
	}
	anvilChain.ethClient = ethClient
	contractAddresses, _ := DeployContracts(context.Background(), ethClient, txSubmitter)
	anvilChain.ContractAddresses = contractAddresses
	return &anvilChain, nil
}

func (chain AnvilChain) GetAccountBalance(accountAddress common.Address) (*big.Int, error) {
	latestBlock, _ := chain.GetLatestBlock()
	return chain.ethClient.BalanceAt(context.Background(), accountAddress, latestBlock.Header().Number)
}

func (chain AnvilChain) GetLatestBlock() (*ethTypes.Block, error) {
	return chain.ethClient.BlockByNumber(context.Background(), nil)
}

func StartAnvil() (AnvilChain, error) {
	anvilChain := AnvilChain{}
	anvilChain.ChainAuthToken = CHAIN_AUTH_TOKEN
	anvilChain.ChainUrl = CHAIN_URL
	anvilChain.ChainPks = ChainPks

	anvilChain.AnvilCmd = exec.Command("anvil", "--chain-id", "1337", "--block-time", "1", "--silent")
	anvilChain.AnvilCmd.Stdout = os.Stdout
	anvilChain.AnvilCmd.Stderr = os.Stderr
	err := anvilChain.AnvilCmd.Start()
	if err != nil {
		return AnvilChain{}, err
	}
	// If Anvil start successfully, delay by 1 second for the chain to initialize
	time.Sleep(1 * time.Second)
	return anvilChain, nil
}

func StartAnvilWithChainUrlArg(chainUrl string) (AnvilChain, error) {
	anvilChain := AnvilChain{}
	anvilChain.ChainAuthToken = CHAIN_AUTH_TOKEN
	anvilChain.ChainUrl = chainUrl
	anvilChain.ChainPks = ChainPks

	chainUrlSplit := strings.Split(chainUrl, ":")

	anvilChain.AnvilCmd = exec.Command("anvil", "--chain-id", "1337", "--block-time", "1", "--silent", "--port", chainUrlSplit[2])
	anvilChain.AnvilCmd.Stdout = os.Stdout
	anvilChain.AnvilCmd.Stderr = os.Stderr
	err := anvilChain.AnvilCmd.Start()
	if err != nil {
		return AnvilChain{}, err
	}
	// If Anvil start successfully, delay by 1 second for the chain to initialize
	time.Sleep(1 * time.Second)
	return anvilChain, nil
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
