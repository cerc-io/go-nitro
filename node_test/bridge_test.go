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
	"github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	NitroAdjudicator "github.com/statechannels/go-nitro/node/engine/chainservice/adjudicator"
	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

func TestExitL2WithLedgerChannelState(t *testing.T) {
	tcL1 := TestCase{
		Chain:             AnvilChainL1,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
		},
		deployerIndex: 1,
	}

	tcL2 := TestCase{
		Chain:             AnvilChainL2,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Bob},
			{StoreType: MemStore, Actor: testactors.Alice},
		},
		ChainPort:     "8546",
		deployerIndex: 0,
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infraL1 := setupSharedInfra(tcL1)
	defer infraL1.Close(t)

	infraL2 := setupSharedInfra(tcL2)
	defer infraL2.Close(t)

	// Create go-nitro nodes
	nodeA, _, _, storeA, chainServiceA := setupIntegrationNode(tcL1, tcL1.Participants[0], infraL1, []string{}, dataFolder)
	defer nodeA.Close()

	nodeB, _, _, _, chainServiceB := setupIntegrationNode(tcL1, tcL1.Participants[1], infraL1, []string{}, dataFolder)
	defer nodeB.Close()

	nodeBPrime, _, _, storeBPrime, _ := setupIntegrationNode(tcL2, tcL2.Participants[0], infraL2, []string{}, dataFolder)
	defer nodeBPrime.Close()

	nodeAPrime, _, _, _, _ := setupIntegrationNode(tcL2, tcL2.Participants[1], infraL2, []string{}, dataFolder)
	defer nodeAPrime.Close()

	// Seperate chain service to listen for events
	testChainService := setupChainService(tcL1, tcL1.Participants[0], infraL1)
	defer testChainService.Close()

	mirroredLedgerChannelId := types.Destination{}
	l1ChannelId := types.Destination{}

	t.Run("Create ledger channel on L1 and mirror it on L2", func(t *testing.T) {
		l1ChannelId, mirroredLedgerChannelId = createL1L2Channels(t, nodeA, nodeB, nodeAPrime, nodeBPrime, storeA, tcL1, tcL2, chainServiceB)
		ledgerChannelInfo, err := nodeBPrime.GetLedgerChannel(mirroredLedgerChannelId)
		if err != nil {
			t.Error(err)
		}

		balanceNodeAPrime := ledgerChannelInfo.Balance.TheirBalance.ToInt()
		balanceNodeBPrime := ledgerChannelInfo.Balance.MyBalance.ToInt()
		t.Log("Balance of node BPrime", balanceNodeBPrime, "\nBalance of node APrime", balanceNodeAPrime)

		// BPrime's and APrime's balance should be equal to ledgerChannelDeposit since no payments happened
		testhelpers.Assert(t, balanceNodeBPrime.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "Balance of node BPrime (%v) should be equal to (%v)", balanceNodeBPrime, ledgerChannelDeposit)
		testhelpers.Assert(t, balanceNodeAPrime.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "Balance of node APrime (%v) should be equal to (%v)", balanceNodeAPrime, ledgerChannelDeposit)
	})

	t.Run("Exit to L1 using L2 ledger channel state", func(t *testing.T) {
		l2SignedState := getLatestSignedState(storeBPrime, mirroredLedgerChannelId)
		l2StateClone := l2SignedState.State().Clone()

		// Both participants on L2 ledger channel sign state where `isFinal = true` which is required for a channel to conclude and finalize
		l2StateClone.IsFinal = true

		Asig, _ := l2StateClone.Sign(tcL2.Participants[1].PrivateKey)
		Bsig, _ := l2StateClone.Sign(tcL2.Participants[0].PrivateKey)

		l2SignedStateClone := state.NewSignedState(l2StateClone)

		_ = l2SignedStateClone.AddSignature(Asig)
		_ = l2SignedStateClone.AddSignature(Bsig)

		// Node A calls modified `concludeAndTransferAllAssets` method to exit to L1 using L2 ledger channel state
		MirrorWithdrawAllTx := protocols.NewMirrorWithdrawAllTransaction(l1ChannelId, l2SignedStateClone)
		err := chainServiceA.SendTransaction(MirrorWithdrawAllTx)
		if err != nil {
			t.Error(err)
		}

		// Listen for allocation updated event
		event := waitForEvent(t, testChainService.EventFeed(), chainservice.AllocationUpdatedEvent{})
		_, ok := event.(chainservice.AllocationUpdatedEvent)
		testhelpers.Assert(t, ok, "Expected allocation updated event")

		balanceNodeA, _ := infraL1.anvilChain.GetAccountBalance(tcL1.Participants[0].Address())
		balanceNodeB, _ := infraL1.anvilChain.GetAccountBalance(tcL1.Participants[1].Address())
		t.Log("Balance of node A", balanceNodeA, "\nBalance of node B", balanceNodeB)

		// Node A's and node B's balance should be equal to ledgerChannelDeposit since no payments happened
		testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "Balance of node A (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit)
		testhelpers.Assert(t, balanceNodeB.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "Balance of node B (%v) should be equal to (%v)", balanceNodeB, ledgerChannelDeposit)
	})
}

func TestExitL2WithPayments(t *testing.T) {
	const payAmount = 2000

	tcL1 := TestCase{
		Chain:             AnvilChainL1,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
		},
		deployerIndex: 1,
	}

	tcL2 := TestCase{
		Chain:             AnvilChainL2,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Bob},
			{StoreType: MemStore, Actor: testactors.Alice},
		},
		deployerIndex: 0,
		ChainPort:     "8546",
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infraL1 := setupSharedInfra(tcL1)
	defer infraL1.Close(t)

	infraL2 := setupSharedInfra(tcL2)
	defer infraL2.Close(t)

	// Create go-nitro nodes
	nodeA, _, _, storeA, chainServiceA := setupIntegrationNode(tcL1, tcL1.Participants[0], infraL1, []string{}, dataFolder)
	defer nodeA.Close()

	nodeB, _, _, _, chainServiceB := setupIntegrationNode(tcL1, tcL1.Participants[1], infraL1, []string{}, dataFolder)
	defer nodeB.Close()

	nodeBPrime, _, _, storeBPrime, _ := setupIntegrationNode(tcL2, tcL2.Participants[0], infraL2, []string{}, dataFolder)
	defer nodeBPrime.Close()

	nodeAPrime, _, _, _, _ := setupIntegrationNode(tcL2, tcL2.Participants[1], infraL2, []string{}, dataFolder)
	defer nodeAPrime.Close()

	// Seperate chain service to listen for events
	testChainService := setupChainService(tcL1, tcL1.Participants[0], infraL1)
	defer testChainService.Close()

	l2ChannelSignedState := state.SignedState{}

	// Create ledger channel on L1 and mirror it on L2
	l1ChannelId, mirroredLedgerChannelId := createL1L2Channels(t, nodeA, nodeB, nodeAPrime, nodeBPrime, storeA, tcL1, tcL2, chainServiceB)

	t.Run("Create virtual channel on mirrored ledger channel and make payments", func(t *testing.T) {
		virtualChannel := createL2VirtualChannel(t, nodeAPrime, nodeBPrime, storeBPrime, tcL2)

		// Bridge pays APrime
		nodeBPrime.Pay(virtualChannel.Id, big.NewInt(payAmount))

		// Wait for APrime to recieve voucher
		nodeAPrimeVoucher := <-nodeAPrime.ReceivedVouchers()
		t.Logf("Voucher recieved %+v", nodeAPrimeVoucher)

		// Virtual defund
		virtualDefundResponse, _ := nodeBPrime.ClosePaymentChannel(virtualChannel.Id)
		waitForObjectives(t, nodeBPrime, nodeAPrime, []node.Node{}, []protocols.ObjectiveId{virtualDefundResponse})

		ledgerChannelInfo, err := nodeBPrime.GetLedgerChannel(mirroredLedgerChannelId)
		if err != nil {
			t.Error(err)
		}
		balanceNodeAPrime := ledgerChannelInfo.Balance.TheirBalance.ToInt()
		balanceNodeBPrime := ledgerChannelInfo.Balance.MyBalance.ToInt()
		t.Log("Balance of node BPrime", balanceNodeBPrime, "\nBalance of node APrime", balanceNodeAPrime)

		// BPrime's balance is determined by subtracting amount paid from it's ledger deposit, while APrime's balance is calculated by adding it's ledger deposit to the amount received
		testhelpers.Assert(t, balanceNodeBPrime.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of node BPrime (%v) should be equal to (%v)", balanceNodeBPrime, ledgerChannelDeposit-payAmount)
		testhelpers.Assert(t, balanceNodeAPrime.Cmp(big.NewInt(ledgerChannelDeposit+payAmount)) == 0, "Balance of node APrime (%v) should be equal to (%v)", balanceNodeAPrime, ledgerChannelDeposit+payAmount)
	})

	t.Run("Exit to L1 using updated L2 ledger channel state after making payments", func(t *testing.T) {
		l2SignedState := getLatestSignedState(storeBPrime, mirroredLedgerChannelId)
		l2StateClone := l2SignedState.State().Clone()

		// Both participants on L2 ledger channel sign state where `isFinal = true` which is required for a channel to conclude and finalize
		l2StateClone.IsFinal = true

		Asig, _ := l2StateClone.Sign(tcL2.Participants[1].PrivateKey)
		Bsig, _ := l2StateClone.Sign(tcL2.Participants[0].PrivateKey)

		l2SignedStateClone := state.NewSignedState(l2StateClone)

		_ = l2SignedStateClone.AddSignature(Asig)
		_ = l2SignedStateClone.AddSignature(Bsig)

		l2ChannelSignedState = l2SignedStateClone

		// Node A calls modified `concludeAndTransferAllAssets` method to exit to L1 using L2 ledger channel state
		MirrorWithdrawAllTx := protocols.NewMirrorWithdrawAllTransaction(l1ChannelId, l2ChannelSignedState)
		err := chainServiceA.SendTransaction(MirrorWithdrawAllTx)
		if err != nil {
			t.Error(err)
		}

		// Listen for allocation updated event
		event := waitForEvent(t, testChainService.EventFeed(), chainservice.AllocationUpdatedEvent{})
		_, ok := event.(chainservice.AllocationUpdatedEvent)
		testhelpers.Assert(t, ok, "Expected allocation updated event")

		balanceNodeA, _ := infraL1.anvilChain.GetAccountBalance(tcL1.Participants[0].Address())
		balanceNodeB, _ := infraL1.anvilChain.GetAccountBalance(tcL1.Participants[1].Address())
		t.Log("Balance of node A", balanceNodeA, "\nBalance of node B", balanceNodeB)

		testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit+payAmount)) == 0, "Balance of node A (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit+payAmount)
		testhelpers.Assert(t, balanceNodeB.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of node B (%v) should be equal to (%v)", balanceNodeB, ledgerChannelDeposit-payAmount)
	})
}

func TestExitL2WithLedgerChannelStateUnilaterally(t *testing.T) {
	const payAmount = 2000

	tcL1 := TestCase{
		Chain:             AnvilChainL1,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
		},
		deployerIndex: 1,
	}

	tcL2 := TestCase{
		Chain:             AnvilChainL2,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Bob},
			{StoreType: MemStore, Actor: testactors.Alice},
		},
		ChainPort:     "8546",
		deployerIndex: 0,
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infraL1 := setupSharedInfra(tcL1)
	defer infraL1.Close(t)

	infraL2 := setupSharedInfra(tcL2)
	defer infraL2.Close(t)

	// Create go-nitro nodes
	nodeA, _, _, storeA, chainServiceA := setupIntegrationNode(tcL1, tcL1.Participants[0], infraL1, []string{}, dataFolder)
	defer nodeA.Close()

	nodeB, _, _, _, chainServiceB := setupIntegrationNode(tcL1, tcL1.Participants[1], infraL1, []string{}, dataFolder)

	infraL2.anvilChain.ContractAddresses.CaAddress = infraL1.anvilChain.ContractAddresses.CaAddress
	infraL2.anvilChain.ContractAddresses.VpaAddress = infraL1.anvilChain.ContractAddresses.VpaAddress

	nodeBPrime, _, _, storeBPrime, _ := setupIntegrationNode(tcL2, tcL2.Participants[0], infraL2, []string{}, dataFolder)

	nodeAPrime, _, _, storeAPrime, _ := setupIntegrationNode(tcL2, tcL2.Participants[1], infraL2, []string{}, dataFolder)
	defer nodeAPrime.Close()

	// Separate chain service to listen for events
	testChainService := setupChainService(tcL1, tcL1.Participants[0], infraL1)
	defer testChainService.Close()

	// Create ledger channel on L1 and mirror it on L2
	l1ChannelId, mirroredLedgerChannelId := createL1L2Channels(t, nodeA, nodeB, nodeAPrime, nodeBPrime, storeA, tcL1, tcL2, chainServiceB)

	// Create virtual channel on mirrored ledger channel and make payments
	virtualChannel := createL2VirtualChannel(t, nodeAPrime, nodeBPrime, storeBPrime, tcL2)

	// Bridge pays APrime
	nodeBPrime.Pay(virtualChannel.Id, big.NewInt(payAmount))

	// Wait for APrime to recieve voucher
	nodeAPrimeVoucher := <-nodeAPrime.ReceivedVouchers()
	t.Logf("Voucher recieved %+v", nodeAPrimeVoucher)

	// Virtual defund
	virtualDefundResponse, _ := nodeBPrime.ClosePaymentChannel(virtualChannel.Id)
	waitForObjectives(t, nodeBPrime, nodeAPrime, []node.Node{}, []protocols.ObjectiveId{virtualDefundResponse})

	t.Run("Exit to L1 using L2 ledger channel state unilaterally", func(t *testing.T) {
		l2SignedState := getLatestSignedState(storeAPrime, mirroredLedgerChannelId)

		// Close bridge nodes
		nodeB.Close()
		nodeBPrime.Close()

		// Node A calls `challenge` contract method with L2 ledger channel state
		challengerSig, _ := NitroAdjudicator.SignChallengeMessage(l2SignedState.State(), tcL1.Participants[0].PrivateKey)
		challengeTx := protocols.NewChallengeTransaction(l1ChannelId, l2SignedState, []state.SignedState{}, challengerSig)
		err := chainServiceA.SendTransaction(challengeTx)
		if err != nil {
			t.Error(err)
		}

		event := waitForEvent(t, testChainService.EventFeed(), chainservice.ChallengeRegisteredEvent{})
		t.Log("Challenge registed event received", event)
		challengeRegisteredEvent, ok := event.(chainservice.ChallengeRegisteredEvent)
		testhelpers.Assert(t, ok, "Expected challenge registered event")

		time.Sleep(time.Duration(tcL1.ChallengeDuration) * time.Second)
		latestBlock, _ := infraL1.anvilChain.GetLatestBlock()
		testhelpers.Assert(t, challengeRegisteredEvent.FinalizesAt.Uint64() <= latestBlock.Header().Time, "Expected channel to be finalized")

		l2ChannelSignedState := getLatestSignedState(storeAPrime, mirroredLedgerChannelId)

		mirrorTransferAllTx := protocols.NewMirrorTransferAllTransaction(l1ChannelId, l2ChannelSignedState)
		err = chainServiceA.SendTransaction(mirrorTransferAllTx)
		if err != nil {
			t.Error(err)
		}

		// Listen for allocation updated event
		event = waitForEvent(t, testChainService.EventFeed(), chainservice.AllocationUpdatedEvent{})
		_, ok = event.(chainservice.AllocationUpdatedEvent)
		testhelpers.Assert(t, ok, "Expected allocation updated event")

		balanceNodeA, _ := infraL1.anvilChain.GetAccountBalance(tcL1.Participants[0].Address())
		balanceNodeB, _ := infraL1.anvilChain.GetAccountBalance(tcL1.Participants[1].Address())
		t.Log("Balance of node A", balanceNodeA, "\nBalance of node B", balanceNodeB)

		// Node A's and node B's balance should be equal to ledgerChannelDeposit since no payments happened
		testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit+payAmount)) == 0, "Balance of node A (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit+payAmount)
		testhelpers.Assert(t, balanceNodeB.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of node B (%v) should be equal to (%v)", balanceNodeB, ledgerChannelDeposit-payAmount)
	})
}

func TestExitL2WithVirtualChannelStateUnilaterally(t *testing.T) {
	const payAmount = 2000

	tcL1 := TestCase{
		Chain:             AnvilChainL1,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
		},
		deployerIndex: 1,
	}

	tcL2 := TestCase{
		Chain:             AnvilChainL2,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Bob},
			{StoreType: MemStore, Actor: testactors.Alice},
		},
		ChainPort:     "8546",
		deployerIndex: 0,
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infraL1 := setupSharedInfra(tcL1)
	defer infraL1.Close(t)

	infraL2 := setupSharedInfra(tcL2)
	defer infraL2.Close(t)

	// Create go-nitro nodes
	nodeA, _, _, storeA, chainServiceA := setupIntegrationNode(tcL1, tcL1.Participants[0], infraL1, []string{}, dataFolder)
	defer nodeA.Close()

	nodeB, _, _, _, chainServiceB := setupIntegrationNode(tcL1, tcL1.Participants[1], infraL1, []string{}, dataFolder)

	infraL2.anvilChain.ContractAddresses.CaAddress = infraL1.anvilChain.ContractAddresses.CaAddress
	infraL2.anvilChain.ContractAddresses.VpaAddress = infraL1.anvilChain.ContractAddresses.VpaAddress

	nodeBPrime, _, _, storeBPrime, _ := setupIntegrationNode(tcL2, tcL2.Participants[0], infraL2, []string{}, dataFolder)

	nodeAPrime, _, _, storeAPrime, _ := setupIntegrationNode(tcL2, tcL2.Participants[1], infraL2, []string{}, dataFolder)
	defer nodeAPrime.Close()

	// Separate chain service to listen for events
	testChainService := setupChainService(tcL1, tcL1.Participants[0], infraL1)
	defer testChainService.Close()

	l1ChannelId, mirroredLedgerChannelId := createL1L2Channels(t, nodeA, nodeB, nodeAPrime, nodeBPrime, storeA, tcL1, tcL2, chainServiceB)

	// Create virtual channel on mirrored ledger channel on L2 and make payments
	virtualChannel := createL2VirtualChannel(t, nodeAPrime, nodeBPrime, storeBPrime, tcL2)

	// Bridge pays APrime
	nodeBPrime.Pay(virtualChannel.Id, big.NewInt(payAmount))

	// Wait for APrime to recieve voucher
	nodeAPrimeVoucher := <-nodeAPrime.ReceivedVouchers()
	t.Logf("Voucher recieved %+v", nodeAPrimeVoucher)

	virtualChannelId := virtualChannel.Id
	nodeAPrimeVirtualPaymentVoucher := nodeAPrimeVoucher

	t.Run("Exit to L1 from L2 virtual channel state unilaterally", func(t *testing.T) {
		// Close bridge nodes
		nodeB.Close()
		nodeBPrime.Close()

		virtualChannel, _ := storeAPrime.GetChannelById(virtualChannelId)
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
			Amount:    nodeAPrimeVirtualPaymentVoucher.Amount,
			Signature: NitroAdjudicator.ConvertSignature(nodeAPrimeVirtualPaymentVoucher.Signature),
		}

		// Use above created type and encode voucher amount and signature
		dataEncoded, err := arguments.Pack(voucherAmountSignatureData)
		if err != nil {
			t.Fatalf("Failed to encode data: %v", err)
		}

		// Create expected payment outcome
		finalVirtualOutcome := finalPaymentOutcome(*nodeA.Address, *nodeB.Address, common.Address{}, 1, uint(nodeAPrimeVirtualPaymentVoucher.Amount.Int64()))

		// Construct variable part with updated outcome and app data
		vp := state.VariablePart{Outcome: finalVirtualOutcome, TurnNum: voucherState.State().TurnNum + 1, AppData: dataEncoded, IsFinal: voucherState.State().IsFinal}

		// Update state with constructed variable part
		newState := state.StateFromFixedAndVariablePart(voucherState.State().FixedPart(), vp)

		// APrime signs constructed state and adds it to the virtual channel
		_, _ = virtualChannel.SignAndAddState(newState, &tcL2.Participants[1].PrivateKey)

		// Update store with updated virtual channel
		_ = storeAPrime.SetChannel(virtualChannel)

		// Get updated virtual channel
		updatedVirtualChannel, _ := storeAPrime.GetChannelById(virtualChannelId)
		signedVirtualState, _ := updatedVirtualChannel.LatestSignedState()
		signedPostFundState := updatedVirtualChannel.SignedPostFundState()

		// Node A calls modified `challenge` with L2 virtual channel state
		virtualChallengerSig, _ := NitroAdjudicator.SignChallengeMessage(signedVirtualState.State(), tcL1.Participants[0].PrivateKey)
		mirrroVirtualChallengeTx := protocols.NewChallengeTransaction(virtualChannelId, signedVirtualState, []state.SignedState{signedPostFundState}, virtualChallengerSig)
		err = chainServiceA.SendTransaction(mirrroVirtualChallengeTx)
		if err != nil {
			t.Error(err)
		}

		// Listen for challenge registered event
		event := waitForEvent(t, testChainService.EventFeed(), chainservice.ChallengeRegisteredEvent{})
		t.Log("Challenge registed event received", event)
		challengeRegisteredEvent, ok := event.(chainservice.ChallengeRegisteredEvent)
		testhelpers.Assert(t, ok, "Expected challenge registered event")

		time.Sleep(time.Duration(tcL2.ChallengeDuration) * time.Second)
		latestBlock, _ := infraL1.anvilChain.GetLatestBlock()
		testhelpers.Assert(t, challengeRegisteredEvent.FinalizesAt.Uint64() <= latestBlock.Header().Time, "Expected channel to be finalized")

		l2SignedState := getLatestSignedState(storeAPrime, mirroredLedgerChannelId)

		// Node A calls modified `challenge` with L2 ledger channel state
		challengerSig, _ := NitroAdjudicator.SignChallengeMessage(l2SignedState.State(), tcL1.Participants[0].PrivateKey)
		challengeTx := protocols.NewChallengeTransaction(l1ChannelId, l2SignedState, []state.SignedState{}, challengerSig)
		err = chainServiceA.SendTransaction(challengeTx)
		if err != nil {
			t.Error(err)
		}

		event = waitForEvent(t, testChainService.EventFeed(), chainservice.ChallengeRegisteredEvent{})
		t.Log("Challenge registed event received", event)
		challengeRegisteredEvent, ok = event.(chainservice.ChallengeRegisteredEvent)
		testhelpers.Assert(t, ok, "Expected challenge registered event")

		time.Sleep(time.Duration(tcL1.ChallengeDuration) * time.Second)
		latestBlock, _ = infraL1.anvilChain.GetLatestBlock()
		testhelpers.Assert(t, challengeRegisteredEvent.FinalizesAt.Uint64() <= latestBlock.Header().Time, "Expected channel to be finalized")

		l2SignedState = getLatestSignedState(storeAPrime, mirroredLedgerChannelId)
		updatedVirtualChannel, _ = storeAPrime.GetChannelById(virtualChannelId)
		signedVirtualState, _ = updatedVirtualChannel.LatestSignedState()

		// Now that ledger and virtual channels are finalized, call modified `reclaim` method
		convertedLedgerFixedPart := NitroAdjudicator.ConvertFixedPart(l2SignedState.State().FixedPart())
		convertedLedgerVariablePart := NitroAdjudicator.ConvertVariablePart(l2SignedState.State().VariablePart())
		virtualStateHash, _ := signedVirtualState.State().Hash()
		sourceOutcome := l2SignedState.State().Outcome
		sourceOb, _ := sourceOutcome.Encode()
		targetOutcome := signedVirtualState.State().Outcome
		targetOb, _ := targetOutcome.Encode()

		reclaimArgs := NitroAdjudicator.IMultiAssetHolderReclaimArgs{
			SourceChannelId:       mirroredLedgerChannelId,
			FixedPart:             convertedLedgerFixedPart,
			VariablePart:          convertedLedgerVariablePart,
			SourceOutcomeBytes:    sourceOb,
			SourceAssetIndex:      common.Big0,
			IndexOfTargetInSource: common.Big2,
			TargetStateHash:       virtualStateHash,
			TargetOutcomeBytes:    targetOb,
			TargetAssetIndex:      common.Big0,
		}

		reclaimTx := protocols.NewReclaimTransaction(l1ChannelId, reclaimArgs)
		err = chainServiceA.SendTransaction(reclaimTx)
		if err != nil {
			t.Error(err)
		}

		// Listen for reclaimed event
		event = waitForEvent(t, testChainService.EventFeed(), chainservice.ReclaimedEvent{})
		_, ok = event.(chainservice.ReclaimedEvent)
		testhelpers.Assert(t, ok, "Expected reclaimed event")

		l2SignedState = getLatestSignedState(storeAPrime, mirroredLedgerChannelId)

		// Compute new state outcome allocations
		aliceOutcomeAllocationAmount := l2SignedState.State().Outcome[0].Allocations[1].Amount
		bobOutcomeAllocationAmount := l2SignedState.State().Outcome[0].Allocations[0].Amount

		aliceOutcomeAllocationAmount.Add(aliceOutcomeAllocationAmount, signedVirtualState.State().Outcome[0].Allocations[1].Amount)
		bobOutcomeAllocationAmount.Add(bobOutcomeAllocationAmount, signedVirtualState.State().Outcome[0].Allocations[0].Amount)

		// Get latest ledger channel state
		latestState := l2SignedState.State()

		// Construct exit state with updated outcome allocations
		latestState.Outcome[0].Allocations = outcome.Allocations{
			{
				Destination:    l2SignedState.State().Outcome[0].Allocations[0].Destination,
				Amount:         bobOutcomeAllocationAmount,
				AllocationType: outcome.SimpleAllocationType,
				Metadata:       l2SignedState.State().Outcome[0].Allocations[0].Metadata,
			},
			{
				Destination:    l2SignedState.State().Outcome[0].Allocations[1].Destination,
				Amount:         aliceOutcomeAllocationAmount,
				AllocationType: outcome.SimpleAllocationType,
				Metadata:       l2SignedState.State().Outcome[0].Allocations[1].Metadata,
			},
		}

		signedConstructedState := state.NewSignedState(latestState)

		mirrorTransferAllTx := protocols.NewMirrorTransferAllTransaction(l1ChannelId, signedConstructedState)
		err = chainServiceA.SendTransaction(mirrorTransferAllTx)
		if err != nil {
			t.Error(err)
		}

		// Listen for allocation updated event
		event = waitForEvent(t, testChainService.EventFeed(), chainservice.AllocationUpdatedEvent{})
		_, ok = event.(chainservice.AllocationUpdatedEvent)
		testhelpers.Assert(t, ok, "Expected allocation updated event")

		balanceNodeA, _ := infraL1.anvilChain.GetAccountBalance(tcL1.Participants[0].Address())
		balanceNodeB, _ := infraL1.anvilChain.GetAccountBalance(tcL1.Participants[1].Address())
		t.Log("Balance of node A", balanceNodeA, "\nBalance of node B", balanceNodeB)

		testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit+payAmount)) == 0, "Balance of node A (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit+payAmount)
		testhelpers.Assert(t, balanceNodeB.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of node B (%v) should be equal to (%v)", balanceNodeB, ledgerChannelDeposit-payAmount)
	})
}

func createL1L2Channels(t *testing.T, nodeA node.Node, nodeB node.Node, nodeAPrime node.Node, nodeBPrime node.Node, nodeStore store.Store, tcL1 TestCase, tcL2 TestCase, bridgeChainService chainservice.ChainService) (types.Destination, types.Destination) {
	// Create ledger channel
	l1LedgerChannelId := openLedgerChannel(t, nodeA, nodeB, types.Address{}, uint32(tcL1.ChallengeDuration))

	l1LedgerChannel, err := nodeStore.GetConsensusChannelById(l1LedgerChannelId)
	if err != nil {
		t.Error(err)
	}

	l1ledgerChannelState := l1LedgerChannel.SupportedSignedState()
	l1ledgerChannelStateClone := l1ledgerChannelState.Clone()

	// Put NodeBPrime's allocation at index 0 as it creates mirrored ledger channel
	// Swap the allocations to be set in mirrored ledger channel
	tempAllocation := l1ledgerChannelStateClone.State().Outcome[0].Allocations[0]
	l1ledgerChannelStateClone.State().Outcome[0].Allocations[0] = l1ledgerChannelStateClone.State().Outcome[0].Allocations[1]
	l1ledgerChannelStateClone.State().Outcome[0].Allocations[1] = tempAllocation

	// Create extended state outcome based on l1ChannelState
	l2ChannelOutcome := l1ledgerChannelStateClone.State().Outcome

	// Create mirrored ledger channel between node BPrime and APrime
	response, err := nodeBPrime.CreateBridgeChannel(*nodeAPrime.Address, uint32(tcL2.ChallengeDuration), l2ChannelOutcome)
	if err != nil {
		t.Error(err)
	}

	nodeBPrimeChannel := nodeBPrime.ObjectiveCompleteChan(response.Id)
	nodeAPrimeChannel := nodeAPrime.ObjectiveCompleteChan(response.Id)
	t.Log("Waiting for bridge-fund objective to complete...")
	<-nodeBPrimeChannel
	<-nodeAPrimeChannel
	t.Log("Completed bridge-fund objective")

	// Node B calls contract method to store L2ChannelId => L1ChannelId
	setL2ToL1Tx := protocols.NewSetL2ToL1Transaction(l1LedgerChannelId, response.ChannelId)
	err = bridgeChainService.SendTransaction(setL2ToL1Tx)
	if err != nil {
		t.Error(err)
	}

	return l1LedgerChannelId, response.ChannelId
}

func createL2VirtualChannel(t *testing.T, nodeAPrime node.Node, nodeBPrime node.Node, L2bridgeStore store.Store, tcL2 TestCase) *channel.Channel {
	// Create virtual channel on mirrored ledger channel on L2 and make payments
	virtualOutcome := initialPaymentOutcome(*nodeBPrime.Address, *nodeAPrime.Address, types.Address{})

	virtualResponse, _ := nodeBPrime.CreatePaymentChannel([]types.Address{}, *nodeAPrime.Address, uint32(tcL2.ChallengeDuration), virtualOutcome)
	waitForObjectives(t, nodeBPrime, nodeAPrime, []node.Node{}, []protocols.ObjectiveId{virtualResponse.Id})

	checkPaymentChannel(t, virtualResponse.ChannelId, virtualOutcome, query.Open, nodeBPrime, nodeAPrime)

	virtualChannel, _ := L2bridgeStore.GetChannelById(virtualResponse.ChannelId)

	return virtualChannel
}
