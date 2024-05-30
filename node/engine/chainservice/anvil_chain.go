package chainservice

import (
	"context"
	"math/big"
	"os"
	"os/exec"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	chainutils "github.com/statechannels/go-nitro/node/engine/chainservice/utils"
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
}

type AnvilChain struct {
	ChainUrl          string
	ChainAuthToken    string
	ChainPks          []string
	AnvilCmd          *exec.Cmd
	ethClient         *ethclient.Client
	ContractAddresses chainutils.ContractAddresses
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
	contractAddresses, _ := chainutils.DeployContracts(context.Background(), ethClient, txSubmitter)
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
