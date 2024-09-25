package chainservice

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"

	"github.com/ethereum/go-ethereum/ethclient/simulated"
	NitroAdjudicator "github.com/statechannels/go-nitro/node/engine/chainservice/adjudicator"
	ConsensusApp "github.com/statechannels/go-nitro/node/engine/chainservice/consensusapp"
	Token "github.com/statechannels/go-nitro/node/engine/chainservice/erc20"
	VirtualPaymentApp "github.com/statechannels/go-nitro/node/engine/chainservice/virtualpaymentapp"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

// This is the chain id used by the simulated backend as well as hardhat
const (
	TEST_CHAIN_ID       = 1337
	TEST_TOKEN_NAME     = "TestToken"
	TEST_TOKEN_SYMBOL   = "TEST"
	TEST_INITIAL_SUPPLY = 100_000_000
)

const (
	TEST_2_TOKEN_NAME   = "TestToken2"
	TEST_2_TOKEN_SYMBOL = "TEST2"
)

var ErrUnableToAssignBigInt = errors.New("simulated_backend_chainservice: unable to assign BigInt")

type binding[T any] struct {
	Address  common.Address
	Contract *T
}

type Bindings struct {
	Adjudicator       binding[NitroAdjudicator.NitroAdjudicator]
	Token             binding[Token.Token]
	ConsensusApp      binding[ConsensusApp.ConsensusApp]
	VirtualPaymentApp binding[VirtualPaymentApp.VirtualPaymentApp]
}

type SimulatedChain interface {
	ethChain
	Commit() common.Hash
	Close() error
}

// This is used to wrap the simulated backend so that we can provide a ChainID function like a real eth client
type BackendWrapper struct {
	*simulated.Backend
	simulated.Client
}

func (b *BackendWrapper) ChainID(ctx context.Context) (*big.Int, error) {
	return big.NewInt(TEST_CHAIN_ID), nil
}

func (b *BackendWrapper) TransactionSender(context.Context, *ethTypes.Transaction, common.Hash, uint) (common.Address, error) {
	return common.Address{}, nil
}

// SimulatedBackendChainService extends EthChainService to automatically mine a block for every transaction
type SimulatedBackendChainService struct {
	*EthChainService
	sim SimulatedChain
}

// NewSimulatedBackendChainService constructs a chain service that submits transactions to a NitroAdjudicator
// and listens to events from an eventSource
func NewSimulatedBackendChainService(sim SimulatedChain, bindings Bindings,
	txSigner *bind.TransactOpts,
) (ChainService, error) {
	ethChainService, err := newEthChainService(sim, 0,
		bindings.Adjudicator.Contract,
		bindings.Adjudicator.Address,
		bindings.ConsensusApp.Address,
		bindings.VirtualPaymentApp.Address,
		txSigner)
	if err != nil {
		return &SimulatedBackendChainService{}, err
	}

	return &SimulatedBackendChainService{sim: sim, EthChainService: ethChainService}, nil
}

// SendTransaction sends the transaction and blocks until it has been mined.
func (sbcs *SimulatedBackendChainService) SendTransaction(tx protocols.ChainTransaction) (*ethTypes.Transaction, error) {
	_, err := sbcs.EthChainService.SendTransaction(tx)
	if err != nil {
		return nil, err
	}
	sbcs.sim.Commit()

	// Mint additional blocks to satisfy REQUIRED_BLOCK_CONFIRMATIONS.
	for i := 0; i < REQUIRED_BLOCK_CONFIRMATIONS; i++ {
		sbcs.sim.Commit()
	}

	return nil, nil
}

// SetupSimulatedBackend creates a new SimulatedBackend with the supplied number of transacting accounts, deploys the Nitro Adjudicator and returns both.
func SetupSimulatedBackend(numAccounts uint64) (SimulatedChain, Bindings, []*bind.TransactOpts, error) {
	accounts := make([]*bind.TransactOpts, numAccounts)
	genesisAlloc := make(map[common.Address]ethTypes.Account)
	contractBindings := Bindings{}

	balance, success := new(big.Int).SetString("10000000000000000000", 10) // 10 eth in wei
	if !success {
		return nil, contractBindings, accounts, ErrUnableToAssignBigInt
	}

	var err error
	for i := range accounts {
		// Setup transacting EOA
		key, _ := crypto.GenerateKey()
		accounts[i], err = bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337)) // 1337 according to docs on SimulatedBackend
		if err != nil {
			return nil, contractBindings, accounts, err
		}
		genesisAlloc[accounts[i].From] = ethTypes.Account{Balance: balance}
	}

	// Setup "blockchain"
	blockGasLimit := uint64(15_000_000)
	sim := simulated.NewBackend(genesisAlloc, func(nodeConf *node.Config, ethConf *ethconfig.Config) {
		ethConf.Genesis.GasLimit = blockGasLimit
	})
	simulatedClient := sim.Client()
	// Deploy Adjudicator
	naAddress, _, na, err := NitroAdjudicator.DeployNitroAdjudicator(accounts[0], simulatedClient)
	if err != nil {
		return nil, contractBindings, accounts, err
	}

	// Deploy ConsensusApp
	consensusAppAddress, _, ca, err := ConsensusApp.DeployConsensusApp(accounts[0], simulatedClient)
	if err != nil {
		return nil, contractBindings, accounts, err
	}

	// Deploy VirtualPaymentChannelApp
	virtualPaymentAppAddress, _, vpa, err := VirtualPaymentApp.DeployVirtualPaymentApp(accounts[0], simulatedClient)
	if err != nil {
		return nil, contractBindings, accounts, err
	}

	// Deploy a test ERC20 Token Contract
	tokenAddress, _, tokenBinding, err := Token.DeployToken(accounts[0], simulatedClient, TEST_TOKEN_NAME, TEST_TOKEN_SYMBOL, accounts[0].From, big.NewInt(TEST_INITIAL_SUPPLY))
	if err != nil {
		return nil, contractBindings, accounts, err
	}

	// https://github.com/ethereum/go-ethereum/issues/15930
	sim.Commit()

	// Distributed tokens to all accounts
	INITIAL_TOKEN_BALANCE := big.NewInt(10_000_000)
	for _, account := range accounts {
		accountAddress := account.From
		_, err := tokenBinding.Transfer(accounts[0], accountAddress, INITIAL_TOKEN_BALANCE)
		if err != nil {
			return nil, contractBindings, accounts, err
		}
	}

	contractBindings = Bindings{
		Adjudicator:       binding[NitroAdjudicator.NitroAdjudicator]{naAddress, na},
		Token:             binding[Token.Token]{tokenAddress, tokenBinding},
		ConsensusApp:      binding[ConsensusApp.ConsensusApp]{consensusAppAddress, ca},
		VirtualPaymentApp: binding[VirtualPaymentApp.VirtualPaymentApp]{virtualPaymentAppAddress, vpa},
	}
	sim.Commit()

	return &BackendWrapper{sim, simulatedClient}, contractBindings, accounts, nil
}

func (sbcs *SimulatedBackendChainService) GetConsensusAppAddress() types.Address {
	return sbcs.consensusAppAddress
}

// GetVirtualPaymentAppAddress returns the address of a deployed VirtualPaymentApp
func (sbcs *SimulatedBackendChainService) GetVirtualPaymentAppAddress() types.Address {
	return sbcs.virtualPaymentAppAddress
}

func (sbcs *SimulatedBackendChainService) DroppedEventFeed() <-chan protocols.DroppedEventInfo {
	return nil
}

func (sbcs *SimulatedBackendChainService) EventFeed() <-chan Event {
	return nil
}
