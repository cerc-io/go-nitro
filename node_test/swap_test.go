package node_test

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	Token "github.com/statechannels/go-nitro/node/engine/chainservice/erc20"
	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/payments"
	"github.com/statechannels/go-nitro/protocols/directfund"
	"github.com/statechannels/go-nitro/protocols/swap"
	"github.com/statechannels/go-nitro/protocols/swapfund"
	"github.com/statechannels/go-nitro/types"
)

type TestUtils struct {
	tc                           TestCase
	nodeA, nodeB                 node.Node
	chainServiceA, chainServiceB chainservice.ChainService
	storeA, storeB               store.Store
	infra                        sharedTestInfrastructure
}

func initializeNodesAndInfra(t *testing.T) (TestUtils, func()) {
	testCase := TestCase{
		Description:       "Swap test",
		Chain:             AnvilChain,
		MessageService:    TestMessageService,
		ChallengeDuration: 0,
		MessageDelay:      0,
		LogName:           "Swap_test",
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
			{StoreType: MemStore, Actor: testactors.Irene},
		},
	}

	dataFolder, removeTempFolder := testhelpers.GenerateTempStoreFolder()

	infra := setupSharedInfra(testCase)

	// Create go-nitro nodes
	nodeA, _, _, storeA, chainServiceA := setupIntegrationNode(testCase, testCase.Participants[0], infra, []string{}, dataFolder)
	nodeB, _, _, storeB, chainServiceB := setupIntegrationNode(testCase, testCase.Participants[1], infra, []string{}, dataFolder)

	utils := TestUtils{
		tc:            testCase,
		nodeA:         nodeA,
		nodeB:         nodeB,
		chainServiceA: chainServiceA,
		chainServiceB: chainServiceB,
		storeA:        storeA,
		storeB:        storeB,
		infra:         infra,
	}

	cleanup := func() {
		nodeA.Close()
		nodeB.Close()
		removeTempFolder()
		infra.Close(t)
	}

	return utils, cleanup
}

func createMultiAssetLedgerChannel(t *testing.T, utils TestUtils) directfund.ObjectiveResponse {
	outcomeEth := CreateLedgerOutcome(*utils.nodeA.Address, *utils.nodeB.Address, ledgerChannelDeposit, ledgerChannelDeposit+10, common.Address{})

	outcomeCustomToken := CreateLedgerOutcome(*utils.nodeA.Address, *utils.nodeB.Address, ledgerChannelDeposit+20, ledgerChannelDeposit+30, utils.infra.anvilChain.ContractAddresses.TokenAddresses[0])

	outcomeCustomToken2 := CreateLedgerOutcome(*utils.nodeA.Address, *utils.nodeB.Address, ledgerChannelDeposit+40, ledgerChannelDeposit+50, utils.infra.anvilChain.ContractAddresses.TokenAddresses[1])

	multiAssetOutcome := append(outcomeEth, outcomeCustomToken...)
	multiAssetOutcome = append(multiAssetOutcome, outcomeCustomToken2...)

	// Create ledger channel
	ledgerResponse, err := utils.nodeA.CreateLedgerChannel(*utils.nodeB.Address, uint32(utils.tc.ChallengeDuration), multiAssetOutcome)
	if err != nil {
		t.Fatal("error creating ledger channel", err)
	}

	t.Log("Waiting for direct-fund objective to complete...")

	chA := utils.nodeA.ObjectiveCompleteChan(ledgerResponse.Id)
	chB := utils.nodeB.ObjectiveCompleteChan(ledgerResponse.Id)
	<-chA
	<-chB
	t.Logf("Ledger channel %v created", ledgerResponse.ChannelId)
	return ledgerResponse
}

func closeSwapChannel(t *testing.T, utils TestUtils, swapChannelId types.Destination) {
	res, err := utils.nodeA.CloseSwapChannel(swapChannelId)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Started swap-defund objective", "objectiveId", res)

	// Wait for swap-defund objectives to complete
	chA := utils.nodeA.ObjectiveCompleteChan(res)
	chB := utils.nodeB.ObjectiveCompleteChan(res)
	<-chA
	<-chB

	t.Log("Completed swap-defund objective")
}

func closeMultiAssetLedgerChannel(t *testing.T, utils TestUtils, ledgerChannelId types.Destination) {
	res, err := utils.nodeA.CloseLedgerChannel(ledgerChannelId, false)
	if err != nil {
		t.Log(err)
	}

	t.Log("Started direct-defund objective", "objectiveId", res)

	// Wait for direct defund objectives to complete
	chA := utils.nodeA.ObjectiveCompleteChan(res)
	chB := utils.nodeA.ObjectiveCompleteChan(res)
	<-chA
	<-chB

	t.Log("Completed direct-defund objective")
}

func createSwapChannel(t *testing.T, utils TestUtils) swapfund.ObjectiveResponse {
	// TODO: Refactor create swap channel outcome method
	multiassetSwapChannelOutcome := outcome.Exit{
		outcome.SingleAssetExit{
			Asset: common.Address{},
			Allocations: outcome.Allocations{
				outcome.Allocation{
					Destination: types.AddressToDestination(*utils.nodeA.Address),
					Amount:      big.NewInt(int64(1001)),
				},
				outcome.Allocation{
					Destination: types.AddressToDestination(*utils.nodeB.Address),
					Amount:      big.NewInt(int64(1002)),
				},
			},
		},
		outcome.SingleAssetExit{
			Asset: utils.infra.anvilChain.ContractAddresses.TokenAddresses[0],
			Allocations: outcome.Allocations{
				outcome.Allocation{
					Destination: types.AddressToDestination(*utils.nodeA.Address),
					Amount:      big.NewInt(int64(501)),
				},
				outcome.Allocation{
					Destination: types.AddressToDestination(*utils.nodeB.Address),
					Amount:      big.NewInt(int64(502)),
				},
			},
		},
		outcome.SingleAssetExit{
			Asset: utils.infra.anvilChain.ContractAddresses.TokenAddresses[1],
			Allocations: outcome.Allocations{
				outcome.Allocation{
					Destination: types.AddressToDestination(*utils.nodeA.Address),
					Amount:      big.NewInt(int64(601)),
				},
				outcome.Allocation{
					Destination: types.AddressToDestination(*utils.nodeB.Address),
					Amount:      big.NewInt(int64(602)),
				},
			},
		},
	}

	swapChannelresponse, err := utils.nodeA.CreateSwapChannel(
		nil,
		*utils.nodeB.Address,
		0,
		multiassetSwapChannelOutcome,
	)
	if err != nil {
		t.Fatal(err)
	}

	chB := utils.nodeB.ObjectiveCompleteChan(swapChannelresponse.Id)
	<-utils.nodeA.ObjectiveCompleteChan(swapChannelresponse.Id)
	<-chB

	t.Log("Completed swap-fund objective")
	return swapChannelresponse
}

func TestStorageOfLastNSwap(t *testing.T) {
	utils, cleanup := initializeNodesAndInfra(t)
	defer cleanup()

	ledgerChannelResponse := createMultiAssetLedgerChannel(t, utils)
	defer closeMultiAssetLedgerChannel(t, utils, ledgerChannelResponse.ChannelId)

	swapChannelResponse := createSwapChannel(t, utils)
	defer closeSwapChannel(t, utils, swapChannelResponse.ChannelId)

	t.Run("Ensure that only the most recent n swaps are being stored ", func(t *testing.T) {
		swapIterations := 7

		var swapsIds []types.Destination
		for i := 1; i <= swapIterations; i++ {

			// Initiate swap from Bob
			swapAssetResponse, err := utils.nodeB.SwapAssets(swapChannelResponse.ChannelId, common.Address{}, utils.infra.anvilChain.ContractAddresses.TokenAddresses[0], big.NewInt(10), big.NewInt(20))
			if err != nil {
				t.Fatal(err)
			}

			// Wait for objective to wait for confirmation
			time.Sleep(3 * time.Second)

			pendingSwap, err := utils.nodeA.GetPendingSwapByChannelId(swapAssetResponse.ChannelId)
			if err != nil {
				t.Fatal(err)
			}

			// Accept the swap
			err = utils.nodeA.ConfirmSwap(pendingSwap.Id, types.Accepted)
			if err != nil {
				t.Fatal(err)
			}

			<-utils.nodeB.ObjectiveCompleteChan(swapAssetResponse.Id)
			swapsIds = append(swapsIds, pendingSwap.Id)
		}

		storesToTest := []store.Store{utils.storeA, utils.storeB}
		for _, nodeStore := range storesToTest {
			lastNSwaps, err := nodeStore.GetSwapsByChannelId(swapChannelResponse.ChannelId)
			if err != nil {
				t.Fatal(err)
			}

			testhelpers.Assert(t, len(lastNSwaps) == payments.MAX_SWAP_STORAGE_LIMIT, "error in storing last n swap: mismatch in length of channel to swaps map")

			firstSwapIndex := swapIterations - payments.MAX_SWAP_STORAGE_LIMIT
			expectedRemovedSwaps := swapsIds[:firstSwapIndex]
			for _, swap := range lastNSwaps {
				for _, expectedRemovedSwapId := range expectedRemovedSwaps {
					testhelpers.Assert(t, swap.Id != expectedRemovedSwapId, "error in storing last n swap")
				}
			}

			for _, expectedRemovedSwapId := range expectedRemovedSwaps {
				_, err := nodeStore.GetSwapById(expectedRemovedSwapId)
				testhelpers.Assert(t, err == store.ErrNoSuchSwap, "expected swap to be removed from store")
			}
		}
	})
}

func TestParallelSwapCreation(t *testing.T) {
	// Currently parallel swap creations are allowed
	t.Skip()
	utils, cleanup := initializeNodesAndInfra(t)
	defer cleanup()

	ledgerChannelResponse := createMultiAssetLedgerChannel(t, utils)
	defer closeMultiAssetLedgerChannel(t, utils, ledgerChannelResponse.ChannelId)

	swapChannelResponse := createSwapChannel(t, utils)
	defer closeSwapChannel(t, utils, swapChannelResponse.ChannelId)

	t.Run("Ensure parallel swaps are not allowed ", func(t *testing.T) {
		nodes := []node.Node{utils.nodeA, utils.nodeB}

		for i, node := range nodes {
			_, err := node.SwapAssets(swapChannelResponse.ChannelId, common.Address{}, utils.infra.anvilChain.ContractAddresses.TokenAddresses[0], big.NewInt(10), big.NewInt(20))
			if i == 0 {
				continue
			}
			testhelpers.Assert(t, errors.Is(err, swap.ErrSwapExists), "expected error: %v", swap.ErrSwapExists)
		}
	})
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

	outcomeEth := CreateLedgerOutcome(*nodeA.Address, *nodeB.Address, ledgerChannelDeposit, ledgerChannelDeposit+10, common.Address{})
	outcomeCustomToken := CreateLedgerOutcome(*nodeA.Address, *nodeB.Address, ledgerChannelDeposit+20, ledgerChannelDeposit+30, infra.anvilChain.ContractAddresses.TokenAddresses[0])

	outcomeCustomToken2 := CreateLedgerOutcome(*nodeA.Address, *nodeB.Address, ledgerChannelDeposit+40, ledgerChannelDeposit+50, infra.anvilChain.ContractAddresses.TokenAddresses[1])

	multiAssetOutcome := append(outcomeEth, outcomeCustomToken...)
	multiAssetOutcome = append(multiAssetOutcome, outcomeCustomToken2...)

	// Create ledger channel
	ledgerResponse, err := nodeA.CreateLedgerChannel(*nodeB.Address, uint32(testCase.ChallengeDuration), multiAssetOutcome)
	if err != nil {
		t.Fatal("error creating ledger channel", err)
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

	// Initiate swap from Bob
	response1, err := nodeB.SwapAssets(swapChannelresponse.ChannelId, common.Address{}, infra.anvilChain.ContractAddresses.TokenAddresses[0], big.NewInt(100), big.NewInt(200))
	if err != nil {
		t.Fatal(err)
	}

	// Wait for objective to wait for confirmation
	time.Sleep(3 * time.Second)

	pendingSwap1, err := nodeA.GetPendingSwapByChannelId(response1.ChannelId)
	if err != nil {
		t.Fatal(err)
	}

	// Accept the swap
	err = nodeA.ConfirmSwap(pendingSwap1.SwapId(), types.Accepted)
	if err != nil {
		t.Fatal(err)
	}

	<-nodeB.ObjectiveCompleteChan(response1.Id)

	// Check swap channel after accepting swap
	modifiedOutcome1 := modifyOutcomeWithSwap(multiassetSwapChannelOutcome, pendingSwap1, 1)

	checkSwapChannel(t, swapChannelresponse.ChannelId, modifiedOutcome1, query.Open, nodeA, nodeB)

	// Initiate swap from Alice
	response2, err := nodeA.SwapAssets(swapChannelresponse.ChannelId, common.Address{}, infra.anvilChain.ContractAddresses.TokenAddresses[0], big.NewInt(100), big.NewInt(200))
	if err != nil {
		t.Fatal(err)
	}

	// Wait for objective to wait for confirmation
	time.Sleep(3 * time.Second)

	pendingSwap2, err := nodeB.GetPendingSwapByChannelId(response2.ChannelId)
	if err != nil {
		t.Fatal(err)
	}

	// Accept the swap
	err = nodeB.ConfirmSwap(pendingSwap2.SwapId(), types.Accepted)
	if err != nil {
		t.Fatal(err)
	}

	<-nodeA.ObjectiveCompleteChan(response2.Id)

	// Check swap channel after accepting swap
	modifiedOutcome2 := modifyOutcomeWithSwap(modifiedOutcome1, pendingSwap2, 0)

	checkSwapChannel(t, swapChannelresponse.ChannelId, modifiedOutcome2, query.Open, nodeA, nodeB)

	// Initiate swap from Alice
	response3, err := nodeA.SwapAssets(swapChannelresponse.ChannelId, common.Address{}, infra.anvilChain.ContractAddresses.TokenAddresses[0], big.NewInt(100), big.NewInt(200))
	if err != nil {
		t.Fatal(err)
	}

	// Wait for objective to wait for confirmation
	time.Sleep(3 * time.Second)

	pendingSwap3, err := nodeB.GetPendingSwapByChannelId(response3.ChannelId)
	if err != nil {
		t.Fatal(err)
	}

	// Bob Rejects the swap
	err = nodeB.ConfirmSwap(pendingSwap3.SwapId(), types.Rejected)
	if err != nil {
		t.Fatal(err)
	}

	<-nodeA.ObjectiveCompleteChan(response3.Id)

	checkSwapChannel(t, swapChannelresponse.ChannelId, modifiedOutcome2, query.Open, nodeA, nodeB)

	// Initiate another swap from Alice
	response4, err := nodeA.SwapAssets(swapChannelresponse.ChannelId, common.Address{}, infra.anvilChain.ContractAddresses.TokenAddresses[0], big.NewInt(100), big.NewInt(200))
	if err != nil {
		t.Fatal(err)
	}

	// Wait for objective to wait for confirmation
	time.Sleep(3 * time.Second)

	pendingSwap4, err := nodeB.GetPendingSwapByChannelId(response4.ChannelId)
	if err != nil {
		t.Fatal(err)
	}

	// Accept the swap
	err = nodeB.ConfirmSwap(pendingSwap4.SwapId(), types.Accepted)
	if err != nil {
		t.Fatal(err)
	}

	<-nodeA.ObjectiveCompleteChan(response4.Id)

	// Check swap channel after accepting swap
	modifiedOutcome4 := modifyOutcomeWithSwap(modifiedOutcome2, pendingSwap3, 0)
	checkSwapChannel(t, swapChannelresponse.ChannelId, modifiedOutcome4, query.Open, nodeA, nodeB)

	ledgerStateBeforeSdf, err := nodeA.GetSignedState(ledgerResponse.ChannelId)
	if err != nil {
		t.Fatal(err)
	}

	// Close crated swap channel
	res, err := nodeA.CloseSwapChannel(swapChannelresponse.ChannelId)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Started swap-defund objective", "objectiveId", res)

	// Wait for swap-defund objectives to complete
	chA = nodeA.ObjectiveCompleteChan(res)
	chB = nodeB.ObjectiveCompleteChan(res)
	<-chA
	<-chB

	t.Log("Completed swap-defund objective")

	expectedLedgerOutcome := createExpectedLedgerOutcome(ledgerStateBeforeSdf.State().Outcome, modifiedOutcome4)
	checkLedgerChannel(t, ledgerResponse.ChannelId, expectedLedgerOutcome, query.Open, channel.Open, nodeA, nodeB)
}
