package node_test

import (
	"math/big"
	"testing"

	"github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

// Flow:
// 1. Create ledger channel on L1
// 2. Create mirrored channel on L2
// 3. Create virtual payment channel on top of mirrored ledger channel and make payments
// 4. Virtual defund and check balance on L2
func TestBridge(t *testing.T) {
	const payAmount = 2000

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
	}

	tcL2 := TestCase{
		Chain:             AnvilChain,
		MessageService:    TestMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Irene},
			{StoreType: MemStore, Actor: testactors.Ivan},
		},
		ChainPort: "8546",
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infraL1 := setupSharedInfra(tcL1)
	defer infraL1.Close(t)

	infraL2 := setupSharedInfra(tcL2)
	defer infraL2.Close(t)

	// Create go-nitro nodes
	nodeA, _, _, storeA, _ := setupIntegrationNode(tcL1, tcL1.Participants[0], infraL1, []string{}, dataFolder)
	defer nodeA.Close()

	nodeB, _, _, _, _ := setupIntegrationNode(tcL1, tcL1.Participants[1], infraL1, []string{}, dataFolder)
	defer nodeB.Close()

	nodeBPrime, _, _, storeBPrime, _ := setupIntegrationNode(tcL2, tcL2.Participants[0], infraL2, []string{}, dataFolder)
	defer nodeBPrime.Close()

	nodeAPrime, _, _, _, _ := setupIntegrationNode(tcL2, tcL2.Participants[1], infraL2, []string{}, dataFolder)
	defer nodeAPrime.Close()

	mirroredLedgerChannelId := types.Destination{}

	t.Run("Create ledger channel on L1 and mirror it on L2", func(t *testing.T) {
		// Create ledger channel
		l1LedgerChannelId := openLedgerChannel(t, nodeA, nodeB, types.Address{}, uint32(tcL1.ChallengeDuration))

		l1LedgerChannel, err := storeA.GetConsensusChannelById(l1LedgerChannelId)
		if err != nil {
			t.Error(err)
		}

		l1ledgerChannelState := l1LedgerChannel.SupportedSignedState()
		l1ledgerChannelStateClone := l1ledgerChannelState.Clone()

		l1ledgerChannelStateClone.State().Outcome[0].Allocations[0].Destination = types.AddressToDestination(*nodeAPrime.Address)
		l1ledgerChannelStateClone.State().Outcome[0].Allocations[1].Destination = types.AddressToDestination(*nodeBPrime.Address)

		// Put NodeBPrime's allocation at index 0 as it creates mirrored ledger channel
		tempAllocation := l1ledgerChannelStateClone.State().Outcome[0].Allocations[0].Destination
		l1ledgerChannelStateClone.State().Outcome[0].Allocations[0].Destination = l1ledgerChannelStateClone.State().Outcome[0].Allocations[1].Destination
		l1ledgerChannelStateClone.State().Outcome[0].Allocations[1].Destination = tempAllocation

		// Create extended state outcome based on l1ChannelState
		l2ChannelOutcome := l1ledgerChannelStateClone.State().Outcome

		// Create mirrored ledger channel between node BPrime and APrime
		response, err := nodeBPrime.CreateBridgeChannel(*nodeAPrime.Address, uint32(tcL2.ChallengeDuration), l2ChannelOutcome)
		if err != nil {
			t.Error(err)
		}

		mirroredLedgerChannelId = response.ChannelId

		t.Log("Waiting for bridge-fund objective to complete...")

		<-nodeBPrime.ObjectiveCompleteChan(response.Id)
		<-nodeAPrime.ObjectiveCompleteChan(response.Id)

		t.Log("Completed bridge-fund objective")
	})

	t.Run("Create virtual channel on mirrored ledger channel and make payments", func(t *testing.T) {
		// Create virtual channel on mirrored ledger channel on L2 and make payments
		virtualOutcome := initialPaymentOutcome(*nodeBPrime.Address, *nodeAPrime.Address, types.Address{})

		virtualResponse, _ := nodeBPrime.CreatePaymentChannel([]types.Address{}, *nodeAPrime.Address, uint32(tcL2.ChallengeDuration), virtualOutcome)
		waitForObjectives(t, nodeBPrime, nodeAPrime, []node.Node{}, []protocols.ObjectiveId{virtualResponse.Id})

		checkPaymentChannel(t, virtualResponse.ChannelId, virtualOutcome, query.Open, nodeBPrime, nodeAPrime)

		virtualChannel, _ := storeBPrime.GetChannelById(virtualResponse.ChannelId)

		// Bridge pays APrime
		nodeBPrime.Pay(virtualResponse.ChannelId, big.NewInt(payAmount))

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
}
