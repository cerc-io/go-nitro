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
	Token "github.com/statechannels/go-nitro/node/engine/chainservice/erc20"
	chainutils "github.com/statechannels/go-nitro/node/engine/chainservice/utils"
)

const (
	CHAIN_AUTH_TOKEN       = ""
	CHAIN_URL_WITHOUT_PORT = "ws://127.0.0.1"
	DEFAULT_CHAIN_PORT     = "8545"
	INITIAL_TOKEN_BALANCE  = 10_000_000
)

// Funded accounts in anvil chain
var ChainPks = []string{
	"ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	"59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d",
	"5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a",
	"7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6",
}

type AnvilChain struct {
	ChainUrl          string
	ChainAuthToken    string
	ChainPks          []string
	AnvilCmd          *exec.Cmd
	EthClient         *ethclient.Client
	ContractAddresses chainutils.ContractAddresses
}

func NewAnvilChain(chainPort string, l2 bool, ethAccountIndex uint) (*AnvilChain, error) {
	if chainPort == "" {
		chainPort = DEFAULT_CHAIN_PORT
	}

	anvilChain, err := StartAnvil(chainPort)
	if err != nil {
		return nil, err
	}

	ethClient, txSubmitter, err := chainutils.ConnectToChain(
		context.Background(),
		anvilChain.ChainUrl,
		anvilChain.ChainAuthToken,
		common.Hex2Bytes(ChainPks[ethAccountIndex]),
	)
	if err != nil {
		return nil, err
	}
	anvilChain.EthClient = ethClient

	// Deploy custom token
	tokenAddress, _, tokenBinding, err := Token.DeployToken(txSubmitter, ethClient, TEST_TOKEN_NAME, TEST_TOKEN_SYMBOL, txSubmitter.From, big.NewInt(TEST_INITIAL_SUPPLY))
	if err != nil {
		return nil, err
	}

	if !l2 {
		anvilChain.ContractAddresses, err = chainutils.DeployContracts(context.Background(), ethClient, txSubmitter)

		// Transfer token
		err := chainutils.TransferToken(ethClient, tokenBinding, txSubmitter, ChainPks, INITIAL_TOKEN_BALANCE)
		if err != nil {
			return nil, err
		}
	}
	anvilChain.ContractAddresses.TokenAddress = tokenAddress

	if err != nil {
		return nil, err
	}

	return &anvilChain, nil
}

func (chain AnvilChain) GetAccountBalance(accountAddress common.Address) (*big.Int, error) {
	latestBlock, _ := chain.GetLatestBlock()
	return chain.EthClient.BalanceAt(context.Background(), accountAddress, latestBlock.Header().Number)
}

func (chain AnvilChain) GetLatestBlock() (*ethTypes.Block, error) {
	return chain.EthClient.BlockByNumber(context.Background(), nil)
}

func StartAnvil(chainPort string) (AnvilChain, error) {
	anvilChain := AnvilChain{}
	anvilChain.ChainUrl = CHAIN_URL_WITHOUT_PORT + ":" + chainPort
	anvilChain.AnvilCmd = exec.Command("anvil", "--chain-id", "1337", "--block-time", "1", "--silent", "--port", chainPort)

	anvilChain.ChainAuthToken = CHAIN_AUTH_TOKEN
	anvilChain.ChainPks = ChainPks

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
