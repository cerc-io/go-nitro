package node_test

import (
	"encoding/json"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
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
	chB := utils.nodeB.ObjectiveCompleteChan(res)
	<-chA
	<-chB

	t.Log("Completed direct-defund objective")
}

func createSwapChannel(t *testing.T, utils TestUtils) (swapfund.ObjectiveResponse, outcome.Exit) {
	// TODO: Refactor create swap channel outcome method
	multiassetSwapChannelOutcome := outcome.Exit{
		outcome.SingleAssetExit{
			Asset: common.Address{},
			Allocations: outcome.Allocations{
				outcome.Allocation{
					Destination: types.AddressToDestination(*utils.nodeA.Address),
					Amount:      big.NewInt(int64(1000)),
				},
				outcome.Allocation{
					Destination: types.AddressToDestination(*utils.nodeB.Address),
					Amount:      big.NewInt(int64(1000)),
				},
			},
		},
		outcome.SingleAssetExit{
			Asset: utils.infra.anvilChain.ContractAddresses.TokenAddresses[0],
			Allocations: outcome.Allocations{
				outcome.Allocation{
					Destination: types.AddressToDestination(*utils.nodeA.Address),
					Amount:      big.NewInt(int64(500)),
				},
				outcome.Allocation{
					Destination: types.AddressToDestination(*utils.nodeB.Address),
					Amount:      big.NewInt(int64(500)),
				},
			},
		},
		outcome.SingleAssetExit{
			Asset: utils.infra.anvilChain.ContractAddresses.TokenAddresses[1],
			Allocations: outcome.Allocations{
				outcome.Allocation{
					Destination: types.AddressToDestination(*utils.nodeA.Address),
					Amount:      big.NewInt(int64(600)),
				},
				outcome.Allocation{
					Destination: types.AddressToDestination(*utils.nodeB.Address),
					Amount:      big.NewInt(int64(600)),
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
	return swapChannelresponse, multiassetSwapChannelOutcome
}

func performSwap(t *testing.T, sender *node.Node, receiver *node.Node, swapSenderIndex int, swapExchange payments.Exchange, swapChannelId types.Destination, expectedInitialOutcome outcome.Exit, action types.SwapStatus) (outcome.Exit, types.Destination, error) {
	swapAssetResponse, err := sender.SwapAssets(swapChannelId, swapExchange.TokenIn, swapExchange.TokenOut, swapExchange.AmountIn, swapExchange.AmountOut)
	if err != nil {
		return outcome.Exit{}, types.Destination{}, err
	}

	// Wait for objective to wait for confirmation
	time.Sleep(3 * time.Second)

	pendingSwap, err := receiver.GetPendingSwapByChannelId(swapAssetResponse.ChannelId)
	if err != nil {
		return outcome.Exit{}, types.Destination{}, err
	}

	// Accept / reject the swap
	err = receiver.ConfirmSwap(pendingSwap.Id, action)
	if err != nil {
		return outcome.Exit{}, types.Destination{}, err
	}

	<-sender.ObjectiveCompleteChan(swapAssetResponse.Id)

	if action == types.Accepted {
		expectedInitialOutcome = modifyOutcomeWithSwap(expectedInitialOutcome, pendingSwap, swapSenderIndex)
	}

	checkSwapChannel(t, swapChannelId, expectedInitialOutcome, query.Open, *sender, *receiver)

	return expectedInitialOutcome, pendingSwap.Id, nil
}

func TestStorageOfLastNSwap(t *testing.T) {
	utils, cleanup := initializeNodesAndInfra(t)
	defer cleanup()

	ledgerChannelResponse := createMultiAssetLedgerChannel(t, utils)
	defer closeMultiAssetLedgerChannel(t, utils, ledgerChannelResponse.ChannelId)

	swapChannelResponse, expectedInitialOutcome := createSwapChannel(t, utils)
	checkSwapChannel(t, swapChannelResponse.ChannelId, expectedInitialOutcome, query.Open, utils.nodeA, utils.nodeB)

	t.Run("Ensure that only the most recent n swaps are being stored ", func(t *testing.T) {
		swapIterations := 7

		var swapsIds []types.Destination
		for i := 1; i <= swapIterations; i++ {

			exchange := payments.Exchange{
				TokenIn:   common.Address{},
				TokenOut:  utils.infra.anvilChain.ContractAddresses.TokenAddresses[0],
				AmountIn:  big.NewInt(10),
				AmountOut: big.NewInt(20),
			}

			out, swapId, err := performSwap(t, &utils.nodeB, &utils.nodeA, 1, exchange, swapChannelResponse.ChannelId, expectedInitialOutcome, types.Accepted)
			if err != nil {
				t.Fatal(err)
			}

			expectedInitialOutcome = out
			swapsIds = append(swapsIds, swapId)
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

	t.Run("Check ledger channel after swapdefund", func(t *testing.T) {
		ledgerStateBeforeSdf, err := utils.nodeA.GetSignedState(ledgerChannelResponse.ChannelId)
		if err != nil {
			t.Fatal(err)
		}

		closeSwapChannel(t, utils, swapChannelResponse.ChannelId)

		expectedLedgerOutcome := createExpectedLedgerOutcome(ledgerStateBeforeSdf.State().Outcome, expectedInitialOutcome)
		checkLedgerChannel(t, ledgerChannelResponse.ChannelId, expectedLedgerOutcome, query.Open, channel.Open, utils.nodeA, utils.nodeB)
	})
}

func TestParallelSwapCreation(t *testing.T) {
	// Currently parallel swap creations are allowed
	t.Skip()
	utils, cleanup := initializeNodesAndInfra(t)
	defer cleanup()

	ledgerChannelResponse := createMultiAssetLedgerChannel(t, utils)
	defer closeMultiAssetLedgerChannel(t, utils, ledgerChannelResponse.ChannelId)

	swapChannelResponse, _ := createSwapChannel(t, utils)
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
	utils, cleanup := initializeNodesAndInfra(t)
	defer cleanup()

	ledgerChannelResponse := createMultiAssetLedgerChannel(t, utils)
	defer closeMultiAssetLedgerChannel(t, utils, ledgerChannelResponse.ChannelId)

	swapChannelResponse, expectedInitialOutcome := createSwapChannel(t, utils)
	checkSwapChannel(t, swapChannelResponse.ChannelId, expectedInitialOutcome, query.Open, utils.nodeA, utils.nodeB)

	t.Run("Test multiple swaps from both nodes", func(t *testing.T) {
		exchange := payments.Exchange{
			TokenIn:   common.Address{},
			TokenOut:  utils.infra.anvilChain.ContractAddresses.TokenAddresses[0],
			AmountIn:  big.NewInt(20),
			AmountOut: big.NewInt(10),
		}

		// Alice initiates swap and Bob accepts
		out, _, err := performSwap(t, &utils.nodeA, &utils.nodeB, 0, exchange, swapChannelResponse.ChannelId, expectedInitialOutcome, types.Accepted)
		if err != nil {
			t.Fatal(err)
		}

		expectedInitialOutcome = out

		// Bob initiates swap and Alice accepts
		out, _, err = performSwap(t, &utils.nodeB, &utils.nodeA, 1, exchange, swapChannelResponse.ChannelId, expectedInitialOutcome, types.Accepted)
		if err != nil {
			t.Fatal(err)
		}

		expectedInitialOutcome = out

		// Alice initiates swap and Bob rejects
		out, _, err = performSwap(t, &utils.nodeA, &utils.nodeB, 0, exchange, swapChannelResponse.ChannelId, expectedInitialOutcome, types.Rejected)
		if err != nil {
			t.Fatal(err)
		}

		expectedInitialOutcome = out

		// Alice initiates swap and Bob accepts
		out, _, err = performSwap(t, &utils.nodeA, &utils.nodeB, 0, exchange, swapChannelResponse.ChannelId, expectedInitialOutcome, types.Accepted)
		if err != nil {
			t.Fatal(err)
		}

		expectedInitialOutcome = out
	})

	t.Run("Check ledger channel after swapdefund", func(t *testing.T) {
		ledgerStateBeforeSdf, err := utils.nodeA.GetSignedState(ledgerChannelResponse.ChannelId)
		if err != nil {
			t.Fatal(err)
		}

		closeSwapChannel(t, utils, swapChannelResponse.ChannelId)

		expectedLedgerOutcome := createExpectedLedgerOutcome(ledgerStateBeforeSdf.State().Outcome, expectedInitialOutcome)
		checkLedgerChannel(t, ledgerChannelResponse.ChannelId, expectedLedgerOutcome, query.Open, channel.Open, utils.nodeA, utils.nodeB)
	})
}

func TestSwapTillEmptyBalance(t *testing.T) {
	utils, cleanup := initializeNodesAndInfra(t)
	defer cleanup()

	ledgerChannelResponse := createMultiAssetLedgerChannel(t, utils)
	defer closeMultiAssetLedgerChannel(t, utils, ledgerChannelResponse.ChannelId)

	swapChannelResponse, expectedInitialOutcome := createSwapChannel(t, utils)
	checkSwapChannel(t, swapChannelResponse.ChannelId, expectedInitialOutcome, query.Open, utils.nodeA, utils.nodeB)

	t.Run("Test performing swaps until balance becomes zero", func(t *testing.T) {
	bobSwapLoop:
		for {
			exchange := payments.Exchange{
				TokenIn:   common.Address{},
				TokenOut:  utils.infra.anvilChain.ContractAddresses.TokenAddresses[0],
				AmountIn:  big.NewInt(100),
				AmountOut: big.NewInt(100),
			}

			out, _, err := performSwap(t, &utils.nodeB, &utils.nodeA, 1, exchange, swapChannelResponse.ChannelId, expectedInitialOutcome, types.Accepted)
			if err != nil {
				// Check that balance of node A is zero now that swap has failed
				var swapInfo query.SwapChannelInfo
				marshalledSwapInfo, er := utils.nodeA.GetSwapChannel(swapChannelResponse.ChannelId)
				if er != nil {
					t.Fatal(err)
				}

				er = json.Unmarshal([]byte(marshalledSwapInfo), &swapInfo)
				if er != nil {
					t.Fatal(er)
				}

				var balanceNodeA *hexutil.Big

				for _, b := range swapInfo.Balances {
					if b.AssetAddress == utils.infra.anvilChain.ContractAddresses.TokenAddresses[0] {
						balanceNodeA = b.MyBalance
					}
				}

				testhelpers.Assert(t, err == swap.ErrInvalidSwap, "Incorrect error")
				testhelpers.Assert(t, types.IsZero((*big.Int)(balanceNodeA)), "Balance of node A should be zero")

				break bobSwapLoop
			}

			expectedInitialOutcome = out
		}

	aliceSwapLoop:
		for {
			exchange := payments.Exchange{
				TokenIn:   common.Address{},
				TokenOut:  utils.infra.anvilChain.ContractAddresses.TokenAddresses[0],
				AmountIn:  big.NewInt(100),
				AmountOut: big.NewInt(100),
			}

			out, _, err := performSwap(t, &utils.nodeA, &utils.nodeB, 0, exchange, swapChannelResponse.ChannelId, expectedInitialOutcome, types.Accepted)
			if err != nil {
				// Check that balance of node B is zero now that swap has failed
				var swapInfo query.SwapChannelInfo
				marshalledSwapInfo, er := utils.nodeB.GetSwapChannel(swapChannelResponse.ChannelId)
				if er != nil {
					t.Fatal(err)
				}

				er = json.Unmarshal([]byte(marshalledSwapInfo), &swapInfo)
				if er != nil {
					t.Fatal(er)
				}

				var balanceNodeB *hexutil.Big

				for _, b := range swapInfo.Balances {
					if b.AssetAddress == utils.infra.anvilChain.ContractAddresses.TokenAddresses[0] {
						balanceNodeB = b.MyBalance
					}
				}

				testhelpers.Assert(t, err == swap.ErrInvalidSwap, "Incorrect error")
				testhelpers.Assert(t, types.IsZero((*big.Int)(balanceNodeB)), "Balance of node B should be zero")

				break aliceSwapLoop
			}

			expectedInitialOutcome = out
		}
	})

	t.Run("Check ledger channel after swapdefund", func(t *testing.T) {
		ledgerStateBeforeSdf, err := utils.nodeA.GetSignedState(ledgerChannelResponse.ChannelId)
		if err != nil {
			t.Fatal(err)
		}

		closeSwapChannel(t, utils, swapChannelResponse.ChannelId)

		expectedLedgerOutcome := createExpectedLedgerOutcome(ledgerStateBeforeSdf.State().Outcome, expectedInitialOutcome)
		checkLedgerChannel(t, ledgerChannelResponse.ChannelId, expectedLedgerOutcome, query.Open, channel.Open, utils.nodeA, utils.nodeB)
	})
}
