package node_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/cmd/utils"
	"github.com/statechannels/go-nitro/internal/chain"
	"github.com/statechannels/go-nitro/internal/testactors"
	ta "github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	NitroAdjudicator "github.com/statechannels/go-nitro/node/engine/chainservice/adjudicator"
	"github.com/statechannels/go-nitro/node/engine/messageservice"
	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

func TestChallenge(t *testing.T) {
	tc := TestCase{
		Description:       "Challenge test",
		Chain:             AnvilChain,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Challenge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
		},
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infra := setupSharedInfra(tc)
	defer infra.Close(t)

	// Create go-nitro nodes
	nodeA, _, _, storeA, chainServiceA := setupIntegrationNode(tc, tc.Participants[0], infra, []string{}, dataFolder)
	defer nodeA.Close()

	nodeB, _, _, _, _ := setupIntegrationNode(tc, tc.Participants[1], infra, []string{}, dataFolder)

	// Separate chain service to listen for events
	testChainServiceA := setupChainService(tc, tc.Participants[0], infra)
	defer testChainServiceA.Close()

	// Create ledger channel
	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, types.Address{}, uint32(tc.ChallengeDuration))

	// Check balance of node
	balanceNodeA, _ := infra.anvilChain.GetAccountBalance(tc.Participants[0].Address())
	balanceNodeB, _ := infra.anvilChain.GetAccountBalance(tc.Participants[1].Address())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	testhelpers.Assert(t, balanceNodeA.Int64() == 0, "Balance of Alice should be zero")
	testhelpers.Assert(t, balanceNodeB.Int64() == 0, "Balance of Bob should be zero")

	// Close the Bob's node
	closeNode(t, &nodeB)

	// Alice calls challenge method
	signedState := getLatestSignedState(storeA, ledgerChannel)
	sendChallengeTransaction(t, signedState, tc.Participants[0].PrivateKey, ledgerChannel, testChainServiceA)

	// Listen for challenge registered event
	event := waitForEvent(t, testChainServiceA.EventFeed(), chainservice.ChallengeRegisteredEvent{})
	challengeRegisteredEvent, ok := event.(chainservice.ChallengeRegisteredEvent)
	testhelpers.Assert(t, ok, "Expected challenge registered event")

	time.Sleep(time.Duration(tc.ChallengeDuration) * time.Second)
	latestBlock, _ := infra.anvilChain.GetLatestBlock()
	testhelpers.Assert(t, challengeRegisteredEvent.FinalizesAt.Uint64() <= latestBlock.Header().Time, "Expected channel to be finalized")

	// Alice calls transferAllAssets method
	transferTx := protocols.NewTransferAllTransaction(ledgerChannel, signedState)
	err := chainServiceA.SendTransaction(transferTx)
	if err != nil {
		t.Error(err)
	}

	// Listen for allocation updated event
	event = waitForEvent(t, testChainServiceA.EventFeed(), chainservice.AllocationUpdatedEvent{})
	_, ok = event.(chainservice.AllocationUpdatedEvent)
	testhelpers.Assert(t, ok, "Expected allocation updated event")

	// TODO: Update off chain states

	// Check assets are liquidated
	balanceNodeA, _ = infra.anvilChain.GetAccountBalance(tc.Participants[0].Address())
	balanceNodeB, _ = infra.anvilChain.GetAccountBalance(tc.Participants[1].Address())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	// Assert balance equals ledger channel deposit since no payment has been made
	testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "Balance of Alice (%v) should be equal to ledgerChannelDeposit (%v)", balanceNodeA, ledgerChannelDeposit)
	testhelpers.Assert(t, balanceNodeB.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "Balance of Bob (%v) should be equal to ledgerChannelDeposit (%v)", balanceNodeB, ledgerChannelDeposit)
}

func TestCheckpoint(t *testing.T) {
	tc := TestCase{
		Description:       "Checkpoint test",
		Chain:             AnvilChain,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Checkpoint_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
		},
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infra := setupSharedInfra(tc)
	defer infra.Close(t)

	// Create go-nitro nodes
	nodeA, _, _, storeA, chainServiceA := setupIntegrationNode(tc, tc.Participants[0], infra, []string{}, dataFolder)
	defer nodeA.Close()
	nodeB, _, _, storeB, chainServiceB := setupIntegrationNode(tc, tc.Participants[1], infra, []string{}, dataFolder)
	defer nodeB.Close()

	// Separate chain service to listen for events
	testChainServiceB := setupChainService(tc, tc.Participants[1], infra)
	defer testChainServiceB.Close()

	// Create ledger channel
	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, types.Address{}, uint32(tc.ChallengeDuration))

	// Store current state
	oldState := getLatestSignedState(storeA, ledgerChannel)

	// Conduct virtual fund and virtual defund
	virtualOutcome := initialPaymentOutcome(*nodeA.Address, *nodeB.Address, common.BigToAddress(common.Big0))
	response, err := nodeA.CreatePaymentChannel([]common.Address{}, *nodeB.Address, uint32(tc.ChallengeDuration), virtualOutcome)
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
	sendChallengeTransaction(t, oldState, tc.Participants[0].PrivateKey, ledgerChannel, chainServiceA)

	// Bob listens for challenge registered event
	event := waitForEvent(t, testChainServiceB.EventFeed(), chainservice.ChallengeRegisteredEvent{})
	t.Log("Challenge registed event received", event)
	challengeRegisteredEvent, ok := event.(chainservice.ChallengeRegisteredEvent)
	testhelpers.Assert(t, ok, "Expected challenge registered event")
	latestBlock, _ := infra.anvilChain.GetLatestBlock()
	testhelpers.Assert(t, latestBlock.Header().Time < challengeRegisteredEvent.FinalizesAt.Uint64(), "Expected channel to not be finalized")

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
	testhelpers.Assert(t, ok, "Expected challenge cleared event")
	testhelpers.Assert(t, challengeClearedEvent.ChannelID() == ledgerChannel, "Channel ID mismatch")

	time.Sleep(time.Duration(tc.ChallengeDuration) * time.Second)
	latestBlock, _ = infra.anvilChain.GetLatestBlock()
	testhelpers.Assert(t, challengeRegisteredEvent.FinalizesAt.Uint64() <= latestBlock.Header().Time, "Expected challenge duration to be completed")

	// Alice attempts to liquidate the asset after the challenge duration, but the attempt fails because the outcome has not been finalized
	transferTx := protocols.NewTransferAllTransaction(ledgerChannel, oldState)
	err = chainServiceA.SendTransaction(transferTx)
	testhelpers.Assert(t, err.Error() == "execution reverted: revert: Channel not finalized.", "Expected execution reverted error")
}

func TestCounterChallenge(t *testing.T) {
	const payAmount = 2000

	tc := TestCase{
		Description:       "Counter challenge test",
		Chain:             AnvilChain,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Counter_challenge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
		},
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infra := setupSharedInfra(tc)
	defer infra.Close(t)

	// Create go-nitro nodes
	nodeA, _, _, storeA, chainServiceA := setupIntegrationNode(tc, tc.Participants[0], infra, []string{}, dataFolder)
	defer nodeA.Close()
	nodeB, _, _, storeB, chainServiceB := setupIntegrationNode(tc, tc.Participants[1], infra, []string{}, dataFolder)
	defer nodeB.Close()

	// Seperate chain service to listen for events
	testChainServiceB := setupChainService(tc, tc.Participants[1], infra)
	defer testChainServiceB.Close()

	// Create ledger channel and check balance of node
	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, types.Address{}, uint32(tc.ChallengeDuration))
	balanceNodeA, _ := infra.anvilChain.GetAccountBalance(tc.Participants[0].Address())
	balanceNodeB, _ := infra.anvilChain.GetAccountBalance(tc.Participants[1].Address())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	testhelpers.Assert(t, balanceNodeA.Int64() == 0, "Balance of Alice should be zero")
	testhelpers.Assert(t, balanceNodeB.Int64() == 0, "Balance of Bob should be zero")

	// Store current state
	oldState := getLatestSignedState(storeA, ledgerChannel)

	// Conduct virtual fund, make payment and virtual defund
	virtualOutcome := initialPaymentOutcome(*nodeA.Address, *nodeB.Address, common.BigToAddress(common.Big0))
	response, err := nodeA.CreatePaymentChannel([]common.Address{}, *nodeB.Address, uint32(tc.ChallengeDuration), virtualOutcome)
	if err != nil {
		t.Error(err)
	}
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{response.Id})
	// Alice pays Bob
	nodeA.Pay(response.ChannelId, big.NewInt(payAmount))
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
	sendChallengeTransaction(t, oldState, tc.Participants[0].PrivateKey, ledgerChannel, chainServiceA)

	// Bob listens for challenge registered event
	event := waitForEvent(t, testChainServiceB.EventFeed(), chainservice.ChallengeRegisteredEvent{})
	t.Log("Challenge registed event received", event)
	challengeRegisteredEvent, ok := event.(chainservice.ChallengeRegisteredEvent)
	testhelpers.Assert(t, ok, "Expected challenge registered event")

	latestBlock, _ := infra.anvilChain.GetLatestBlock()
	testhelpers.Assert(t, latestBlock.Header().Time < challengeRegisteredEvent.FinalizesAt.Uint64(), "Expected channel to not be finalized")

	// Bob calls challenge method using new state
	sendChallengeTransaction(t, newState, tc.Participants[1].PrivateKey, ledgerChannel, chainServiceB)

	// Listen for challenge register event
	event = waitForEvent(t, testChainServiceB.EventFeed(), chainservice.ChallengeRegisteredEvent{})
	t.Log("Challenge registed event received", event)
	challengeRegisteredEvent, ok = event.(chainservice.ChallengeRegisteredEvent)
	testhelpers.Assert(t, ok, "Expected challenge registered event")

	time.Sleep(time.Duration(tc.ChallengeDuration) * time.Second)
	latestBlock, _ = infra.anvilChain.GetLatestBlock()
	testhelpers.Assert(t, challengeRegisteredEvent.FinalizesAt.Uint64() <= latestBlock.Header().Time, "Expected channel to be finalized")

	// Alice attempts to liquidate an asset with an outdated state but fails
	transferTx := protocols.NewTransferAllTransaction(ledgerChannel, oldState)
	err = chainServiceB.SendTransaction(transferTx)
	testhelpers.Assert(t, err.Error() == "execution reverted: revert: incorrect fingerprint", "Expected execution reverted error")

	// Bob calls transferAllAssets method using new state
	transferTx = protocols.NewTransferAllTransaction(ledgerChannel, newState)
	err = chainServiceB.SendTransaction(transferTx)
	if err != nil {
		t.Error(err)
	}
	// TODO: Update off chain states

	// Listen for allocation updated event
	event = waitForEvent(t, testChainServiceB.EventFeed(), chainservice.AllocationUpdatedEvent{})
	_, ok = event.(chainservice.AllocationUpdatedEvent)
	testhelpers.Assert(t, ok, "Expected allocation updated event")

	// Check assets are liquidated
	balanceNodeA, _ = infra.anvilChain.GetAccountBalance(tc.Participants[0].Address())
	balanceNodeB, _ = infra.anvilChain.GetAccountBalance(tc.Participants[1].Address())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	// Alice's balance is determined by subtracting amount paid from her ledger deposit, while Bob's balance is calculated by adding his ledger deposit to the amount received
	testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of Alice  (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit-payAmount)
	testhelpers.Assert(t, balanceNodeB.Cmp(big.NewInt(ledgerChannelDeposit+payAmount)) == 0, "Balance of Bob (%v) should be equal to (%v)", balanceNodeB, ledgerChannelDeposit+payAmount)
}

func sendChallengeTransaction(t *testing.T, signedState state.SignedState, privateKey []byte, ledgerChannel types.Destination, chainService chainservice.ChainService) {
	challengerSig, _ := NitroAdjudicator.SignChallengeMessage(signedState.State(), privateKey)
	challengeTx := protocols.NewChallengeTransaction(ledgerChannel, signedState, make([]state.SignedState, 0), challengerSig)
	err := chainService.SendTransaction(challengeTx)
	if err != nil {
		t.Error(err)
	}
}

func setupNodeAndChainService(sim chainservice.SimulatedChain, bindings chainservice.Bindings, ethAccount *bind.TransactOpts, privateKey []byte, msgBroker messageservice.Broker, dataFolder string) (node.Node, store.Store, chainservice.ChainService) {
	chainService, _ := chainservice.NewSimulatedBackendChainService(sim, bindings, ethAccount)
	node, store := setupNode(privateKey, chainService, msgBroker, 0, dataFolder)
	return node, store, chainService
}

func TestVirtualPaymentChannel(t *testing.T){
	const challengeDuration = 5

	// Start the chain & deploy contract
	t.Log("Starting chain")
	sim, bindings, ethAccounts, err := chainservice.SetupSimulatedBackend(2)
	defer closeSimulatedChain(t, sim)
	if err != nil {
		t.Fatal(err)
	}

	// Create go-nitro nodes
	chainServiceA, _ := chainservice.NewSimulatedBackendChainService(sim, bindings, ethAccounts[0])
	chainServiceB, _ := chainservice.NewSimulatedBackendChainService(sim, bindings, ethAccounts[1])
	testChainServiceA, _ := chainservice.NewSimulatedBackendChainService(sim, bindings, ethAccounts[0])
	msgBroker := messageservice.NewBroker()
	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()
	nodeA, storeA := setupNode(ta.Alice.PrivateKey, chainServiceA, msgBroker, 0, dataFolder)
	defer closeNode(t, &nodeA)
	nodeB, _ := setupNode(ta.Bob.PrivateKey, chainServiceB, msgBroker, 0, dataFolder)

	// Create ledger channel
	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, types.Address{}, challengeDuration)

	// Check balance of node
	latestBlock, _ := sim.BlockByNumber(context.Background(), nil)
	balanceNodeA, _ := sim.BalanceAt(context.Background(), ethAccounts[0].From, latestBlock.Number())
	balanceNodeB, _ := sim.BalanceAt(context.Background(), ethAccounts[1].From, latestBlock.Number())
	t.Log("Balance of node A", balanceNodeA, "\nBalance of Node B", balanceNodeB)

	// Create virtual channel
	virtualOutcome := initialPaymentOutcome(*nodeA.Address, *nodeB.Address, types.Address{})
	virtualResponse, _ := nodeA.CreatePaymentChannel([]common.Address{}, *nodeB.Address, challengeDuration, virtualOutcome)

	// Wait for objective to complete
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{virtualResponse.Id})
	checkPaymentChannel(t, virtualResponse.ChannelId, virtualOutcome, query.Open, nodeA, nodeB)

	// Make payment
	nodeA.Pay(virtualResponse.ChannelId, big.NewInt(int64(100)))

	// Wait for node B to recieve voucher
	nodeBVoucher := <-nodeB.ReceivedVouchers()
	t.Log("Voucher recieved", nodeBVoucher)

	targetFinalOutcome := finalPaymentOutcome(*nodeA.Address, *nodeB.Address, types.Address{}, 1, 100)
	checkPaymentChannel(t, virtualResponse.ChannelId, targetFinalOutcome, query.Open, nodeA, nodeB)

	closeNode(t, &nodeB)

	// Call Reclaim method
	signedUpdatedLedgerState := getLatestSignedState(storeA, ledgerChannel)
	signedStateHash, _ := signedUpdatedLedgerState.State().Hash()
	virtualLatestState, _ := getVirtualSignedState(storeA, virtualResponse.ChannelId)
	virtualStateHash, _ := virtualLatestState.State().Hash()
	sourceOutcome := signedUpdatedLedgerState.State().Outcome
	sourceOb, _ := sourceOutcome.Encode()
	targetOutcome := virtualLatestState.State().Outcome
	targetOb, _ := targetOutcome.Encode()

	reclaimArgs := NitroAdjudicator.IMultiAssetHolderReclaimArgs{
		SourceChannelId:       ledgerChannel,
		SourceStateHash:       signedStateHash,
		SourceOutcomeBytes:    sourceOb,
		SourceAssetIndex:      common.Big0,
		IndexOfTargetInSource: common.Big2,
		TargetStateHash:       virtualStateHash,
		TargetOutcomeBytes:    targetOb,
		TargetAssetIndex:      common.Big0,
	}

	// Node A calls challenge method on virtual channel
	virtualChallengerSig, _ := NitroAdjudicator.SignChallengeMessage(virtualLatestState.State(), ta.Alice.PrivateKey)
	virtualChallengeTx := protocols.NewChallengeTransaction(virtualResponse.ChannelId, virtualLatestState, []state.SignedState{}, virtualChallengerSig)

	err = testChainServiceA.SendTransaction(virtualChallengeTx)
	if err != nil {
		t.Error(err)
	}

		// Wait for challenge duration
		time.Sleep(time.Duration(challengeDuration) * time.Second)

	// Finalize Outcome
	sim.Commit()

	// Node A calls challenge method
	challengerSig, _ := NitroAdjudicator.SignChallengeMessage(signedUpdatedLedgerState.State(), ta.Alice.PrivateKey)
	challengeTx := protocols.NewChallengeTransaction(ledgerChannel, signedUpdatedLedgerState, make([]state.SignedState, 0), challengerSig)
	err = testChainServiceA.SendTransaction(challengeTx)
	if err != nil {
		t.Error(err)
	}

	// Wait for challenge duration
	time.Sleep(time.Duration(challengeDuration) * time.Second)

	// Finalize Outcome
	sim.Commit()

	reclaimTx := protocols.NewReclaimTransaction(ledgerChannel, reclaimArgs)
	err = testChainServiceA.SendTransaction(reclaimTx)
	if err != nil {
		t.Error(err)
	}

	reclaimedLedgerState := getLatestSignedState(storeA, ledgerChannel)

	// Node A calls transferAllAssets method
	transferTx := protocols.NewTransferAllTransaction(ledgerChannel, reclaimedLedgerState)
	err = testChainServiceA.SendTransaction(transferTx)
	if err != nil {
		t.Error(err)
	}

	latestBlock, _ = sim.BlockByNumber(context.Background(), nil)
	balanceANodeA, _ := sim.BalanceAt(context.Background(), ethAccounts[0].From, latestBlock.Number())
	balanceANodeB, _ := sim.BalanceAt(context.Background(), ethAccounts[1].From, latestBlock.Number())
	t.Log("Balance of node A", balanceANodeA, "\nBalance of Node B", balanceANodeB)

	// Check assets are liquidated
	latestBlock, _ = sim.BlockByNumber(context.Background(), nil)
	balanceA, _ := sim.BalanceAt(context.Background(), ta.Alice.Address(), latestBlock.Number())
	balanceB, _ := sim.BalanceAt(context.Background(), ta.Bob.Address(), latestBlock.Number())
	t.Log("Balance of A", balanceA, "\nBalance of B", balanceB)
	testhelpers.Assert(t, balanceA.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "BalanceA (%v) should be equal to ledgerChannelDeposit (%v)", balanceA, ledgerChannelDeposit)
	testhelpers.Assert(t, balanceB.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "BalanceB (%v) should be equal to ledgerChannelDeposit (%v)", balanceB, ledgerChannelDeposit)
}

func TestVirtualPaymentChannelUsingAnvil(t *testing.T) {
	const challengeDuration = 5

	// TODO: Remove after getting latest state for transferAllAssets transaction
	t.Skip()
	anvilCmd, _ := chain.StartAnvil()
	defer utils.StopCommands(anvilCmd)

	chainAuthToken := ""
	chainUrl := "ws://127.0.0.1:8545"
	aliceChainPk := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	bobChainPk := "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
	contractAddresses, _ := chain.DeployContracts(context.Background(), chainUrl, chainAuthToken, aliceChainPk)

	chainOptsAlice := chainservice.ChainOpts{
		ChainUrl:        chainUrl,
		ChainStartBlock: 0,
		ChainAuthToken:  chainAuthToken,
		ChainPk:         aliceChainPk,
		NaAddress:       contractAddresses.NaAddress,
		VpaAddress:      contractAddresses.VpaAddress,
		CaAddress:       contractAddresses.CaAddress,
	}
	chainOptsBob := chainservice.ChainOpts{
		ChainUrl:        chainUrl,
		ChainStartBlock: 0,
		ChainAuthToken:  chainAuthToken,
		ChainPk:         bobChainPk,
		NaAddress:       contractAddresses.NaAddress,
		VpaAddress:      contractAddresses.VpaAddress,
		CaAddress:       contractAddresses.CaAddress,
	}
	chainServiceA, _ := chainservice.NewEthChainService(chainOptsAlice)
	chainServiceB, _ := chainservice.NewEthChainService(chainOptsBob)
	testChainServiceA, _ := chainservice.NewEthChainService(chainOptsBob)

	// Create go-nitro nodes
	msgBroker := messageservice.NewBroker()
	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()
	nodeA, storeA := setupNode(ta.Alice.PrivateKey, chainServiceA, msgBroker, 0, dataFolder)
	defer closeNode(t, &nodeA)
	nodeB, _ := setupNode(ta.Bob.PrivateKey, chainServiceB, msgBroker, 0, dataFolder)

	// Create ledger channel
	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, types.Address{}, challengeDuration)

	// Create virtual channel
	virtualOutcome := initialPaymentOutcome(*nodeA.Address, *nodeB.Address, types.Address{})
	virtualResponse, _ := nodeA.CreatePaymentChannel([]common.Address{}, *nodeB.Address, challengeDuration, virtualOutcome)

	// Wait for objective to complete
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{virtualResponse.Id})
	// TODO: Debug checkPaymentChannel method
	// checkPaymentChannel(t, virtualResponse.ChannelId, virtualOutcome, query.Open, nodeA, nodeB)

	// _, oldSignedState  := getVirtualSignedState(storeA, virtualResponse.ChannelId)
	// oldLedgerState := getLatestSignedState(storeA, ledgerChannel)

	// Make payment
	nodeA.Pay(virtualResponse.ChannelId, big.NewInt(int64(100)))

	// Wait for node B to recieve voucher
	nodeBVoucher := <-nodeB.ReceivedVouchers()
	t.Log("Voucher recieved", nodeBVoucher)

	// TODO: Debug checkPaymentChannel method
	// targetFinalOutcome := finalPaymentOutcome(*nodeA.Address, *nodeB.Address, types.Address{}, 1, 100)
	// checkPaymentChannel(t, virtualResponse.ChannelId, targetFinalOutcome, query.Open, nodeA, nodeB)

	closeNode(t, &nodeB)

	// Call Reclaim method
	signedUpdatedLedgerState := getLatestSignedState(storeA, ledgerChannel)
	signedStateHash, _ := signedUpdatedLedgerState.State().Hash()
	virtualLatestState, _ := getVirtualSignedState(storeA, virtualResponse.ChannelId)
	virtualStateHash, _ := virtualLatestState.State().Hash()
	sourceOutcome := signedUpdatedLedgerState.State().Outcome
	sourceOb, _ := sourceOutcome.Encode()
	targetOutcome := virtualLatestState.State().Outcome
	targetOb, _ := targetOutcome.Encode()

	reclaimArgs := NitroAdjudicator.IMultiAssetHolderReclaimArgs{
		SourceChannelId:       ledgerChannel,
		SourceStateHash:       signedStateHash,
		SourceOutcomeBytes:    sourceOb,
		SourceAssetIndex:      common.Big0,
		IndexOfTargetInSource: common.Big2,
		TargetStateHash:       virtualStateHash,
		TargetOutcomeBytes:    targetOb,
		TargetAssetIndex:      common.Big0,
	}

	// Node A calls challenge method on virtual channel
	virtualChallengerSig, _ := NitroAdjudicator.SignChallengeMessage(virtualLatestState.State(), ta.Alice.PrivateKey)
	virtualChallengeTx := protocols.NewChallengeTransaction(virtualResponse.ChannelId, virtualLatestState, []state.SignedState{}, virtualChallengerSig)
	err := testChainServiceA.SendTransaction(virtualChallengeTx)
	if err != nil {
		t.Error(err)
	}

	// Wait for challenge duration
	time.Sleep(time.Duration(challengeDuration) * time.Second)

	// Node A calls challenge method on ledger channel
	challengerSig, _ := NitroAdjudicator.SignChallengeMessage(signedUpdatedLedgerState.State(), ta.Alice.PrivateKey)
	challengeTx := protocols.NewChallengeTransaction(ledgerChannel, signedUpdatedLedgerState, make([]state.SignedState, 0), challengerSig)
	err = testChainServiceA.SendTransaction(challengeTx)
	if err != nil {
		t.Error(err)
	}

	// Wait for challenge duration
	time.Sleep(time.Duration(challengeDuration) * time.Second)

	reclaimTx := protocols.NewReclaimTransaction(ledgerChannel, reclaimArgs)
	err = testChainServiceA.SendTransaction(reclaimTx)
	if err != nil {
		t.Error(err)
	}

	// TODO: Use correct state for transferAllAssets transaction
	// Node A calls transferAllAssets method
	transferTx := protocols.NewTransferAllTransaction(ledgerChannel, signedUpdatedLedgerState)
	err = testChainServiceA.SendTransaction(transferTx)
	if err != nil {
		t.Error(err)
	}
}

func getLatestSignedState(store store.Store, id types.Destination) state.SignedState {
	consensusChannel, _ := store.GetConsensusChannelById(id)
	return consensusChannel.SupportedSignedState()
}

func getVirtualSignedState(store store.Store, id types.Destination) (state.SignedState, state.SignedState) {
	virtualChannel, _ := store.GetChannelById(id)
	virtualSignedState, _ := virtualChannel.LatestSignedState()
	virtualPostfundState := virtualChannel.SignedPostFundState()
	return virtualSignedState, virtualPostfundState
}
