package node_test

import (
	"math/big"
	"testing"

	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/node/engine/messageservice"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

func TestBridge(t *testing.T) {
	const payAmount = 2000

	const (
		CHAIN_URL_L1 = "ws://127.0.0.1:8545"
		CHAIN_URL_L2 = "ws://127.0.0.1:8546"
	)

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
			{StoreType: MemStore, Actor: testactors.Irene},
			{StoreType: MemStore, Actor: testactors.Ivan},
		},
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infraL1 := setupSharedInfraWithChainUrlArg(tc, CHAIN_URL_L1)
	defer infraL1.Close(t)

	infraL2 := setupSharedInfraWithChainUrlArg(tc, CHAIN_URL_L2)
	defer infraL2.Close(t)

	if tc.MessageService == TestMessageService {

		broker := messageservice.NewBroker()
		infraL1.broker = &broker
		infraL2.broker = &broker
	}

	// Create go-nitro nodes
	nodeA, _, _, storeA, _ := setupIntegrationNode(tc, tc.Participants[0], infraL1, []string{}, dataFolder)
	defer nodeA.Close()

	nodeB, _, _, _, _ := setupIntegrationNode(tc, tc.Participants[1], infraL1, []string{}, dataFolder)
	defer nodeB.Close()

	nodeBPrime, _, _, storeBPrime, _ := setupIntegrationNode(tc, tc.Participants[2], infraL2, []string{}, dataFolder)
	defer nodeBPrime.Close()

	nodeAPrime, _, _, _, _ := setupIntegrationNode(tc, tc.Participants[3], infraL2, []string{}, dataFolder)
	defer nodeAPrime.Close()

	// Create ledger channel
	l1LedgerChannelId := openLedgerChannel(t, nodeA, nodeB, types.Address{}, uint32(tc.ChallengeDuration))

	l1LedgerChannel, err := storeA.GetConsensusChannelById(l1LedgerChannelId)
	if err != nil {
		t.Error(err)
	}

	l1ledgerChannelState := l1LedgerChannel.SupportedSignedState()

	l1ledgerChannelStateClone := l1ledgerChannelState.Clone()

	nodeBPrimeAllocation := l1ledgerChannelStateClone.State().Outcome[0].Allocations[1]
	nodeBPrimeAllocation.Destination = types.AddressToDestination(*nodeBPrime.Address)

	nodeAPrimeAllocation := l1ledgerChannelStateClone.State().Outcome[0].Allocations[0]
	nodeAPrimeAllocation.Destination = types.AddressToDestination(*nodeAPrime.Address)

	// Create extended state outcome based on l1ChannelState
	l2ChannelOutcome := outcome.Exit{
		{
			Asset:         l1ledgerChannelStateClone.State().Outcome[0].Asset,
			AssetMetadata: l1ledgerChannelStateClone.State().Outcome[0].AssetMetadata,
			Allocations: outcome.Allocations{
				nodeBPrimeAllocation,
				nodeAPrimeAllocation,
			},
		},
	}

	// Create mirrored ledger channel between node BPrime and APrime
	response, err := nodeBPrime.CreateBridgeChannel(*nodeAPrime.Address, uint32(tc.ChallengeDuration), l2ChannelOutcome)
	if err != nil {
		t.Error(err)
	}

	t.Log("Waiting for bridge-fund objective to complete...")

	<-nodeBPrime.ObjectiveCompleteChan(response.Id)
	<-nodeAPrime.ObjectiveCompleteChan(response.Id)

	t.Log("Completed bridge-fund objective")

	virtualOutcome := initialPaymentOutcome(*nodeBPrime.Address, *nodeAPrime.Address, types.Address{})

	virtualResponse, _ := nodeBPrime.CreatePaymentChannel([]types.Address{}, *nodeAPrime.Address, uint32(tc.ChallengeDuration), virtualOutcome)
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

	latestSignedState := getLatestSignedState(storeBPrime, response.ChannelId)

	balanceNodeBPrime := latestSignedState.State().Outcome[0].Allocations[0].Amount
	balanceNodeAPrime := latestSignedState.State().Outcome[0].Allocations[1].Amount
	t.Log("Balance of node BPrime", balanceNodeBPrime, "\nBalance of node APrime", balanceNodeAPrime)

	// BPrime's balance is determined by subtracting amount paid from it's ledger deposit, while APrime's balance is calculated by adding it's ledger deposit to the amount received
	testhelpers.Assert(t, balanceNodeBPrime.Cmp(big.NewInt(ledgerChannelDeposit-payAmount)) == 0, "Balance of node BPrime (%v) should be equal to (%v)", balanceNodeBPrime, ledgerChannelDeposit-payAmount)
	testhelpers.Assert(t, balanceNodeAPrime.Cmp(big.NewInt(ledgerChannelDeposit+payAmount)) == 0, "Balance of node APrime (%v) should be equal to (%v)", balanceNodeAPrime, ledgerChannelDeposit+payAmount)
}
