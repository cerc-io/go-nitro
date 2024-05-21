package node_test

import (
	"context"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/channel/state"
	ta "github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	NitroAdjudicator "github.com/statechannels/go-nitro/node/engine/chainservice/adjudicator"
	"github.com/statechannels/go-nitro/node/engine/messageservice"
	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

func TestChallenge(t *testing.T) {
	const challengeDuration = 5

	// Start the chain & deploy contract
	t.Log("Starting chain")
	sim, bindings, ethAccounts, err := chainservice.SetupSimulatedBackend(2)
	defer closeSimulatedChain(t, sim)
	if err != nil {
		t.Fatal(err)
	}

	// Create go-nitro nodes
	msgBroker := messageservice.NewBroker()
	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()
	nodeA, storeA, _ := setupTestNode(sim, bindings, ethAccounts[0], ta.Alice.PrivateKey, msgBroker, dataFolder)
	nodeB, _, _ := setupTestNode(sim, bindings, ethAccounts[1], ta.Bob.PrivateKey, msgBroker, dataFolder)
	defer closeNode(t, &nodeA)

	testChainServiceA, _ := chainservice.NewSimulatedBackendChainService(sim, bindings, ethAccounts[0])
	defer testChainServiceA.Close()

	// Create ledger channel
	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, types.Address{}, challengeDuration)

	// Check balance of node
	latestBlock, _ := sim.BlockByNumber(context.Background(), nil)
	balanceNodeA, _ := sim.BalanceAt(context.Background(), ta.Alice.Address(), latestBlock.Number())
	balanceNodeB, _ := sim.BalanceAt(context.Background(), ta.Bob.Address(), latestBlock.Number())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	testhelpers.Assert(t, balanceNodeA.Int64() == 0, "Balance of Alice should be zero")
	testhelpers.Assert(t, balanceNodeB.Int64() == 0, "Balance of Bob should be zero")

	// Close the Bob's node
	closeNode(t, &nodeB)

	// Alice calls challenge method
	// The sendTransaction method from simulatedBackendService mints three blocks
	// The timestamp of each succeeding block is 10 seconds more than previous block hence calling sendTransaction moves the time forward by 30 seconds
	// Hence challenge duration is over as it is less than 30 seconds and channel is computed as finalized
	signedState := getLatestSignedState(storeA, ledgerChannel)
	sendChallengeTransaction(t, signedState, ta.Alice.PrivateKey, ledgerChannel, testChainServiceA)

	// Alice calls transferAllAssets method
	transferTx := protocols.NewTransferAllTransaction(ledgerChannel, signedState)
	err = testChainServiceA.SendTransaction(transferTx)
	if err != nil {
		t.Error(err)
	}
	// TODO: Update off chain states

	// Check assets are liquidated
	latestBlock, _ = sim.BlockByNumber(context.Background(), nil)
	balanceA, _ := sim.BalanceAt(context.Background(), ta.Alice.Address(), latestBlock.Number())
	balanceB, _ := sim.BalanceAt(context.Background(), ta.Bob.Address(), latestBlock.Number())
	t.Log("Balance of Alice", balanceA, "\nBalance of Bob", balanceB)
	// Assert balance equals ledger channel deposit since no payment has been made
	testhelpers.Assert(t, balanceA.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "Balance of Alice (%v) should be equal to ledgerChannelDeposit (%v)", balanceA, ledgerChannelDeposit)
	testhelpers.Assert(t, balanceB.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "Balance of Bob (%v) should be equal to ledgerChannelDeposit (%v)", balanceB, ledgerChannelDeposit)
}

func TestCheckpoint(t *testing.T) {
	// The sendTransaction method from simulatedBackendService mints three blocks
	// The timestamp of each succeeding block is 10 seconds more than previous block hence calling sendTransaction moves the time forward by 30 seconds
	// Hence if challenge duration is less than or equal to 30, on calling checkpoint method channel is computed as finalized
	// Therefore, challenge duration of 31 or greater is necessary
	const challengeDuration = 31

	// Start the chain & deploy contract
	t.Log("Starting chain")
	sim, bindings, ethAccounts, err := chainservice.SetupSimulatedBackend(2)
	defer closeSimulatedChain(t, sim)
	if err != nil {
		t.Fatal(err)
	}

	// Create go-nitro nodes
	msgBroker := messageservice.NewBroker()
	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()
	nodeA, storeA, chainServiceA := setupTestNode(sim, bindings, ethAccounts[0], ta.Alice.PrivateKey, msgBroker, dataFolder)
	nodeB, storeB, chainServiceB := setupTestNode(sim, bindings, ethAccounts[1], ta.Bob.PrivateKey, msgBroker, dataFolder)
	defer closeNode(t, &nodeA)
	defer closeNode(t, &nodeB)

	// Seperate chain service to listen for events
	testChainServiceB, _ := chainservice.NewSimulatedBackendChainService(sim, bindings, ethAccounts[1])
	defer testChainServiceB.Close()

	// Create ledger channel and check balance of node
	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, types.Address{}, challengeDuration)

	// Store current state
	oldState := getLatestSignedState(storeA, ledgerChannel)

	// Conduct virtual fund and virtual defund
	virtualOutcome := initialPaymentOutcome(*nodeA.Address, *nodeB.Address, common.BigToAddress(common.Big0))
	response, err := nodeA.CreatePaymentChannel([]common.Address{}, *nodeB.Address, challengeDuration, virtualOutcome)
	if err != nil {
		t.Error(err)
	}
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{response.Id})
	virtualDefundResponse, err := nodeA.ClosePaymentChannel(response.ChannelId)
	if err != nil {
		t.Error(err)
	}
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{virtualDefundResponse})

	// Store current state after virtual fund and defund
	newState := getLatestSignedState(storeB, ledgerChannel)

	// Alice calls challenge method using old state
	sendChallengeTransaction(t, oldState, ta.Alice.PrivateKey, ledgerChannel, chainServiceA)

	// Bob listens for challenge registered event
	event := waitForEvent(t, testChainServiceB.EventFeed(), chainservice.ChallengeRegisteredEvent{})
	t.Log("Challenge registed event received", event)

	// Bob calls checkpoint method using new state
	checkpointTx := protocols.NewCheckpointTransaction(ledgerChannel, newState, make([]state.SignedState, 0))
	err = chainServiceB.SendTransaction(checkpointTx)
	if err != nil {
		t.Error(err)
	}

	// Listen for challenge cleared event
	event = waitForEvent(t, testChainServiceB.EventFeed(), chainservice.ChallengeClearedEvent{})
	t.Log("Challenge cleared event received", event)
	challengeClearedEvent, ok := event.(chainservice.ChallengeClearedEvent)
	testhelpers.Assert(t, ok, "Expect challenge cleared event")
	testhelpers.Assert(t, challengeClearedEvent.ChannelID() == ledgerChannel, "Channel ID mismatch")

	// Alice attempts to liquidate the asset after the challenge duration, but the attempt fails because the outcome has not been finalized
	transferTx := protocols.NewTransferAllTransaction(ledgerChannel, oldState)
	err = chainServiceA.SendTransaction(transferTx)
	testhelpers.Assert(t, err.Error() == "execution reverted: Channel not finalized.", "Expects execution reverted error")
}

func TestCounterChallenge(t *testing.T) {
	// The sendTransaction method from simulatedBackendService mints three blocks
	// The timestamp of each succeeding block is 10 seconds more than previous block hence calling sendTransaction moves the time forward by 30 seconds
	// Hence if challenge duration is less than or equal to 30, on calling challenge method again channel is computed as finalized
	// Therefore, challenge duration of 31 or greater is necessary
	const ChallengeDuration = 31

	// Start the chain & deploy contract
	t.Log("Starting chain")
	sim, bindings, ethAccounts, err := chainservice.SetupSimulatedBackend(2)
	defer closeSimulatedChain(t, sim)
	if err != nil {
		t.Fatal(err)
	}

	// Create go-nitro nodes
	msgBroker := messageservice.NewBroker()
	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()
	nodeA, storeA, chainServiceA := setupTestNode(sim, bindings, ethAccounts[0], ta.Alice.PrivateKey, msgBroker, dataFolder)
	nodeB, storeB, chainServiceB := setupTestNode(sim, bindings, ethAccounts[1], ta.Bob.PrivateKey, msgBroker, dataFolder)
	defer closeNode(t, &nodeA)
	defer closeNode(t, &nodeB)

	// Seperate chain service to listen for events
	testChainServiceB, _ := chainservice.NewSimulatedBackendChainService(sim, bindings, ethAccounts[1])
	defer testChainServiceB.Close()

	// Create ledger channel and check balance of node
	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, types.Address{}, ChallengeDuration)
	latestBlock, _ := sim.BlockByNumber(context.Background(), nil)
	balanceNodeA, _ := sim.BalanceAt(context.Background(), ta.Alice.Address(), latestBlock.Number())
	balanceNodeB, _ := sim.BalanceAt(context.Background(), ta.Bob.Address(), latestBlock.Number())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	testhelpers.Assert(t, balanceNodeA.Int64() == 0, "Balance of Alice should be zero")
	testhelpers.Assert(t, balanceNodeB.Int64() == 0, "Balance of Bob should be zero")

	// Store current state
	oldState := getLatestSignedState(storeA, ledgerChannel)

	// Conduct virtual fund, make payment and virtual defund
	virtualOutcome := initialPaymentOutcome(*nodeA.Address, *nodeB.Address, common.BigToAddress(common.Big0))
	response, err := nodeA.CreatePaymentChannel([]common.Address{}, *nodeB.Address, ChallengeDuration, virtualOutcome)
	if err != nil {
		t.Error(err)
	}
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{response.Id})
	nodeA.Pay(response.ChannelId, big.NewInt(virtualChannelDeposit))
	nodeBVoucher := <-nodeB.ReceivedVouchers()
	t.Logf("Voucher recieved %+v", nodeBVoucher)
	virtualDefundResponse, err := nodeA.ClosePaymentChannel(response.ChannelId)
	if err != nil {
		t.Error(err)
	}
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{virtualDefundResponse})

	// Store current state after payment and virtual defund
	newState := getLatestSignedState(storeB, ledgerChannel)

	// Alice calls challenge method using old state
	sendChallengeTransaction(t, oldState, ta.Alice.PrivateKey, ledgerChannel, chainServiceA)

	// Bob listens for challenge registered event
	event := waitForEvent(t, testChainServiceB.EventFeed(), chainservice.ChallengeRegisteredEvent{})
	t.Log("Challenge registed event received", event)
	_, ok := event.(chainservice.ChallengeRegisteredEvent)
	testhelpers.Assert(t, ok, "Expect challenge registered event")

	// Bob calls challenge method using new state
	sendChallengeTransaction(t, newState, ta.Bob.PrivateKey, ledgerChannel, chainServiceB)

	// Listen for challenge register event
	event = waitForEvent(t, testChainServiceB.EventFeed(), chainservice.ChallengeRegisteredEvent{})
	t.Log("Challenge registed event received", event)
	_, ok = event.(chainservice.ChallengeRegisteredEvent)
	testhelpers.Assert(t, ok, "Expect challenge registered event")

	// Transfer can be done only after channel is finalized
	// Due to SendTransaction, 2 additional blocks have been minted (moved ahead by 20s)
	// Mint additional block (10s) for channel to get finalized
	sim.Commit()

	// Alice attempts to liquidate an asset with an outdated state but fails
	transferTx := protocols.NewTransferAllTransaction(ledgerChannel, oldState)
	err = chainServiceB.SendTransaction(transferTx)
	testhelpers.Assert(t, err.Error() == "execution reverted: incorrect fingerprint", "Expects execution reverted error")

	// Bob calls transferAllAssets method using new state
	transferTx = protocols.NewTransferAllTransaction(ledgerChannel, newState)
	err = chainServiceB.SendTransaction(transferTx)
	if err != nil {
		t.Error(err)
	}

	// Check assets are liquidated
	latestBlock, _ = sim.BlockByNumber(context.Background(), nil)
	balanceA, _ := sim.BalanceAt(context.Background(), ta.Alice.Address(), latestBlock.Number())
	balanceB, _ := sim.BalanceAt(context.Background(), ta.Bob.Address(), latestBlock.Number())
	t.Log("Balance of Alice", balanceA, "\nBalance of Bob", balanceB)
	// Alice's balance is calculated by adding her ledger deposit to the amount received through payments, while Bob's balance is determined by subtracting amount paid from his ledger deposit
	testhelpers.Assert(t, balanceA.Cmp(big.NewInt(ledgerChannelDeposit-virtualChannelDeposit)) == 0, "Balance of Alice  (%v) should be equal to (%v)", balanceA, ledgerChannelDeposit-virtualChannelDeposit)
	testhelpers.Assert(t, balanceB.Cmp(big.NewInt(ledgerChannelDeposit+virtualChannelDeposit)) == 0, "Balance of Bob (%v) should be equal to (%v)", balanceB, ledgerChannelDeposit+virtualChannelDeposit)
}

func sendChallengeTransaction(t *testing.T, signedState state.SignedState, privateKey []byte, ledgerChannel types.Destination, chainService chainservice.ChainService) {
	challengerSig, _ := NitroAdjudicator.SignChallengeMessage(signedState.State(), privateKey)
	challengeTx := protocols.NewChallengeTransaction(ledgerChannel, signedState, make([]state.SignedState, 0), challengerSig)
	err := chainService.SendTransaction(challengeTx)
	if err != nil {
		t.Error(err)
	}
}

func setupTestNode(sim chainservice.SimulatedChain, bindings chainservice.Bindings, ethAccount *bind.TransactOpts, privateKey []byte, msgBroker messageservice.Broker, dataFolder string) (node.Node, store.Store, chainservice.ChainService) {
	chainService, _ := chainservice.NewSimulatedBackendChainService(sim, bindings, ethAccount)
	node, store := setupNode(privateKey, chainService, msgBroker, 0, dataFolder)
	return node, store, chainService
}

func waitForEvent(t *testing.T, eventChannel <-chan chainservice.Event, eventType chainservice.Event) chainservice.Event {
	for event := range eventChannel {
		if reflect.TypeOf(event) == reflect.TypeOf(eventType) {
			return event
		} else {
			t.Log("Ignoring other events")
		}
	}
	return nil
}

func getLatestSignedState(store store.Store, id types.Destination) state.SignedState {
	consensusChannel, _ := store.GetConsensusChannelById(id)
	return consensusChannel.SupportedSignedState()
}
