package node_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/crypto"
	"github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	NitroAdjudicator "github.com/statechannels/go-nitro/node/engine/chainservice/adjudicator"
	Token "github.com/statechannels/go-nitro/node/engine/chainservice/erc20"

	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/directdefund"
	"github.com/statechannels/go-nitro/types"
)

func TestChallenge(t *testing.T) {
	testCase := TestCase{
		Description:       "Direct defund with Challenge",
		Chain:             AnvilChain,
		MessageService:    TestMessageService,
		ChallengeDuration: 5,
		MessageDelay:      0,
		LogName:           "challenge_test",
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
			{StoreType: MemStore, Actor: testactors.Irene},
		},
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infra := setupSharedInfra(testCase)
	defer infra.Close(t)

	tokenBinding, err := Token.NewToken(infra.anvilChain.ContractAddresses.TokenAddresses[0], infra.anvilChain.EthClient)
	if err != nil {
		t.Fatal(err)
	}

	// Create go-nitro nodes
	nodeA, _, _, storeA, _ := setupIntegrationNode(testCase, testCase.Participants[0], infra, []string{}, dataFolder)
	defer nodeA.Close()
	nodeB, _, _, storeB, _ := setupIntegrationNode(testCase, testCase.Participants[1], infra, []string{}, dataFolder)
	defer nodeB.Close()

	// Create ledger channel
	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, infra.anvilChain.ContractAddresses.TokenAddresses[0], uint32(testCase.ChallengeDuration))

	// Check balance of node
	balanceNodeA, _ := tokenBinding.BalanceOf(nil, testCase.Participants[0].Address())
	balanceNodeB, _ := tokenBinding.BalanceOf(nil, testCase.Participants[1].Address())

	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	testhelpers.Assert(t, balanceNodeA.Int64() == 0, "Balance of Alice should be zero")
	testhelpers.Assert(t, balanceNodeB.Int64() == 0, "Balance of Bob should be zero")

	// Alice initiates the challenge transaction
	response, err := nodeA.CloseLedgerChannel(ledgerChannel, true)
	if err != nil {
		t.Log(err)
	}

	// Wait for Bob's objective to be in challenge mode
	time.Sleep(5 * time.Second)
	objectiveA, _ := storeA.GetObjectiveByChannelId(ledgerChannel)
	objectiveB, _ := storeB.GetObjectiveByChannelId(ledgerChannel)
	objA, _ := objectiveA.(*directdefund.Objective)
	objB, _ := objectiveB.(*directdefund.Objective)

	testhelpers.Assert(t, objA.C.OnChain.ChannelMode == channel.Challenge, "Expected channel status to be challenge")
	testhelpers.Assert(t, objB.C.OnChain.ChannelMode == channel.Challenge, "Expected channel status to be challenge")

	// Wait for objectives to complete
	chA := nodeA.ObjectiveCompleteChan(response)
	chB := nodeB.ObjectiveCompleteChan(response)
	<-chA
	<-chB

	// Check assets are liquidated
	balanceNodeA, _ = tokenBinding.BalanceOf(nil, testCase.Participants[0].Address())
	balanceNodeB, _ = tokenBinding.BalanceOf(nil, testCase.Participants[1].Address())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	// Assert balance equals ledger channel deposit since no payment has been made
	testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "Balance of Alice (%v) should be equal to ledgerChannelDeposit (%v)", balanceNodeA, ledgerChannelDeposit)
	testhelpers.Assert(t, balanceNodeB.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "Balance of Bob (%v) should be equal to ledgerChannelDeposit (%v)", balanceNodeB, ledgerChannelDeposit)
}

func TestCheckpoint(t *testing.T) {
	testCase := TestCase{
		Description:       "Check point test",
		Chain:             AnvilChain,
		MessageService:    TestMessageService,
		ChallengeDuration: 10,
		MessageDelay:      0,
		LogName:           "Checkpoint_test",
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
			{StoreType: MemStore, Actor: testactors.Irene},
		},
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infra := setupSharedInfra(testCase)
	defer infra.Close(t)

	tokenBinding, err := Token.NewToken(infra.anvilChain.ContractAddresses.TokenAddresses[0], infra.anvilChain.EthClient)
	if err != nil {
		t.Fatal(err)
	}

	// Create go-nitro nodes
	nodeA, _, _, storeA, _ := setupIntegrationNode(testCase, testCase.Participants[0], infra, []string{}, dataFolder)
	defer nodeA.Close()
	nodeB, _, _, _, _ := setupIntegrationNode(testCase, testCase.Participants[1], infra, []string{}, dataFolder)
	defer nodeB.Close()

	// Create ledger channel and check balance of node
	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, infra.anvilChain.ContractAddresses.TokenAddresses[0], uint32(testCase.ChallengeDuration))

	// Check balance of node
	balanceNodeA, _ := tokenBinding.BalanceOf(nil, testCase.Participants[0].Address())
	balanceNodeB, _ := tokenBinding.BalanceOf(nil, testCase.Participants[1].Address())

	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	testhelpers.Assert(t, balanceNodeA.Int64() == 0, "Balance of Alice should be zero")
	testhelpers.Assert(t, balanceNodeB.Int64() == 0, "Balance of Bob should be zero")

	oldConsensusChannelAlice, err := storeA.GetConsensusChannelById(ledgerChannel)
	if err != nil {
		t.Error(err)
	}

	ledgerUpdatesChannelNodeB := nodeB.LedgerUpdatedChan(ledgerChannel)

	// Conduct virtual fund, make payment and virtual defund
	virtualOutcome := initialPaymentOutcome(*nodeA.Address, *nodeB.Address, common.BigToAddress(common.Big0))
	response, err := nodeA.CreatePaymentChannel([]common.Address{}, *nodeB.Address, uint32(testCase.ChallengeDuration), virtualOutcome)
	if err != nil {
		t.Error(err)
	}
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{response.Id})
	// Alice pays Bob
	err = nodeA.Pay(response.ChannelId, big.NewInt(payAmount))
	if err != nil {
		t.Fatal(err)
	}

	nodeBVoucher := <-nodeB.ReceivedVouchers()
	t.Logf("Voucher recieved %+v", nodeBVoucher)
	virtualDefundResponse, err := nodeA.ClosePaymentChannel(response.ChannelId)
	if err != nil {
		t.Error(err)
	}
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{virtualDefundResponse})

	// Alice performs a direct defund with a challenge using the old state
	newConsensusChannelAlice, err := storeA.GetConsensusChannelById(ledgerChannel)
	if err != nil {
		t.Error(err)
	}
	err = storeA.SetConsensusChannel(oldConsensusChannelAlice)
	if err != nil {
		t.Log(err)
	}
	res, err := nodeA.CloseLedgerChannel(ledgerChannel, true)
	if err != nil {
		t.Log(err)
	}

	// Bob waits for the channel to enter challenge mode and then counters the registered challenge by checkpoint
	listenForLedgerUpdates(ledgerUpdatesChannelNodeB, channel.Challenge)
	nodeB.CounterChallenge(ledgerChannel, types.Checkpoint, state.SignedState{})

	// Wait for direct defund objectives to complete
	chA := nodeA.ObjectiveCompleteChan(res)
	chB := nodeB.ObjectiveCompleteChan(res)
	<-chA
	<-chB

	// Bob performs normal direct defund
	err = storeA.SetConsensusChannel(newConsensusChannelAlice)
	if err != nil {
		t.Log(err)
	}
	res, err = nodeB.CloseLedgerChannel(ledgerChannel, false)
	if err != nil {
		t.Log(err)
	}
	// Wait for direct defund objectives to complete
	chA = nodeA.ObjectiveCompleteChan(res)
	chB = nodeB.ObjectiveCompleteChan(res)
	<-chA
	<-chB

	// Check assets are liquidated
	balanceNodeA, _ = tokenBinding.BalanceOf(nil, testCase.Participants[0].Address())
	balanceNodeB, _ = tokenBinding.BalanceOf(nil, testCase.Participants[1].Address())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	// Alice's balance is determined by subtracting amount paid from her ledger deposit, while Bob's balance is calculated by adding his ledger deposit to the amount received
	testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of Alice (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit-payAmount)
	testhelpers.Assert(t, balanceNodeB.Cmp(big.NewInt(ledgerChannelDeposit+payAmount)) == 0, "Balance of Bob (%v) should be equal to (%v)", balanceNodeB, ledgerChannelDeposit+payAmount)
}

func TestCounterChallenge(t *testing.T) {
	testCase := TestCase{
		Description:       "Counter challenge test",
		Chain:             AnvilChain,
		MessageService:    TestMessageService,
		ChallengeDuration: 10,
		MessageDelay:      0,
		LogName:           "Counter_challenge_test",
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
			{StoreType: MemStore, Actor: testactors.Irene},
		},
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infra := setupSharedInfra(testCase)
	defer infra.Close(t)

	tokenBinding, err := Token.NewToken(infra.anvilChain.ContractAddresses.TokenAddresses[0], infra.anvilChain.EthClient)
	if err != nil {
		t.Fatal(err)
	}

	// Create go-nitro nodes
	nodeA, _, _, storeA, _ := setupIntegrationNode(testCase, testCase.Participants[0], infra, []string{}, dataFolder)
	defer nodeA.Close()
	nodeB, _, _, _, _ := setupIntegrationNode(testCase, testCase.Participants[1], infra, []string{}, dataFolder)
	defer nodeB.Close()

	// Create ledger channel and check balance of node
	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, infra.anvilChain.ContractAddresses.TokenAddresses[0], uint32(testCase.ChallengeDuration))

	// Check balance of node
	balanceNodeA, _ := tokenBinding.BalanceOf(nil, testCase.Participants[0].Address())
	balanceNodeB, _ := tokenBinding.BalanceOf(nil, testCase.Participants[1].Address())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	testhelpers.Assert(t, balanceNodeA.Int64() == 0, "Balance of Alice should be zero")
	testhelpers.Assert(t, balanceNodeB.Int64() == 0, "Balance of Bob should be zero")

	oldConsensusChannel, err := storeA.GetConsensusChannelById(ledgerChannel)
	if err != nil {
		t.Error(err)
	}

	ledgerUpdatesChannelNodeB := nodeB.LedgerUpdatedChan(ledgerChannel)

	// Conduct virtual fund, make payment and virtual defund
	virtualOutcome := initialPaymentOutcome(*nodeA.Address, *nodeB.Address, common.BigToAddress(common.Big0))
	response, err := nodeA.CreatePaymentChannel([]common.Address{}, *nodeB.Address, uint32(testCase.ChallengeDuration), virtualOutcome)
	if err != nil {
		t.Error(err)
	}
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{response.Id})
	// Alice pays Bob
	err = nodeA.Pay(response.ChannelId, big.NewInt(payAmount))
	if err != nil {
		t.Fatal(err)
	}

	nodeBVoucher := <-nodeB.ReceivedVouchers()
	t.Logf("Voucher recieved %+v", nodeBVoucher)
	virtualDefundResponse, err := nodeA.ClosePaymentChannel(response.ChannelId)
	if err != nil {
		t.Error(err)
	}
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{virtualDefundResponse})

	// Alice performs a direct fund with a challenge using the old state
	err = storeA.SetConsensusChannel(oldConsensusChannel)
	if err != nil {
		t.Log(err)
	}
	res, err := nodeA.CloseLedgerChannel(ledgerChannel, true)
	if err != nil {
		t.Log(err)
	}

	// Bob waits for the channel to enter challenge mode and then counters the registered challenge by raising a new one
	listenForLedgerUpdates(ledgerUpdatesChannelNodeB, channel.Challenge)
	nodeB.CounterChallenge(ledgerChannel, types.Challenge, state.SignedState{})

	// Wait for direct defund objectives to complete
	chA := nodeA.ObjectiveCompleteChan(res)
	chB := nodeB.ObjectiveCompleteChan(res)
	<-chA
	<-chB

	// Check assets are liquidated
	balanceNodeA, _ = tokenBinding.BalanceOf(nil, testCase.Participants[0].Address())
	balanceNodeB, _ = tokenBinding.BalanceOf(nil, testCase.Participants[1].Address())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	// Alice's balance is determined by subtracting amount paid from her ledger deposit, while Bob's balance is calculated by adding his ledger deposit to the amount received
	testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of Alice (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit-payAmount)
	testhelpers.Assert(t, balanceNodeB.Cmp(big.NewInt(ledgerChannelDeposit+payAmount)) == 0, "Balance of Bob (%v) should be equal to (%v)", balanceNodeB, ledgerChannelDeposit+payAmount)
}

func TestVirtualPaymentChannel(t *testing.T) {
	tc := TestCase{
		Description:       "Virtual channel test",
		Chain:             AnvilChain,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Virtual_channel_test",
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
	nodeA, _, _, storeA, _ := setupIntegrationNode(tc, tc.Participants[0], infra, []string{}, dataFolder)
	nodeB, _, _, storeB, chainServiceB := setupIntegrationNode(tc, tc.Participants[1], infra, []string{}, dataFolder)
	defer nodeB.Close()

	// Seperate chain service to listen for events
	testChainService := setupChainService(tc, tc.Participants[1], infra)
	defer testChainService.Close()

	// Create ledger channel
	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, types.Address{}, uint32(tc.ChallengeDuration))

	// Create virtual channel
	virtualOutcome := initialPaymentOutcome(*nodeA.Address, *nodeB.Address, types.Address{})
	virtualResponse, _ := nodeA.CreatePaymentChannel([]common.Address{}, *nodeB.Address, uint32(tc.ChallengeDuration), virtualOutcome)

	// Wait for objective to complete
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{virtualResponse.Id})
	checkPaymentChannel(t, virtualResponse.ChannelId, virtualOutcome, query.Open, nodeA, nodeB)

	// Alice pays Bob
	err := nodeA.Pay(virtualResponse.ChannelId, big.NewInt(payAmount))
	if err != nil {
		t.Fatal(err)
	}

	// Close Alice's node
	closeNode(t, &nodeA)

	// Wait for Bob to recieve voucher
	nodeBVoucher := <-nodeB.ReceivedVouchers()
	t.Logf("Voucher recieved %+v", nodeBVoucher)

	// As Alice's node has closed, Bob has to force the defunding from virtual and ledger channel
	// To call challenge method on virtual channel with voucher, the voucher info needs to be encoded in `AppData` field of channel state

	virtualChannel, _ := storeB.GetChannelById(virtualResponse.ChannelId)
	voucherState, _ := virtualChannel.LatestSignedState()

	// Create type to encode voucher amount and signature
	voucherAmountSigTy, _ := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "amount", Type: "uint256"},
		{Name: "signature", Type: "tuple", Components: []abi.ArgumentMarshaling{
			{Name: "v", Type: "uint8"},
			{Name: "r", Type: "bytes32"},
			{Name: "s", Type: "bytes32"},
		}},
	})

	arguments := abi.Arguments{
		{Type: voucherAmountSigTy},
	}

	voucherAmountSignatureData := protocols.VoucherAmountSignature{
		Amount:    nodeBVoucher.Amount,
		Signature: NitroAdjudicator.ConvertSignature(nodeBVoucher.Signature),
	}

	// Use above created type and encode voucher amount and signature
	dataEncoded, err := arguments.Pack(voucherAmountSignatureData)
	if err != nil {
		t.Fatalf("Failed to encode data: %v", err)
	}

	// Create expected payment outcome
	finalVirtualOutcome := finalPaymentOutcome(*nodeA.Address, *nodeB.Address, common.Address{}, 1, uint(nodeBVoucher.Amount.Int64()))

	// Construct variable part with updated outcome and app data
	vp := state.VariablePart{Outcome: finalVirtualOutcome, TurnNum: voucherState.State().TurnNum + 1, AppData: dataEncoded, IsFinal: voucherState.State().IsFinal}

	// Update state with constructed variable part
	newState := state.StateFromFixedAndVariablePart(voucherState.State().FixedPart(), vp)

	// Bob signs constructed state and adds it to the virtual channel
	_, _ = virtualChannel.SignAndAddState(newState, &tc.Participants[1].PrivateKey)

	// Update store with updated virtual channel
	_ = storeB.SetChannel(virtualChannel)

	// Get updated virtual channel
	updatedVirtualChannel, _ := storeB.GetChannelById(virtualResponse.ChannelId)

	signedLedgerState := getLatestSignedState(storeB, ledgerChannel)
	signedVirtualState, _ := updatedVirtualChannel.LatestSignedState()
	signedPostFundState := updatedVirtualChannel.SignedPostFundState()

	// Bob calls challenge method on virtual channel
	virtualChallengerSig, _ := NitroAdjudicator.SignChallengeMessage(signedVirtualState.State(), tc.Participants[1].PrivateKey)
	virtualChallengeTx := protocols.NewChallengeTransaction(virtualResponse.ChannelId, signedVirtualState, []state.SignedState{signedPostFundState}, virtualChallengerSig)
	_, err = chainServiceB.SendTransaction(virtualChallengeTx)
	if err != nil {
		t.Error(err)
	}

	// Listen for challenge registered event
	event := waitForEvent(t, testChainService.EventEngineFeed(), chainservice.ChallengeRegisteredEvent{})
	t.Log("Challenge registed event received", event)
	challengeRegisteredEvent, ok := event.(chainservice.ChallengeRegisteredEvent)
	testhelpers.Assert(t, ok, "Expected challenge registered event")

	time.Sleep(time.Duration(tc.ChallengeDuration) * time.Second)
	latestBlock, _ := infra.anvilChain.GetLatestBlock()
	testhelpers.Assert(t, challengeRegisteredEvent.FinalizesAt.Uint64() <= latestBlock.Header().Time, "Expected channel to be finalized")

	// Bob calls challenge method on ledger channel
	challengerSig, _ := NitroAdjudicator.SignChallengeMessage(signedLedgerState.State(), tc.Participants[1].PrivateKey)
	challengeTx := protocols.NewChallengeTransaction(ledgerChannel, signedLedgerState, make([]state.SignedState, 0), challengerSig)
	_, err = chainServiceB.SendTransaction(challengeTx)
	if err != nil {
		t.Error(err)
	}

	// Listen for challenge registered event
	event = waitForEvent(t, testChainService.EventEngineFeed(), chainservice.ChallengeRegisteredEvent{})
	t.Log("Challenge registed event received", event)
	challengeRegisteredEvent, ok = event.(chainservice.ChallengeRegisteredEvent)
	testhelpers.Assert(t, ok, "Expected challenge registered event")

	time.Sleep(time.Duration(tc.ChallengeDuration) * time.Second)
	latestBlock, _ = infra.anvilChain.GetLatestBlock()
	testhelpers.Assert(t, challengeRegisteredEvent.FinalizesAt.Uint64() <= latestBlock.Header().Time, "Expected channel to be finalized")

	// Now that ledger and virtual channels are finalized, call reclaim method
	convertedLedgerFixedPart := NitroAdjudicator.ConvertFixedPart(signedLedgerState.State().FixedPart())
	convertedLedgerVariablePart := NitroAdjudicator.ConvertVariablePart(signedLedgerState.State().VariablePart())
	virtualStateHash, _ := signedVirtualState.State().Hash()
	sourceOutcome := signedLedgerState.State().Outcome
	sourceOb, _ := sourceOutcome.Encode()
	targetOutcome := signedVirtualState.State().Outcome
	targetOb, _ := targetOutcome.Encode()

	reclaimArgs := NitroAdjudicator.IMultiAssetHolderReclaimArgs{
		SourceChannelId:       ledgerChannel,
		FixedPart:             convertedLedgerFixedPart,
		VariablePart:          convertedLedgerVariablePart,
		SourceOutcomeBytes:    sourceOb,
		SourceAssetIndex:      common.Big0,
		IndexOfTargetInSource: common.Big2,
		TargetStateHash:       virtualStateHash,
		TargetOutcomeBytes:    targetOb,
		TargetAssetIndex:      common.Big0,
	}

	reclaimTx := protocols.NewReclaimTransaction(ledgerChannel, reclaimArgs)
	_, err = chainServiceB.SendTransaction(reclaimTx)
	if err != nil {
		t.Error(err)
	}

	// Listen for reclaimed event
	event = waitForEvent(t, testChainService.EventEngineFeed(), chainservice.ReclaimedEvent{})
	_, ok = event.(chainservice.ReclaimedEvent)
	testhelpers.Assert(t, ok, "Expected reclaimed event")

	// Compute new state outcome allocations
	aliceOutcomeAllocationAmount := signedLedgerState.State().Outcome[0].Allocations[0].Amount
	bobOutcomeAllocationAmount := signedLedgerState.State().Outcome[0].Allocations[1].Amount

	aliceOutcomeAllocationAmount.Add(aliceOutcomeAllocationAmount, signedVirtualState.State().Outcome[0].Allocations[0].Amount)
	bobOutcomeAllocationAmount.Add(bobOutcomeAllocationAmount, signedVirtualState.State().Outcome[0].Allocations[1].Amount)

	// Get latest ledger channel state
	latestLedgerState := getLatestSignedState(storeA, ledgerChannel)
	latestState := latestLedgerState.State()

	// Construct exit state with updated outcome allocations
	latestState.Outcome[0].Allocations = outcome.Allocations{
		{
			Destination:    latestLedgerState.State().Outcome[0].Allocations[0].Destination,
			Amount:         aliceOutcomeAllocationAmount,
			AllocationType: outcome.SimpleAllocationType,
			Metadata:       latestLedgerState.State().Outcome[0].Allocations[0].Metadata,
		},
		{
			Destination:    latestLedgerState.State().Outcome[0].Allocations[1].Destination,
			Amount:         bobOutcomeAllocationAmount,
			AllocationType: outcome.SimpleAllocationType,
			Metadata:       latestLedgerState.State().Outcome[0].Allocations[1].Metadata,
		},
	}

	signedConstructedState := state.NewSignedState(latestState)

	// Bob calls transferAllAssets method
	transferTx := protocols.NewTransferAllTransaction(ledgerChannel, signedConstructedState)
	_, err = chainServiceB.SendTransaction(transferTx)

	testhelpers.Assert(t, err == nil, "Expected assets liquidated")

	// Listen for allocation updated event
	event = waitForEvent(t, testChainService.EventEngineFeed(), chainservice.AllocationUpdatedEvent{})
	_, ok = event.(chainservice.AllocationUpdatedEvent)
	testhelpers.Assert(t, ok, "Expected allocation updated event")

	// Check assets are liquidated
	balanceNodeA, _ := infra.anvilChain.GetAccountBalance(tc.Participants[0].Address())
	balanceNodeB, _ := infra.anvilChain.GetAccountBalance(tc.Participants[1].Address())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)

	// Alice's balance is determined by subtracting amount paid from her ledger deposit, while Bob's balance is calculated by adding his ledger deposit to the amount received
	testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of Alice (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit-payAmount)
	testhelpers.Assert(t, balanceNodeB.Cmp(big.NewInt(ledgerChannelDeposit+payAmount)) == 0, "Balance of Bob (%v) should be equal to (%v)", balanceNodeB, ledgerChannelDeposit+payAmount)
}

func getLatestSignedState(store store.Store, id types.Destination) state.SignedState {
	consensusChannel, _ := store.GetConsensusChannelById(id)
	return consensusChannel.SupportedSignedState()
}

func listenForLedgerUpdates(ledgerUpdatesChan <-chan query.LedgerChannelInfo, listenType channel.ChannelMode) {
	for ledgerInfo := range ledgerUpdatesChan {
		if ledgerInfo.ChannelMode == listenType {
			return
		}
	}
}

func TestVirtualPaymentChannelWithObjective(t *testing.T) {
	testCase := TestCase{
		Description:       "Virtual channel test with objective",
		Chain:             AnvilChain,
		MessageService:    TestMessageService,
		ChallengeDuration: 10,
		MessageDelay:      0,
		LogName:           "Virtual_channel_test_with_objective",
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
		},
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infra := setupSharedInfra(testCase)
	defer infra.Close(t)

	// Create go-nitro nodes
	nodeA, _, _, _, _ := setupIntegrationNode(testCase, testCase.Participants[0], infra, []string{}, dataFolder)
	defer nodeA.Close()
	nodeB, _, _, _, _ := setupIntegrationNode(testCase, testCase.Participants[1], infra, []string{}, dataFolder)
	defer nodeB.Close()

	// Create ledger channel and virtual fund
	// TODO: Custom tokens are not being used since reclaim method does not support them
	ledgerChannel := openLedgerChannel(t, nodeB, nodeA, types.Address{}, uint32(testCase.ChallengeDuration))
	// Check balance of node
	balanceNodeA, _ := infra.anvilChain.GetAccountBalance(testCase.Participants[0].Address())
	balanceNodeB, _ := infra.anvilChain.GetAccountBalance(testCase.Participants[1].Address())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	testhelpers.Assert(t, balanceNodeA.Int64() == 0, "Balance of Alice should be zero")
	testhelpers.Assert(t, balanceNodeB.Int64() == 0, "Balance of Bob should be zero")

	virtualOutcome := initialPaymentOutcome(*nodeB.Address, *nodeA.Address, common.BigToAddress(common.Big0))
	response, err := nodeB.CreatePaymentChannel([]common.Address{}, *nodeA.Address, uint32(testCase.ChallengeDuration), virtualOutcome)
	if err != nil {
		t.Error(err)
	}
	waitForObjectives(t, nodeA, nodeB, []node.Node{}, []protocols.ObjectiveId{response.Id})

	paymentAmount := 2000
	err = nodeB.Pay(response.ChannelId, big.NewInt(int64(paymentAmount)))
	if err != nil {
		t.Fatal(err)
	}

	nodeAVoucher := <-nodeA.ReceivedVouchers()
	t.Log("voucher recieved  for channel", nodeAVoucher.ChannelId)

	// Alice initiates the challenge transaction
	ledgerResponse, err := nodeA.CloseLedgerChannel(ledgerChannel, true)
	if err != nil {
		t.Log(err)
	}

	// Wait for objectives to complete
	chA := nodeA.ObjectiveCompleteChan(ledgerResponse)
	chB := nodeB.ObjectiveCompleteChan(ledgerResponse)
	<-chA
	<-chB

	// Check assets are liquidated
	balanceNodeA, _ = infra.anvilChain.GetAccountBalance(testCase.Participants[0].Address())
	balanceNodeB, _ = infra.anvilChain.GetAccountBalance(testCase.Participants[1].Address())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(int64(ledgerChannelDeposit+paymentAmount))) == 0, "Balance of Alice (%v) should be equal to ledgerChannelDeposit (%v)", balanceNodeA, ledgerChannelDeposit+paymentAmount)
	testhelpers.Assert(t, balanceNodeB.Cmp(big.NewInt(int64(ledgerChannelDeposit-paymentAmount))) == 0, "Balance of Bob (%v) should be equal to ledgerChannelDeposit (%v)", balanceNodeB, ledgerChannelDeposit-paymentAmount)
}

func GetEthereumAddress(chainPk string) types.Address {
	return crypto.GetAddressFromSecretKeyBytes(common.Hex2Bytes(chainPk))
}
