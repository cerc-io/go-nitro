package node_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/bridge"
	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	NitroAdjudicator "github.com/statechannels/go-nitro/node/engine/chainservice/adjudicator"
	Token "github.com/statechannels/go-nitro/node/engine/chainservice/erc20"

	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/mirrorbridgeddefund"
	"github.com/statechannels/go-nitro/types"
)

type Utils struct {
	tcL1, tcL2                   TestCase
	nodeA, nodeB                 node.Node
	nodeAPrime, nodeBPrime       node.Node
	chainServiceA, chainServiceB chainservice.ChainService
	testChainService             chainservice.ChainService
	storeA, storeB               store.Store
	storeAPrime, storeBPrime     store.Store
	infraL1, infraL2             sharedTestInfrastructure
}

type UtilsWithBridge struct {
	tcL1, tcL2                                 TestCase
	bridge                                     *bridge.Bridge
	bridgeAddress                              common.Address
	bridgeMultiaddressL1, bridgeMultiaddressL2 string
	dataFolder                                 string
	nodeA, nodeAPrime                          node.Node
	chainServiceA, chainServiceAPrime          chainservice.ChainService
	storeA, storeAPrime                        store.Store
	infraL1, infraL2                           sharedTestInfrastructure
	l1TokenBinding                             *Token.Token
}

func TestBridgedFund(t *testing.T) {
	utils, cleanup := initializeUtilsWithBridge(t, true)
	defer cleanup()

	tcL1, tcL2 := utils.tcL1, utils.tcL2
	nodeA, nodeAPrime := utils.nodeA, utils.nodeAPrime
	bridge, bridgeAddress := utils.bridge, utils.bridgeAddress
	storeA := utils.storeA
	infraL1 := utils.infraL1

	var l1LedgerChannelId types.Destination
	var l2LedgerChannelId types.Destination

	t.Run("Create ledger channel on L1 and mirror it on L2", func(t *testing.T) {
		// Alice create ledger channel with bridge
		outcome := CreateLedgerOutcome(*nodeA.Address, bridgeAddress, ledgerChannelDeposit, 0, infraL1.anvilChain.ContractAddresses.TokenAddress)
		l1LedgerChannelResponse, err := nodeA.CreateLedgerChannel(bridgeAddress, uint32(tcL1.ChallengeDuration), outcome)
		if err != nil {
			t.Fatal(err)
		}
		t.Log("Waiting for direct-fund objective to complete...")
		l1LedgerChannelId = l1LedgerChannelResponse.ChannelId
		<-nodeA.ObjectiveCompleteChan(l1LedgerChannelResponse.Id)
		t.Log("L1 channel created", l1LedgerChannelResponse.Id)

		// Wait for mirror channel to be created
		createdMirrorChannel := <-bridge.CreatedMirrorChannels()
		l2LedgerChannelId, _ = bridge.GetL2ChannelIdByL1ChannelId(l1LedgerChannelResponse.ChannelId)
		testhelpers.Assert(t, createdMirrorChannel == l2LedgerChannelId, "Expects mirror channel id to be %v", l2LedgerChannelId)
		checkLedgerChannel(t, l1LedgerChannelResponse.ChannelId, CreateLedgerOutcome(*nodeA.Address, bridgeAddress, ledgerChannelDeposit, 0, infraL1.anvilChain.ContractAddresses.TokenAddress), query.Open, nodeA)
		checkLedgerChannel(t, l2LedgerChannelId, CreateLedgerOutcome(bridgeAddress, *nodeAPrime.Address, 0, ledgerChannelDeposit, infraL1.anvilChain.ContractAddresses.TokenAddress), query.Open, nodeAPrime)
	})

	t.Run("Create virtual channel on mirrored ledger channel and make payments", func(t *testing.T) {
		// Create virtual channel on mirrored ledger channel on L2
		virtualOutcome := initialPaymentOutcome(*nodeAPrime.Address, bridgeAddress, types.Address{})
		virtualResponse, _ := nodeAPrime.CreatePaymentChannel([]types.Address{}, bridgeAddress, uint32(tcL2.ChallengeDuration), virtualOutcome)
		<-nodeAPrime.ObjectiveCompleteChan(virtualResponse.Id)
		checkPaymentChannel(t, virtualResponse.ChannelId, virtualOutcome, query.Open, nodeAPrime)

		// APrime pays BPrime
		err := nodeAPrime.Pay(virtualResponse.ChannelId, big.NewInt(payAmount))
		if err != nil {
			t.Fatal(err)
		}

		// Virtual defund
		virtualDefundResponse, _ := nodeAPrime.ClosePaymentChannel(virtualResponse.ChannelId)
		<-nodeAPrime.ObjectiveCompleteChan(virtualDefundResponse)

		ledgerChannelInfo, _ := nodeAPrime.GetLedgerChannel(l2LedgerChannelId)
		balanceNodeBPrime := ledgerChannelInfo.Balance.TheirBalance.ToInt()
		balanceNodeAPrime := ledgerChannelInfo.Balance.MyBalance.ToInt()
		t.Log("Balance of node BPrime", balanceNodeBPrime, "\nBalance of node APrime", balanceNodeAPrime)

		// APrime's balance is determined by subtracting amount paid from it's ledger deposit, while BPrime's balance is calculated by adding the amount received
		testhelpers.Assert(t, balanceNodeBPrime.Cmp(big.NewInt(payAmount)) == 0, "Balance of node BPrime (%v) should be equal to (%v)", balanceNodeBPrime, ledgerChannelDeposit+payAmount)
		testhelpers.Assert(t, balanceNodeAPrime.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of node APrime (%v) should be equal to (%v)", balanceNodeAPrime, ledgerChannelDeposit-payAmount)
	})

	t.Run("Exit to L1 using updated L2 ledger channel state after making payments", func(t *testing.T) {
		completedObjectiveChannel := nodeA.CompletedObjectives()
		_, err := nodeAPrime.CloseBridgeChannel(l2LedgerChannelId)
		if err != nil {
			t.Fatal(err)
		}

		// Wait for mirror bridged defund to complete on L1
		for completedObjectiveId := range completedObjectiveChannel {
			if mirrorbridgeddefund.IsMirrorBridgedDefundObjective(completedObjectiveId) {
				objective, err := storeA.GetObjectiveById(completedObjectiveId)
				if err != nil {
					t.Fatal("mirror bridged defund objective not found", err)
				}

				if objective.OwnsChannel() == l1LedgerChannelId {
					break
				}
			}
		}

		checkLedgerChannel(t, l1LedgerChannelId, CreateLedgerOutcome(*nodeA.Address, bridgeAddress, ledgerChannelDeposit-payAmount, payAmount, infraL1.anvilChain.ContractAddresses.TokenAddress), query.Complete, nodeA)

		balanceNodeA, _ := utils.l1TokenBinding.BalanceOf(nil, tcL1.Participants[0].Address())
		balanceBridge, _ := utils.l1TokenBinding.BalanceOf(nil, tcL1.Participants[1].Address())
		t.Logf("Balance of node A %v \nBalance of Bridge %v", balanceNodeA, balanceBridge)

		// NodeA's balance is determined by subtracting amount paid from it's ledger deposit, while Bridge's balance is calculated by adding the amount received
		testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of node A (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit-payAmount)
		testhelpers.Assert(t, balanceBridge.Cmp(big.NewInt(payAmount)) == 0, "Balance of Bridge (%v) should be equal to (%v)", balanceBridge, payAmount)
	})
}

func TestBridgedFundWithCheckpoint(t *testing.T) {
	utils, cleanup := initializeUtilsWithBridge(t, true)
	defer cleanup()

	tcL1, tcL2 := utils.tcL1, utils.tcL2
	nodeA, nodeAPrime := utils.nodeA, utils.nodeAPrime
	bridge, bridgeAddress := utils.bridge, utils.bridgeAddress
	storeA, storeAPrime := utils.storeA, utils.storeAPrime
	infraL1 := utils.infraL1

	l1LedgerChannelId, l2LedgerChannelId := createMirrorChannel(t, nodeA, bridge, tcL1.ChallengeDuration, infraL1.anvilChain.ContractAddresses.TokenAddress)

	cc, err := storeAPrime.GetConsensusChannelById(l2LedgerChannelId)
	if err != nil {
		t.Fatal(err)
	}
	oldL2SignedState := cc.SupportedSignedState()

	createVirtualChannelAndMakePayment(t, nodeAPrime, bridgeAddress, tcL2.ChallengeDuration)

	t.Run("Clear the registered challenge using checkpoint and exit L2 using latest L2 state", func(t *testing.T) {
		ledgerUpdatesChannelNodeA := nodeA.LedgerUpdatedChan(l1LedgerChannelId)

		// Alice unilaterally exits from L1 using old L2 signed state
		_, err := nodeA.MirrorBridgedDefund(l1LedgerChannelId, oldL2SignedState, true)
		if err != nil {
			t.Fatal(err)
		}

		newL2signedState, err := bridge.GetL2SupportedSignedState(l2LedgerChannelId)
		if err != nil {
			t.Log(err)
		}

		// Wait for challenge registered event
		listenForLedgerUpdates(ledgerUpdatesChannelNodeA, channel.Challenge)

		completedObjectiveChannel := nodeA.CompletedObjectives()
		// Bridge clears the challenge using new L2 signed state
		bridge.CounterChallenge(l1LedgerChannelId, types.Checkpoint, newL2signedState)

		// Wait for mirror bridged defund to complete on L1 (objective is completed after the challenge cleared event occurs)
		completedObjectiveId := <-completedObjectiveChannel
		expectedObjectiveId := protocols.ObjectiveId(mirrorbridgeddefund.ObjectivePrefix + l1LedgerChannelId.String())
		testhelpers.Assert(t, completedObjectiveId == expectedObjectiveId, "Expects objective Id to be %v", expectedObjectiveId)

		// Alice exits L1
		_, err = nodeAPrime.CloseBridgeChannel(l2LedgerChannelId)
		if err != nil {
			t.Fatal(err)
		}

		// Wait for mirror bridged defund to complete on L1
		completedObjectiveId = <-completedObjectiveChannel

		testhelpers.Assert(t, mirrorbridgeddefund.IsMirrorBridgedDefundObjective(completedObjectiveId), "Expects objective to be mirror bridged defund objective")
		objective, err := storeA.GetObjectiveById(completedObjectiveId)
		if err != nil {
			t.Fatal("mirror bridged defund objective not found", err)
		}
		testhelpers.Assert(t, objective.OwnsChannel() == l1LedgerChannelId, "Expects L1 ledger channel Id to be %v", l1LedgerChannelId)

		balanceNodeA, _ := utils.l1TokenBinding.BalanceOf(nil, tcL1.Participants[0].Address())
		balanceBridge, _ := utils.l1TokenBinding.BalanceOf(nil, tcL1.Participants[1].Address())
		t.Logf("Balance of node A %v \nBalance of Bridge %v", balanceNodeA, balanceBridge)

		// NodeA's balance is determined by subtracting amount paid from it's ledger deposit, while Bridge's balance is calculated by adding the amount received
		testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of node A (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit-payAmount)
		testhelpers.Assert(t, balanceBridge.Cmp(big.NewInt(payAmount)) == 0, "Balance of Bridge (%v) should be equal to (%v)", balanceBridge, payAmount)
	})
}

func TestBridgedFundWithCounterChallenge(t *testing.T) {
	utils, cleanup := initializeUtilsWithBridge(t, true)
	defer cleanup()

	tcL1, tcL2 := utils.tcL1, utils.tcL2
	nodeA, nodeAPrime := utils.nodeA, utils.nodeAPrime
	bridge, bridgeAddress := utils.bridge, utils.bridgeAddress
	storeA, storeAPrime := utils.storeA, utils.storeAPrime
	infraL1 := utils.infraL1

	l1LedgerChannelId, l2LedgerChannelId := createMirrorChannel(t, nodeA, bridge, tcL1.ChallengeDuration, infraL1.anvilChain.ContractAddresses.TokenAddress)

	cc, err := storeAPrime.GetConsensusChannelById(l2LedgerChannelId)
	if err != nil {
		t.Fatal(err)
	}
	oldL2SignedState := cc.SupportedSignedState()

	createVirtualChannelAndMakePayment(t, nodeAPrime, bridgeAddress, tcL2.ChallengeDuration)

	t.Run("Counter the registered challenge by challenging with new L2 state and exit L2 using the new L2 state", func(t *testing.T) {
		ledgerUpdatesChannelNodeA := nodeA.LedgerUpdatedChan(l1LedgerChannelId)

		// Alice unilaterally exits from L1 using old L2 signed state
		_, err := nodeA.MirrorBridgedDefund(l1LedgerChannelId, oldL2SignedState, true)
		if err != nil {
			t.Fatal(err)
		}

		newL2signedState, err := bridge.GetL2SupportedSignedState(l2LedgerChannelId)
		if err != nil {
			t.Log(err)
		}

		// Wait for Alice's challenge to be registered
		listenForLedgerUpdates(ledgerUpdatesChannelNodeA, channel.Challenge)

		completedObjectiveChannel := nodeA.CompletedObjectives()
		// Bridge counters the Alice' challenge using new L2 signed state
		bridge.CounterChallenge(l1LedgerChannelId, types.Challenge, newL2signedState)

		// Wait for mirror bridged defund to complete on L1
		completedObjectiveId := <-completedObjectiveChannel
		testhelpers.Assert(t, mirrorbridgeddefund.IsMirrorBridgedDefundObjective(completedObjectiveId), "Expects objective to be mirror bridged defund objective")
		objective, err := storeA.GetObjectiveById(completedObjectiveId)
		if err != nil {
			t.Fatal("mirror bridged defund objective not found", err)
		}
		testhelpers.Assert(t, objective.OwnsChannel() == l1LedgerChannelId, "Expects L1 ledger channel Id to be %v", l1LedgerChannelId)

		balanceNodeA, _ := utils.l1TokenBinding.BalanceOf(nil, tcL1.Participants[0].Address())
		balanceBridge, _ := utils.l1TokenBinding.BalanceOf(nil, tcL1.Participants[1].Address())
		t.Logf("Balance of node A %v \nBalance of Bridge %v", balanceNodeA, balanceBridge)

		// NodeA's balance is determined by subtracting amount paid from it's ledger deposit, while Bridge's balance is calculated by adding the amount received
		testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of node A (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit-payAmount)
		testhelpers.Assert(t, balanceBridge.Cmp(big.NewInt(payAmount)) == 0, "Balance of Bridge (%v) should be equal to (%v)", balanceBridge, payAmount)
	})
}

func TestBridgedFundWithIntermediary(t *testing.T) {
	utils, cleanup := initializeUtilsWithBridge(t, true)
	defer cleanup()

	tcL1, tcL2 := utils.tcL1, utils.tcL2
	nodeA, nodeAPrime := utils.nodeA, utils.nodeAPrime
	bridge, bridgeAddress := utils.bridge, utils.bridgeAddress
	bridgeMultiaddressL1, bridgeMultiaddressL2 := utils.bridgeMultiaddressL1, utils.bridgeMultiaddressL2
	storeA := utils.storeA
	infraL1, infraL2 := utils.infraL1, utils.infraL2
	dataFolder := utils.dataFolder

	nodeC, _, _, storeC, _ := setupIntegrationNode(tcL1, tcL1.Participants[2], infraL1, []string{bridgeMultiaddressL1}, dataFolder)
	defer nodeC.Close()

	nodeCPrime, _, _, _, _ := setupIntegrationNode(tcL2, tcL2.Participants[2], infraL2, []string{bridgeMultiaddressL2}, dataFolder)
	defer nodeCPrime.Close()

	var l1AliceBridgeLedgerChannelId types.Destination
	var l1CharlieBridgeLedgerChannelId types.Destination
	var l2AliceBridgeLedgerChannelId types.Destination
	var l2CharlieBridgeLedgerChannelId types.Destination

	t.Run("Create ledger channels on L1 and mirror it on L2", func(t *testing.T) {
		// Alice create ledger channel with bridge
		outcome := CreateLedgerOutcome(*nodeA.Address, bridgeAddress, ledgerChannelDeposit, ledgerChannelDeposit, infraL1.anvilChain.ContractAddresses.TokenAddress)
		l1LedgerChannelResponse, err := nodeA.CreateLedgerChannel(bridgeAddress, uint32(tcL1.ChallengeDuration), outcome)
		if err != nil {
			t.Fatal(err)
		}
		l1AliceBridgeLedgerChannelId = l1LedgerChannelResponse.ChannelId
		t.Log("Waiting for direct-fund objective to complete...")
		<-nodeA.ObjectiveCompleteChan(l1LedgerChannelResponse.Id)
		t.Log("L1 channel created", l1LedgerChannelResponse.Id)

		// Wait for mirror channel to be created
		createdMirrorChannel := <-bridge.CreatedMirrorChannels()

		l2AliceBridgeLedgerChannelId, _ = bridge.GetL2ChannelIdByL1ChannelId(l1LedgerChannelResponse.ChannelId)
		testhelpers.Assert(t, createdMirrorChannel == l2AliceBridgeLedgerChannelId, "Expects mirror channel id to be %v", l2AliceBridgeLedgerChannelId)

		checkLedgerChannel(t, l1AliceBridgeLedgerChannelId, CreateLedgerOutcome(*nodeA.Address, bridgeAddress, ledgerChannelDeposit, ledgerChannelDeposit, infraL1.anvilChain.ContractAddresses.TokenAddress), query.Open, nodeA)
		checkLedgerChannel(t, l2AliceBridgeLedgerChannelId, CreateLedgerOutcome(bridgeAddress, *nodeAPrime.Address, ledgerChannelDeposit, ledgerChannelDeposit, infraL1.anvilChain.ContractAddresses.TokenAddress), query.Open, nodeAPrime)

		// Irene create ledger channel with bridge
		outcome = CreateLedgerOutcome(*nodeC.Address, bridgeAddress, ledgerChannelDeposit, ledgerChannelDeposit, infraL1.anvilChain.ContractAddresses.TokenAddress)
		l1LedgerChannelResponse, err = nodeC.CreateLedgerChannel(bridgeAddress, uint32(tcL1.ChallengeDuration), outcome)
		if err != nil {
			t.Fatal(err)
		}
		l1CharlieBridgeLedgerChannelId = l1LedgerChannelResponse.ChannelId

		t.Log("Waiting for direct-fund objective to complete...")
		<-nodeC.ObjectiveCompleteChan(l1LedgerChannelResponse.Id)
		t.Log("L1 channel created", l1LedgerChannelResponse.Id)
		// Wait for mirror channel to be created
		createdMirrorChannel = <-bridge.CreatedMirrorChannels()

		l2CharlieBridgeLedgerChannelId, _ = bridge.GetL2ChannelIdByL1ChannelId(l1LedgerChannelResponse.ChannelId)
		testhelpers.Assert(t, createdMirrorChannel == l2CharlieBridgeLedgerChannelId, "Expects mirror channel id to be %v", l2CharlieBridgeLedgerChannelId)

		checkLedgerChannel(t, l1CharlieBridgeLedgerChannelId, CreateLedgerOutcome(*nodeC.Address, bridgeAddress, ledgerChannelDeposit, ledgerChannelDeposit, infraL1.anvilChain.ContractAddresses.TokenAddress), query.Open, nodeC)
		checkLedgerChannel(t, l2CharlieBridgeLedgerChannelId, CreateLedgerOutcome(bridgeAddress, *nodeCPrime.Address, ledgerChannelDeposit, ledgerChannelDeposit, infraL1.anvilChain.ContractAddresses.TokenAddress), query.Open, nodeCPrime)
	})

	t.Run("Create virtual channel on mirrored ledger channel and make payments via bridge as intermediary", func(t *testing.T) {
		virtualOutcome := initialPaymentOutcome(*nodeAPrime.Address, *nodeCPrime.Address, types.Address{})
		virtualResponse, _ := nodeAPrime.CreatePaymentChannel([]types.Address{bridgeAddress}, *nodeCPrime.Address, uint32(tcL2.ChallengeDuration), virtualOutcome)
		<-nodeAPrime.ObjectiveCompleteChan(virtualResponse.Id)

		// APrime pays CharliePrime
		err := nodeAPrime.Pay(virtualResponse.ChannelId, big.NewInt(payAmount))
		if err != nil {
			t.Fatal(err)
		}

		// Wait for CharliePrime to recieve voucher
		nodeCPrimeVoucher := <-nodeCPrime.ReceivedVouchers()
		t.Logf("Voucher recieved %+v", nodeCPrimeVoucher)

		// Virtual defund
		virtualDefundResponse, _ := nodeAPrime.ClosePaymentChannel(virtualResponse.ChannelId)
		nodeAPrimeObjCompleteChan := nodeAPrime.ObjectiveCompleteChan(virtualDefundResponse)
		nodeCPrimeObjCompleteChan := nodeCPrime.ObjectiveCompleteChan(virtualDefundResponse)

		<-nodeAPrimeObjCompleteChan
		<-nodeCPrimeObjCompleteChan

		checkLedgerChannel(t, l2AliceBridgeLedgerChannelId, CreateLedgerOutcome(*nodeAPrime.Address, bridgeAddress, ledgerChannelDeposit-payAmount, ledgerChannelDeposit+payAmount, infraL1.anvilChain.ContractAddresses.TokenAddress), query.Open, nodeAPrime)
		checkLedgerChannel(t, l2CharlieBridgeLedgerChannelId, CreateLedgerOutcome(bridgeAddress, *nodeCPrime.Address, ledgerChannelDeposit-payAmount, ledgerChannelDeposit+payAmount, infraL1.anvilChain.ContractAddresses.TokenAddress), query.Open, nodeCPrime)
	})

	t.Run("Exit to L1 using updated L2 ledger channel states after making payments", func(t *testing.T) {
		completedObjectiveChannel := nodeA.CompletedObjectives()
		// Alice exits
		_, err := nodeAPrime.CloseBridgeChannel(l2AliceBridgeLedgerChannelId)
		if err != nil {
			t.Fatal(err)
		}

		// Wait for mirror bridged defund (Alice<->Bridge) to complete on L1
		for completedObjectiveId := range completedObjectiveChannel {
			if mirrorbridgeddefund.IsMirrorBridgedDefundObjective(completedObjectiveId) {
				objective, err := storeA.GetObjectiveById(completedObjectiveId)
				if err != nil {
					t.Fatal("mirror bridged defund objective not found", err)
				}

				if objective.OwnsChannel() == l1AliceBridgeLedgerChannelId {
					break
				}
			}
		}

		checkLedgerChannel(t, l1AliceBridgeLedgerChannelId, CreateLedgerOutcome(*nodeA.Address, bridgeAddress, ledgerChannelDeposit-payAmount, ledgerChannelDeposit+payAmount, infraL1.anvilChain.ContractAddresses.TokenAddress), query.Complete, nodeA)

		completedObjectiveChannel = nodeC.CompletedObjectives()
		// Charlie exits
		_, err = nodeCPrime.CloseBridgeChannel(l2CharlieBridgeLedgerChannelId)
		if err != nil {
			t.Fatal(err)
		}

		// Wait for mirror bridged defund (Charlie<->Bridge) to complete on L1
		for completedObjectiveId := range completedObjectiveChannel {
			if mirrorbridgeddefund.IsMirrorBridgedDefundObjective(completedObjectiveId) {
				objective, err := storeC.GetObjectiveById(completedObjectiveId)
				if err != nil {
					t.Fatal("mirror bridged defund objective not found", err)
				}

				if objective.OwnsChannel() == l1CharlieBridgeLedgerChannelId {
					break
				}
			}
		}

		checkLedgerChannel(t, l1CharlieBridgeLedgerChannelId, CreateLedgerOutcome(*nodeC.Address, bridgeAddress, ledgerChannelDeposit+payAmount, ledgerChannelDeposit-payAmount, infraL1.anvilChain.ContractAddresses.TokenAddress), query.Complete, nodeC)

		balanceNodeA, _ := utils.l1TokenBinding.BalanceOf(nil, tcL1.Participants[0].Address())
		balanceNodeC, _ := utils.l1TokenBinding.BalanceOf(nil, tcL1.Participants[2].Address())
		t.Logf("Balance of node A %v \nBalance of node C %v", balanceNodeA, balanceNodeC)

		// NodeA's balance is determined by subtracting amount paid from its ledger deposit, while NodeC's balance is calculated by adding the amount received to its ledger deposit
		testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of node A (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit-payAmount)
		testhelpers.Assert(t, balanceNodeC.Cmp(big.NewInt(ledgerChannelDeposit+payAmount)) == 0, "Balance of node C (%v) should be equal to (%v)", balanceNodeC, ledgerChannelDeposit+payAmount)
	})
}

func TestBridgedFundWithChallenge(t *testing.T) {
	utils, cleanup := initializeUtilsWithBridge(t, true)
	defer cleanup()

	tcL1, tcL2 := utils.tcL1, utils.tcL2
	nodeA, nodeAPrime := utils.nodeA, utils.nodeAPrime
	bridge, bridgeAddress := utils.bridge, utils.bridgeAddress
	storeA, storeAPrime := utils.storeA, utils.storeAPrime
	infraL1 := utils.infraL1

	l1LedgerChannelId, l2LedgerChannelId := createMirrorChannel(t, nodeA, bridge, tcL1.ChallengeDuration, infraL1.anvilChain.ContractAddresses.TokenAddress)
	createVirtualChannelAndMakePayment(t, nodeAPrime, bridgeAddress, tcL2.ChallengeDuration)

	t.Run("Unilaterally exit to L1 using updated L2 ledger channel state after making payments", func(t *testing.T) {
		cc, err := storeAPrime.GetConsensusChannelById(l2LedgerChannelId)
		if err != nil {
			t.Fatal("required L2 ledger channel not found: %w", err)
		}

		l2SignedState := cc.SupportedSignedState()

		completedObjectiveChannel := nodeA.CompletedObjectives()
		// Alice unilaterally exits from L1 using L2 signed state
		_, err = nodeA.MirrorBridgedDefund(l1LedgerChannelId, l2SignedState, true)
		if err != nil {
			t.Fatal(err)
		}

		// Wait for mirror bridged defund to complete on L1
		completedObjectiveId := <-completedObjectiveChannel
		testhelpers.Assert(t, mirrorbridgeddefund.IsMirrorBridgedDefundObjective(completedObjectiveId), "Expects objective to be mirror bridged defund objective")
		objective, err := storeA.GetObjectiveById(completedObjectiveId)
		if err != nil {
			t.Fatal("mirror bridged defund objective not found", err)
		}
		testhelpers.Assert(t, objective.OwnsChannel() == l1LedgerChannelId, "Expects L1 ledger channel Id to be %v", l1LedgerChannelId)

		balanceNodeA, _ := utils.l1TokenBinding.BalanceOf(nil, tcL1.Participants[0].Address())
		balanceBridge, _ := utils.l1TokenBinding.BalanceOf(nil, tcL1.Participants[1].Address())
		t.Logf("Balance of node A %v \nBalance of Bridge %v", balanceNodeA, balanceBridge)

		// NodeA's balance is determined by subtracting amount paid from it's ledger deposit, while Bridge's balance is calculated by adding the amount received
		testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of node A (%v) should be equal to (%v)", balanceNodeA, ledgerChannelDeposit-payAmount)
		testhelpers.Assert(t, balanceBridge.Cmp(big.NewInt(payAmount)) == 0, "Balance of Bridge (%v) should be equal to (%v)", balanceBridge, payAmount)
	})
}

func TestExitL2WithVirtualChannelStateUnilaterally(t *testing.T) {
	utils, cleanupUtils := initializeUtils(t, false)
	defer cleanupUtils()

	tcL1, tcL2 := utils.tcL1, utils.tcL2
	nodeA, nodeB := utils.nodeA, utils.nodeB
	nodeAPrime, nodeBPrime := utils.nodeAPrime, utils.nodeBPrime
	chainServiceB := utils.chainServiceB
	testChainService := utils.testChainService
	storeA := utils.storeA
	storeAPrime, storeBPrime := utils.storeAPrime, utils.storeBPrime
	infraL1 := utils.infraL1

	l1ChannelId, mirroredLedgerChannelId := createL1L2Channels(t, nodeA, nodeB, nodeAPrime, nodeBPrime, storeA, tcL1, tcL2, chainServiceB)

	// Create virtual channel on mirrored ledger channel on L2 and make payments
	virtualChannel := createL2VirtualChannel(t, nodeAPrime, nodeBPrime, storeBPrime, tcL2)

	// Bridge pays APrime
	err := nodeBPrime.Pay(virtualChannel.Id, big.NewInt(payAmount))
	if err != nil {
		t.Fatal(err)
	}

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

		// Close Alice node to prevent creating objective from chain events and use test chain service for sending transactions
		nodeA.Close()

		// Node A calls modified `challenge` with L2 virtual channel state
		virtualChallengerSig, _ := NitroAdjudicator.SignChallengeMessage(signedVirtualState.State(), tcL1.Participants[0].PrivateKey)
		mirrroVirtualChallengeTx := protocols.NewChallengeTransaction(virtualChannelId, signedVirtualState, []state.SignedState{signedPostFundState}, virtualChallengerSig)
		_, err = testChainService.SendTransaction(mirrroVirtualChallengeTx)
		if err != nil {
			t.Error(err)
		}

		// Listen for challenge registered event
		event := waitForEvent(t, testChainService.EventEngineFeed(), chainservice.ChallengeRegisteredEvent{})
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
		_, err = testChainService.SendTransaction(challengeTx)
		if err != nil {
			t.Error(err)
		}

		event = waitForEvent(t, testChainService.EventEngineFeed(), chainservice.ChallengeRegisteredEvent{})
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
		_, err = testChainService.SendTransaction(reclaimTx)
		if err != nil {
			t.Error(err)
		}

		// Listen for reclaimed event
		event = waitForEvent(t, testChainService.EventEngineFeed(), chainservice.ReclaimedEvent{})
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
		_, err = testChainService.SendTransaction(mirrorTransferAllTx)
		if err != nil {
			t.Error(err)
		}

		// Listen for allocation updated event
		event = waitForEvent(t, testChainService.EventEngineFeed(), chainservice.AllocationUpdatedEvent{})
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
	_, err = bridgeChainService.SendTransaction(setL2ToL1Tx)
	if err != nil {
		t.Error(err)
	}

	return l1LedgerChannelId, response.ChannelId
}

func createL2VirtualChannel(t *testing.T, nodeAPrime node.Node, nodeBPrime node.Node, L2bridgeStore store.Store, tcL2 TestCase) *channel.Channel {
	// Create virtual channel on mirrored ledger channel on L2
	virtualOutcome := initialPaymentOutcome(*nodeBPrime.Address, *nodeAPrime.Address, types.Address{})

	virtualResponse, _ := nodeBPrime.CreatePaymentChannel([]types.Address{}, *nodeAPrime.Address, uint32(tcL2.ChallengeDuration), virtualOutcome)
	waitForObjectives(t, nodeBPrime, nodeAPrime, []node.Node{}, []protocols.ObjectiveId{virtualResponse.Id})

	checkPaymentChannel(t, virtualResponse.ChannelId, virtualOutcome, query.Open, nodeBPrime, nodeAPrime)

	virtualChannel, _ := L2bridgeStore.GetChannelById(virtualResponse.ChannelId)

	return virtualChannel
}

func initializeUtils(t *testing.T, closeBridge bool) (Utils, func()) {
	tcL1 := TestCase{
		Chain:             AnvilChain,
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
		Chain:             LaconicdChain,
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

	infraL1 := setupSharedInfra(tcL1)
	infraL2 := setupSharedInfra(tcL2)

	// Create go-nitro nodes
	nodeA, _, _, storeA, chainServiceA := setupIntegrationNode(tcL1, tcL1.Participants[0], infraL1, []string{}, dataFolder)

	nodeB, _, _, storeB, chainServiceB := setupIntegrationNode(tcL1, tcL1.Participants[1], infraL1, []string{}, dataFolder)

	infraL2.laconicdChain.ContractAddresses.CaAddress = infraL1.anvilChain.ContractAddresses.CaAddress
	infraL2.laconicdChain.ContractAddresses.VpaAddress = infraL1.anvilChain.ContractAddresses.VpaAddress

	nodeBPrime, _, _, storeBPrime, _ := setupIntegrationNode(tcL2, tcL2.Participants[0], infraL2, []string{}, dataFolder)

	nodeAPrime, _, _, storeAPrime, _ := setupIntegrationNode(tcL2, tcL2.Participants[1], infraL2, []string{}, dataFolder)

	// Seperate chain service to listen for events
	testChainService := setupChainService(tcL1, tcL1.Participants[0], infraL1)

	utils := Utils{
		tcL1:             tcL1,
		tcL2:             tcL2,
		nodeA:            nodeA,
		nodeB:            nodeB,
		nodeAPrime:       nodeAPrime,
		nodeBPrime:       nodeBPrime,
		chainServiceA:    chainServiceA,
		chainServiceB:    chainServiceB,
		testChainService: testChainService,
		storeA:           storeA,
		storeB:           storeB,
		storeAPrime:      storeAPrime,
		storeBPrime:      storeBPrime,
		infraL1:          infraL1,
		infraL2:          infraL2,
	}

	cleanupUtils := func() {
		cleanup()

		if closeBridge {
			utils.nodeB.Close()
			utils.nodeBPrime.Close()
		}

		utils.testChainService.Close()

		utils.nodeA.Close()
		utils.nodeAPrime.Close()

		utils.infraL1.Close(t)
		utils.infraL2.Close(t)
	}

	return utils, cleanupUtils
}

func initializeUtilsWithBridge(t *testing.T, closeBridge bool) (UtilsWithBridge, func()) {
	tcL1 := TestCase{
		Chain:             AnvilChain,
		MessageService:    P2PMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
			{StoreType: MemStore, Actor: testactors.Irene},
		},
		deployerIndex: 1,
	}

	tcL2 := TestCase{
		Chain:             LaconicdChain,
		MessageService:    P2PMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.BobPrime},
			{StoreType: MemStore, Actor: testactors.AlicePrime},
			{StoreType: MemStore, Actor: testactors.Irene},
		},
		ChainPort:     "8546",
		deployerIndex: 0,
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()

	infraL1 := setupSharedInfra(tcL1)

	infraL2 := setupSharedInfra(tcL2)

	l1TokenBinding, err := Token.NewToken(infraL1.anvilChain.ContractAddresses.TokenAddress, infraL1.anvilChain.EthClient)
	if err != nil {
		t.Fatal(err)
	}

	bridgeConfig := bridge.BridgeConfig{
		L1ChainUrl:        infraL1.anvilChain.ChainUrl,
		L1ChainStartBlock: 0,
		ChainPK:           infraL1.anvilChain.ChainPks[tcL1.Participants[1].ChainAccountIndex],
		StateChannelPK:    common.Bytes2Hex(tcL1.Participants[1].PrivateKey),
		NaAddress:         infraL1.anvilChain.ContractAddresses.NaAddress.String(),
		VpaAddress:        infraL1.anvilChain.ContractAddresses.VpaAddress.String(),
		CaAddress:         infraL1.anvilChain.ContractAddresses.CaAddress.String(),
		DurableStoreDir:   dataFolder,
		BridgePublicIp:    DEFAULT_PUBLIC_IP,
		NodeL1MsgPort:     int(tcL1.Participants[1].Port),
		NodeL2MsgPort:     int(tcL2.Participants[0].Port),
	}

	bridge := bridge.New()
	_, _, bridgeMultiaddressL1, bridgeMultiaddressL2, err := bridge.Start(bridgeConfig)
	if err != nil {
		t.Log("error in starting bridge", err)
	}
	bridgeAddress := bridge.GetBridgeAddress()

	nodeA, _, _, storeA, chainServiceA := setupIntegrationNode(tcL1, tcL1.Participants[0], infraL1, []string{bridgeMultiaddressL1}, dataFolder)
	nodeAPrime, _, _, storeAPrime, chainServiceAPrime := setupIntegrationNode(tcL2, tcL2.Participants[1], infraL2, []string{bridgeMultiaddressL2}, dataFolder)

	utils := UtilsWithBridge{
		tcL1:                 tcL1,
		tcL2:                 tcL2,
		nodeA:                nodeA,
		bridge:               bridge,
		bridgeAddress:        bridgeAddress,
		bridgeMultiaddressL1: bridgeMultiaddressL1,
		bridgeMultiaddressL2: bridgeMultiaddressL2,
		dataFolder:           dataFolder,
		nodeAPrime:           nodeAPrime,
		chainServiceA:        chainServiceA,
		chainServiceAPrime:   chainServiceAPrime,
		storeA:               storeA,
		storeAPrime:          storeAPrime,
		infraL1:              infraL1,
		infraL2:              infraL2,
		l1TokenBinding:       l1TokenBinding,
	}

	cleanupUtilsWithBridge := func() {
		cleanup()

		if closeBridge {
			utils.bridge.Close()
		}

		utils.nodeA.Close()
		utils.nodeAPrime.Close()

		utils.infraL1.Close(t)
		utils.infraL2.Close(t)
	}

	return utils, cleanupUtilsWithBridge
}

func createMirrorChannel(t *testing.T, node node.Node, bridge *bridge.Bridge, challengeDuration uint, l1AssetAddress common.Address) (types.Destination, types.Destination) {
	outcome := CreateLedgerOutcome(*node.Address, bridge.GetBridgeAddress(), ledgerChannelDeposit, 0, l1AssetAddress)
	l1LedgerChannelResponse, err := node.CreateLedgerChannel(bridge.GetBridgeAddress(), uint32(challengeDuration), outcome)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Waiting for direct-fund objective to complete...")
	l1LedgerChannelId := l1LedgerChannelResponse.ChannelId

	<-node.ObjectiveCompleteChan(l1LedgerChannelResponse.Id)
	t.Log("L1 channel created", l1LedgerChannelId)

	// Wait for mirror channel to be created
	l2LedgerChannelId := <-bridge.CreatedMirrorChannels()
	t.Log("L2 channel created", l2LedgerChannelId)

	return l1LedgerChannelId, l2LedgerChannelId
}

func createVirtualChannelAndMakePayment(t *testing.T, node node.Node, counterParty common.Address, challengeDuration uint) {
	// Create virtual channel
	virtualOutcome := initialPaymentOutcome(*node.Address, counterParty, types.Address{})
	virtualResponse, _ := node.CreatePaymentChannel([]types.Address{}, counterParty, uint32(challengeDuration), virtualOutcome)
	<-node.ObjectiveCompleteChan(virtualResponse.Id)

	// Make payment
	err := node.Pay(virtualResponse.ChannelId, big.NewInt(payAmount))
	if err != nil {
		t.Fatal(err)
	}

	// Close virtual channel
	virtualDefundResponse, _ := node.ClosePaymentChannel(virtualResponse.ChannelId)
	<-node.ObjectiveCompleteChan(virtualDefundResponse)
}
