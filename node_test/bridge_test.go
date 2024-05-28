package node_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/bridgedfund"
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
	defer nodeBPrime.Close()

	nodeAPrime, _, _, _, _ := setupIntegrationNode(tc, tc.Participants[3], infra, []string{}, dataFolder)
	defer nodeAPrime.Close()

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

	nodeBPrimeAllocation := l1ledgerChannelStateClone.State().Outcome[0].Allocations[1]
	nodeBPrimeAllocation.Destination = types.AddressToDestination(*nodeBPrime.Address)

	nodeAPrimeAllocation := l1ledgerChannelStateClone.State().Outcome[0].Allocations[0]
	nodeAPrimeAllocation.Destination = types.AddressToDestination(*nodeAPrime.Address)

	// Create extended state based on l1ChannelState
	l2ChannelOutcome := outcome.Exit{
		{
			Asset:         l1ledgerChannelStateClone.State().Outcome[0].Asset,
			AssetMetadata: l1ledgerChannelStateClone.State().Outcome[0].AssetMetadata,
			Allocations: outcome.Allocations{
				nodeAPrimeAllocation,
				nodeBPrimeAllocation,
			},
		},
	}

	l2ChannelState := state.State{
		Participants:      []common.Address{*nodeBPrime.Address, *nodeAPrime.Address},
		ChannelNonce:      state.TestState.ChannelNonce,
		AppDefinition:     l1ledgerChannelStateClone.State().AppDefinition,
		ChallengeDuration: l1ledgerChannelStateClone.State().ChallengeDuration,
		AppData:           l1ledgerChannelStateClone.State().AppData,
		Outcome:           l2ChannelOutcome,
		TurnNum:           0,
		IsFinal:           l1ledgerChannelStateClone.State().IsFinal,
	}

	// Test Crank method of bridgedfund protocol
	id := protocols.ObjectiveId(bridgedfund.ObjectivePrefix + l2ChannelState.ChannelId().String())
	op, err := protocols.CreateObjectivePayload(id, bridgedfund.SignedStatePayload, state.NewSignedState(l2ChannelState))
	if err != nil {
		t.Error(err)
	}

	s, err := bridgedfund.ConstructFromPayload(false, op, l2ChannelState.Participants[0])

	if err != nil {
		t.Error(err)
	}

	preFundSignatureBPrime, _ := s.C.PreFundState().Sign(tc.Participants[2].PrivateKey)
	preFundSignatureAprime, _ := s.C.PreFundState().Sign(tc.Participants[3].PrivateKey)

	postFundSignatureBprime, _ := s.C.PostFundState().Sign(tc.Participants[2].PrivateKey)
	postFundSignatureAprime, _ := s.C.PostFundState().Sign(tc.Participants[3].PrivateKey)

	o := s.Approve().(*bridgedfund.Objective)

	// Initial Crank
	_, _, waitingFor, err := o.Crank(&tc.Participants[2].PrivateKey)
	if err != nil {
		t.Error(err)
	}

	if waitingFor != bridgedfund.WaitingForCompletePrefund {
		t.Fatalf(`WaitingFor: expected %v, got %v`, bridgedfund.WaitingForCompletePrefund, waitingFor)
	}

	// Manually progress the extended state by collecting prefund signatures
	o.C.AddStateWithSignature(o.C.PreFundState(), preFundSignatureBPrime)
	o.C.AddStateWithSignature(o.C.PreFundState(), preFundSignatureAprime)

	// Cranking should move us to the next waiting point
	_, _, waitingFor, err = o.Crank(&tc.Participants[2].PrivateKey)
	if err != nil {
		t.Error(err)
	}

	if waitingFor != bridgedfund.WaitingForMyTurnToFund {
		t.Fatalf(`WaitingFor: expected %v, got %v`, bridgedfund.WaitingForMyTurnToFund, waitingFor)
	}

	// Manually make the first "deposit"
	o.C.OnChain.Holdings[l2ChannelState.Outcome[0].Asset] = l2ChannelState.Outcome[0].Allocations[0].Amount
	_, _, waitingFor, err = o.Crank(&tc.Participants[2].PrivateKey)
	if err != nil {
		t.Error(err)
	}
	if waitingFor != bridgedfund.WaitingForCompleteFunding {
		t.Fatalf(`WaitingFor: expected %v, got %v`, bridgedfund.WaitingForCompleteFunding, waitingFor)
	}

	// Manually make the second "deposit"
	totalAmountAllocated := l2ChannelState.Outcome[0].TotalAllocated()
	o.C.OnChain.Holdings[l2ChannelState.Outcome[0].Asset] = totalAmountAllocated
	_, _, waitingFor, err = o.Crank(&tc.Participants[2].PrivateKey)
	if err != nil {
		t.Error(err)
	}
	if waitingFor != bridgedfund.WaitingForCompletePostFund {
		t.Fatalf(`WaitingFor: expected %v, got %v`, bridgedfund.WaitingForCompletePostFund, waitingFor)
	}

	// Manually progress the extended state by collecting postfund signatures
	o.C.AddStateWithSignature(o.C.PostFundState(), postFundSignatureBprime)
	o.C.AddStateWithSignature(o.C.PostFundState(), postFundSignatureAprime)

	// This should be the final crank
	o.C.OnChain.Holdings[l2ChannelState.Outcome[0].Asset] = totalAmountAllocated
	_, _, waitingFor, err = o.Crank(&tc.Participants[2].PrivateKey)
	if err != nil {
		t.Error(err)
	}
	if waitingFor != bridgedfund.WaitingForNothing {
		t.Fatalf(`WaitingFor: expected %v, got %v`, bridgedfund.WaitingForNothing, waitingFor)
	}
}
