package node_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/internal/testactors"
	td "github.com/statechannels/go-nitro/internal/testdata"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	Token "github.com/statechannels/go-nitro/node/engine/chainservice/erc20"
	"github.com/statechannels/go-nitro/node/engine/messageservice"
	p2pms "github.com/statechannels/go-nitro/node/engine/messageservice/p2p-message-service"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

func TestSimpleIntegrationScenario(t *testing.T) {
	simpleCase := TestCase{
		Description:    "Simple test",
		Chain:          MockChain,
		MessageService: TestMessageService,
		NumOfChannels:  1,
		MessageDelay:   0,
		LogName:        "simple_integration",
		NumOfHops:      1,
		NumOfPayments:  1,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
			{StoreType: MemStore, Actor: testactors.Irene},
		},
	}

	RunIntegrationTestCase(simpleCase, t)
}

func TestComplexIntegrationScenario(t *testing.T) {
	complexCase := TestCase{
		Description:    "Complex test",
		Chain:          SimulatedChain,
		MessageService: P2PMessageService,
		NumOfChannels:  5,
		MessageDelay:   0,
		LogName:        "complex_integration",
		NumOfHops:      2,
		NumOfPayments:  5,
		Participants: []TestParticipant{
			{StoreType: DurableStore, Actor: testactors.Alice},
			{StoreType: DurableStore, Actor: testactors.Bob},
			{StoreType: DurableStore, Actor: testactors.Irene},
			{StoreType: DurableStore, Actor: testactors.Ivan},
		},
	}
	RunIntegrationTestCase(complexCase, t)
}

func TestSwapFund(t *testing.T) {
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

	// TokenBinding
	_, err := Token.NewToken(infra.anvilChain.ContractAddresses.TokenAddresses[0], infra.anvilChain.EthClient)
	if err != nil {
		t.Fatal(err)
	}

	// Create go-nitro nodes
	nodeA, _, _, _, _ := setupIntegrationNode(testCase, testCase.Participants[0], infra, []string{}, dataFolder)
	defer nodeA.Close()
	nodeB, _, _, _, _ := setupIntegrationNode(testCase, testCase.Participants[1], infra, []string{}, dataFolder)
	defer nodeB.Close()

	outcomeEth := CreateLedgerOutcome(*nodeA.Address, *nodeB.Address, ledgerChannelDeposit, ledgerChannelDeposit, common.Address{})
	outcomeCustomToken := CreateLedgerOutcome(*nodeA.Address, *nodeB.Address, ledgerChannelDeposit, ledgerChannelDeposit, infra.anvilChain.ContractAddresses.TokenAddresses[0])

	outcomeCustomToken2 := CreateLedgerOutcome(*nodeA.Address, *nodeB.Address, ledgerChannelDeposit, ledgerChannelDeposit, infra.anvilChain.ContractAddresses.TokenAddresses[1])

	multiAssetOutcome := append(outcomeEth, outcomeCustomToken...)
	multiAssetOutcome = append(multiAssetOutcome, outcomeCustomToken2...)

	// Create ledger channel
	ledgerResponse, err := nodeA.CreateLedgerChannel(*nodeB.Address, uint32(testCase.ChallengeDuration), multiAssetOutcome)
	if err != nil {
		t.Error("error creating ledger channel", err)
	}

	t.Log("Waiting for direct-fund objective to complete...")

	chA := nodeA.ObjectiveCompleteChan(ledgerResponse.Id)
	chB := nodeB.ObjectiveCompleteChan(ledgerResponse.Id)
	<-chA
	<-chB

	multiassetSwapChannelOutcome := outcome.Exit{
		outcome.SingleAssetExit{
			Asset: common.Address{},
			Allocations: outcome.Allocations{
				outcome.Allocation{
					Destination: types.AddressToDestination(*nodeA.Address),
					Amount:      big.NewInt(int64(1001)),
				},
				outcome.Allocation{
					Destination: types.AddressToDestination(*nodeB.Address),
					Amount:      big.NewInt(int64(1002)),
				},
			},
		},
		outcome.SingleAssetExit{
			Asset: infra.anvilChain.ContractAddresses.TokenAddresses[0],
			Allocations: outcome.Allocations{
				outcome.Allocation{
					Destination: types.AddressToDestination(*nodeA.Address),
					Amount:      big.NewInt(int64(501)),
				},
				outcome.Allocation{
					Destination: types.AddressToDestination(*nodeB.Address),
					Amount:      big.NewInt(int64(502)),
				},
			},
		},
		outcome.SingleAssetExit{
			Asset: infra.anvilChain.ContractAddresses.TokenAddresses[1],
			Allocations: outcome.Allocations{
				outcome.Allocation{
					Destination: types.AddressToDestination(*nodeA.Address),
					Amount:      big.NewInt(int64(601)),
				},
				outcome.Allocation{
					Destination: types.AddressToDestination(*nodeB.Address),
					Amount:      big.NewInt(int64(602)),
				},
			},
		},
	}

	swapChannelresponse, err := nodeA.CreateSwapChannel(
		nil,
		*nodeB.Address,
		0,
		multiassetSwapChannelOutcome,
	)
	if err != nil {
		t.Fatal(err)
	}

	chB = nodeB.ObjectiveCompleteChan(swapChannelresponse.Id)
	<-nodeA.ObjectiveCompleteChan(swapChannelresponse.Id)
	<-chB

	t.Log("Completed swap-fund objective")

	checkSwapChannel(t, swapChannelresponse.ChannelId, multiassetSwapChannelOutcome, query.Open, nodeA, nodeB)
}

// RunIntegrationTestCase runs the integration test case.
func RunIntegrationTestCase(tc TestCase, t *testing.T) {
	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	t.Run(tc.Description, func(t *testing.T) {
		err := tc.Validate()
		if err != nil {
			t.Fatal(err)
		}
		infra := setupSharedInfra(tc)
		defer infra.Close(t)

		msgServices := make([]messageservice.MessageService, 0)

		// Setup clients
		t.Log("Initalizing intermediary node(s)...")
		intermediaries := make([]node.Node, 0)
		bootPeers := make([]string, 0)
		for _, intermediary := range tc.Participants[2:] {
			clientI, msgI, multiAddr, _, _ := setupIntegrationNode(tc, intermediary, infra, []string{}, dataFolder)

			intermediaries = append(intermediaries, clientI)
			msgServices = append(msgServices, msgI)
			bootPeers = append(bootPeers, multiAddr)
		}

		defer func() {
			for i := range intermediaries {
				intermediaries[i].Close()
			}
		}()
		t.Log("Intermediary node(s) setup complete")

		clientA, msgA, _, _, _ := setupIntegrationNode(tc, tc.Participants[0], infra, bootPeers, dataFolder)
		defer clientA.Close()
		msgServices = append(msgServices, msgA)

		clientB, msgB, _, _, _ := setupIntegrationNode(tc, tc.Participants[1], infra, bootPeers, dataFolder)
		defer clientB.Close()
		msgServices = append(msgServices, msgB)

		if tc.MessageService != TestMessageService {
			p2pServices := make([]*p2pms.P2PMessageService, len(tc.Participants))
			for i, msgService := range msgServices {
				p2pServices[i] = msgService.(*p2pms.P2PMessageService)
			}

			t.Log("Waiting for peer info exchange...")
			waitForPeerInfoExchange(p2pServices...)
			t.Log("Peer info exchange complete")
		}

		asset := common.Address{}
		// Setup ledger channels between Alice/Bob and intermediaries
		aliceLedgers := make([]types.Destination, tc.NumOfHops)
		bobLedgers := make([]types.Destination, tc.NumOfHops)
		for i, clientI := range intermediaries {
			t.Log("DEBUG: Setting up ledger channel between Alice/Bob and intermediaries, intermediary number", i)
			// Setup and check the ledger channel between Alice and the intermediary
			aliceLedgers[i] = openLedgerChannel(t, clientA, clientI, asset, 0)
			checkLedgerChannel(t, aliceLedgers[i], CreateLedgerOutcome(*clientA.Address, *clientI.Address, ledgerChannelDeposit, ledgerChannelDeposit, asset), query.Open, clientA)
			// Setup and check the ledger channel between Bob and the intermediary
			bobLedgers[i] = openLedgerChannel(t, clientI, clientB, asset, 0)
			checkLedgerChannel(t, bobLedgers[i], CreateLedgerOutcome(*clientI.Address, *clientB.Address, ledgerChannelDeposit, ledgerChannelDeposit, asset), query.Open, clientB)

		}

		t.Log("DEBUG: After setting up ledger channels between Alice/Bob and intermediaries")

		if tc.NumOfHops == 2 {
			openLedgerChannel(t, intermediaries[0], intermediaries[1], asset, 0)
			t.Log("DEBUG: After setting up ledger channels when NumOfHops is 2")
		}
		// Setup virtual channels
		objectiveIds := make([]protocols.ObjectiveId, tc.NumOfChannels)
		virtualIds := make([]types.Destination, tc.NumOfChannels)
		for i := 0; i < int(tc.NumOfChannels); i++ {
			outcome := td.Outcomes.Create(testactors.Alice.Address(), testactors.Bob.Address(), virtualChannelDeposit, 0, types.Address{})
			response, err := clientA.CreatePaymentChannel(
				clientAddresses(intermediaries),
				testactors.Bob.Address(),
				0,
				outcome,
			)
			if err != nil {
				t.Fatal(err)
			}

			t.Log("DEBUG: Created virtual channel, number: ", i)
			objectiveIds[i] = response.Id
			virtualIds[i] = response.ChannelId

		}
		// Wait for all the virtual channels to be ready
		waitForObjectives(t, clientA, clientB, intermediaries, objectiveIds)

		t.Log("DEBUG: After Setting up virtual channels")

		// Check all the virtual channels
		for i := 0; i < len(virtualIds); i++ {
			checkPaymentChannel(t,
				virtualIds[i],
				initialPaymentOutcome(*clientA.Address, *clientB.Address, asset),
				query.Open,
				clientA, clientB)
		}

		// Send payments
		for i := 0; i < len(virtualIds); i++ {
			for j := 0; j < int(tc.NumOfPayments); j++ {
				err = clientA.Pay(virtualIds[i], big.NewInt(int64(1)))
				if err != nil {
					t.Fatal(err)
				}
			}
		}

		t.Log("DEBUG: After making payments")

		// Wait for all the vouchers to be received by bob
		for i := 0; i < len(virtualIds)*int(tc.NumOfPayments); i++ {
			<-clientB.ReceivedVouchers()
		}

		t.Log("DEBUG: After waiting for vouchers")

		// Check the payment channels have the correct outcome after the payments
		for i := 0; i < len(virtualIds); i++ {
			checkPaymentChannel(t,
				virtualIds[i],
				finalPaymentOutcome(*clientA.Address, *clientB.Address, asset, tc.NumOfPayments, 1),
				query.Open,
				clientA, clientB)
		}

		t.Log("DEBUG: After checking payment channels")

		// Close virtual channels
		closeVirtualIds := make([]protocols.ObjectiveId, len(virtualIds))
		for i := 0; i < len(virtualIds); i++ {
			// alternative who is responsible for closing the channel
			switch i % 2 {
			case 0:
				closeVirtualIds[i], err = clientA.ClosePaymentChannel(virtualIds[i])
				if err != nil {
					t.Fatal(err)
				}
			case 1:
				closeVirtualIds[i], err = clientB.ClosePaymentChannel(virtualIds[i])
				if err != nil {
					t.Fatal(err)
				}
			}
		}

		waitForObjectives(t, clientA, clientB, intermediaries, closeVirtualIds)

		t.Log("DEBUG: After closing virtual channels")

		// Close all the ledger channels we opened

		closeLedgerChannel(t, clientA, intermediaries[0], aliceLedgers[0])
		checkLedgerChannel(t, aliceLedgers[0], finalAliceLedger(*intermediaries[0].Address, asset, tc.NumOfPayments, 1, tc.NumOfChannels), query.Complete, clientA)
		t.Log("DEBUG: After closing first alice ledger Channel")

		// TODO: This is brittle, we should generalize this to n number of intermediaries
		if tc.NumOfHops == 1 {
			closeLedgerChannel(t, intermediaries[0], clientB, bobLedgers[0])
			checkLedgerChannel(t, bobLedgers[0], finalBobLedger(*intermediaries[0].Address, asset, tc.NumOfPayments, 1, tc.NumOfChannels), query.Complete, clientB)
			t.Log("DEBUG: After closing ledger channel when NumOfHops is 1")
		}
		if tc.NumOfHops == 2 {
			closeLedgerChannel(t, intermediaries[1], clientB, bobLedgers[1])
			checkLedgerChannel(t, bobLedgers[1], finalBobLedger(*intermediaries[1].Address, asset, tc.NumOfPayments, 1, tc.NumOfChannels), query.Complete, clientB)
			t.Log("DEBUG: After closing ledger channel when NumOfHops is 2")
		}

		t.Log("DEBUG: After closing all ledger channels")

		var chainLastConfirmedBlockNum uint64
		if infra.mockChain != nil {
			chainLastConfirmedBlockNum = infra.mockChain.BlockNum
		} else if infra.simulatedChain != nil {
			latestBlock, err := infra.simulatedChain.BlockByNumber(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			chainLastConfirmedBlockNum = latestBlock.NumberU64() - chainservice.REQUIRED_BLOCK_CONFIRMATIONS
		}

		t.Log("DEBUG: Waiting for block confirmations")

		waitForClientBlockNum(t, clientA, chainLastConfirmedBlockNum, 10*time.Second)
		waitForClientBlockNum(t, clientB, chainLastConfirmedBlockNum, 10*time.Second)

		t.Log("DEBUG: After waiting for client block num")
	})
}

func waitForClientBlockNum(t *testing.T, n node.Node, targetBlockNum uint64, timeout time.Duration) {
	// Setup up a context with a timeout so we exit if we don't get the block num in time
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	lastBlockNum := uint64(0)
	var err error
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("expected block num of at least %d, got %d", targetBlockNum, lastBlockNum)
		default:
			lastBlockNum, err = n.GetLastBlockNum()
			if err != nil {
				t.Fatal(err)
			}
			if lastBlockNum >= targetBlockNum {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
