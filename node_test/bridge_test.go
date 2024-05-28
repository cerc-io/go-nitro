package node_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/directfund"
	"github.com/statechannels/go-nitro/types"
)

func TestBridge(t *testing.T) {
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

	infra := setupSharedInfra(tc)
	defer infra.Close(t)

	// Create go-nitro nodes
	nodeA, _, _, storeA, _ := setupIntegrationNode(tc, tc.Participants[0], infra, []string{}, dataFolder)
	defer nodeA.Close()

	nodeB, _, _, _, _ := setupIntegrationNode(tc, tc.Participants[1], infra, []string{}, dataFolder)
	defer nodeB.Close()

	nodeBPrime, _, _, _, _ := setupIntegrationNode(tc, tc.Participants[2], infra, []string{}, dataFolder)
	defer nodeA.Close()

	nodeAPrime, _, _, _, _ := setupIntegrationNode(tc, tc.Participants[3], infra, []string{}, dataFolder)

	// Separate chain service to listen for events
	testChainServiceA := setupChainService(tc, tc.Participants[0], infra)
	defer testChainServiceA.Close()

	// Create ledger channel
	l1LedgerChannelId := openLedgerChannel(t, nodeA, nodeB, types.Address{}, uint32(tc.ChallengeDuration))

	l1LedgerChannel, err := storeA.GetConsensusChannelById(l1LedgerChannelId)
	if err != nil {
		t.Error(err)
	}

	l1ledgerChannelState := l1LedgerChannel.SupportedSignedState()

	l1ledgerChannelStateClone := l1ledgerChannelState.Clone()

	nodeBPrimeAllocation := l1ledgerChannelStateClone.State().Outcome[0].Allocations[0]
	nodeBPrimeAllocation.Destination = types.AddressToDestination(*nodeBPrime.Address)

	nodeAPrimeAllocation := l1ledgerChannelStateClone.State().Outcome[1].Allocations[0]
	nodeAPrimeAllocation.Destination = types.AddressToDestination(*nodeBPrime.Address)

	// Create extended state based on l1ChannelState
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

	// 1. Create state reflecting ledger channel state on l1
	l2ChannelState := state.State{
		Participants:      []common.Address{*nodeBPrime.Address, *nodeAPrime.Address},
		ChannelNonce:      state.TestState.ChannelNonce,
		AppDefinition:     l1ledgerChannelStateClone.State().AppDefinition,
		ChallengeDuration: l1ledgerChannelStateClone.State().ChallengeDuration,
		AppData:           l1ledgerChannelStateClone.State().AppData,
		Outcome:           l2ChannelOutcome,
		TurnNum:           l1ledgerChannelStateClone.State().TurnNum,
		IsFinal:           l1ledgerChannelStateClone.State().IsFinal,
	}

	// 2. Contruct objective using it and progress by cranking it
	id := protocols.ObjectiveId(directfund.ObjectivePrefix + l2ChannelState.ChannelId().String())
	op, err := protocols.CreateObjectivePayload(id, directfund.SignedStatePayload, state.NewSignedState(l2ChannelState))

	s, _ := directfund.ConstructFromPayload(false, op, l2ChannelState.Participants[0])

	o := s.Approve().(*directfund.Objective)
	// 3. TODO: Create `bridgedfund` protocol and use it to crank the constructed objective
}
