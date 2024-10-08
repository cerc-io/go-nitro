package store_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/go-cmp/cmp"
	"github.com/statechannels/go-nitro/channel"
	cc "github.com/statechannels/go-nitro/channel/consensus_channel"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	nc "github.com/statechannels/go-nitro/crypto"
	ta "github.com/statechannels/go-nitro/internal/testactors"
	td "github.com/statechannels/go-nitro/internal/testdata"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/directfund"
	"github.com/statechannels/go-nitro/protocols/virtualfund"
	"github.com/statechannels/go-nitro/types"
	"github.com/tidwall/buntdb"
)

func compareObjectives(a, b protocols.Objective) string {
	return cmp.Diff(&a, &b, cmp.AllowUnexported(
		directfund.Objective{},
		virtualfund.Objective{},
		channel.Channel{},
		big.Int{},
		state.SignedState{},
		cc.ConsensusChannel{},
		cc.Vars{},
		cc.LedgerOutcome{},
		cc.Balance{},
	))
}

func TestNewMemStore(t *testing.T) {
	sk := common.Hex2Bytes(`2af069c584758f9ec47c4224a8becc1983f28acfbe837bd7710b70f9fc6d5e44`)
	store.NewMemStore(sk)
}

func TestSetGetObjective(t *testing.T) {
	sk := common.Hex2Bytes(`2af069c584758f9ec47c4224a8becc1983f28acfbe837bd7710b70f9fc6d5e44`)

	ms := store.NewMemStore(sk)

	id := protocols.ObjectiveId("404")
	got, err := ms.GetObjectiveById(id)
	if err == nil {
		t.Fatalf("expected not to find the %s objective, but found %v", id, got)
	}

	wants := []protocols.Objective{}
	dfo := td.Objectives.Directfund.GenericDFO()
	vfo := td.Objectives.Virtualfund.GenericVFO()
	wants = append(wants, &dfo)
	wants = append(wants, &vfo)

	for _, want := range wants {

		if err := ms.SetObjective(want); err != nil {
			t.Errorf("error setting objective %v: %s", want, err.Error())
		}

		got, err = ms.GetObjectiveById(want.Id())
		if err != nil {
			t.Errorf("expected to find the inserted objective, but didn't: %s", err)
		}

		if got.Id() != want.Id() {
			t.Errorf("expected to retrieve same objective Id as was passed in, but didn't")
		}

		if diff := compareObjectives(got, want); diff != "" {
			t.Errorf("expected no diff between set and retrieved objective, but found:\n%s", diff)
		}
	}
}

func TestGetObjectiveByChannelId(t *testing.T) {
	sk := common.Hex2Bytes(`2af069c584758f9ec47c4224a8becc1983f28acfbe837bd7710b70f9fc6d5e44`)

	ms := store.NewMemStore(sk)

	dfo := td.Objectives.Directfund.GenericDFO()

	// Store an unapproved objective
	if err := ms.SetObjective(&dfo); err != nil {
		t.Errorf("error setting objective %v: %s", dfo, err.Error())
	}

	_, ok := ms.GetObjectiveByChannelId(dfo.C.Id)
	if ok {
		t.Error("when an unapproved objective is stored, the objective should not own the channel")
	}

	// Now, approve the objective
	dfo.Status = protocols.Approved
	if err := ms.SetObjective(&dfo); err != nil {
		t.Errorf("error setting objective %v: %s", dfo, err.Error())
	}
	got, ok := ms.GetObjectiveByChannelId(dfo.C.Id)

	if !ok {
		t.Errorf("expected to find the inserted objective, but didn't")
	}
	if got.Id() != dfo.Id() {
		t.Errorf("expected to retrieve same objective Id as was passed in, but didn't")
	}
	if diff := compareObjectives(got, &dfo); diff != "" {
		t.Errorf("expected no diff between set and retrieved objective, but found:\n%s", diff)
	}
}

func TestGetChannelSecretKey(t *testing.T) {
	// from state/test-fixtures.go
	sk := common.Hex2Bytes("caab404f975b4620747174a75f08d98b4e5a7053b691b41bcfc0d839d48b7634")
	pk := common.HexToAddress("0xF5A1BB5607C9D079E46d1B3Dc33f257d937b43BD")

	ms := store.NewMemStore(sk)
	key := ms.GetChannelSecretKey()

	msg := []byte("sign this")

	signedMsg, _ := nc.SignEthereumMessage(msg, *key)
	recoveredSigner, _ := nc.RecoverEthereumMessageSigner(msg, signedMsg)

	if recoveredSigner != pk {
		t.Fatalf("expected to recover %x, but got %x", pk, recoveredSigner)
	}
}

func TestConsensusChannelStore(t *testing.T) {
	sk := common.Hex2Bytes(`2af069c584758f9ec47c4224a8becc1983f28acfbe837bd7710b70f9fc6d5e44`)

	ms := store.NewMemStore(sk)

	got, ok := ms.GetConsensusChannel(ta.Alice.Address())
	if ok {
		t.Fatalf("expected not to find the a consensus channel, but found %v", got)
	}

	fp := td.Objectives.Directfund.GenericDFO().C.FixedPart // TODO replace with testdata not nested under GenericDFO
	fp.Participants[0] = ta.Alice.Address()
	fp.Participants[1] = ta.Bob.Address()
	asset := types.Address{}
	left := cc.NewBalance(ta.Alice.Destination(), big.NewInt(6))
	right := cc.NewBalance(ta.Bob.Destination(), big.NewInt(4))

	existingGuarantee := cc.NewGuarantee(big.NewInt(1), types.Destination{1}, left.AsAllocation().Destination, right.AsAllocation().Destination)
	outcome := cc.NewLedgerOutcome(asset, left, right, []cc.Guarantee{existingGuarantee})

	initialVars := cc.Vars{Outcome: *outcome, TurnNum: 0}

	aliceSig, _ := initialVars.AsState(fp).Sign(ta.Alice.PrivateKey)
	bobsSig, _ := initialVars.AsState(fp).Sign(ta.Bob.PrivateKey)

	leader, err := cc.NewLeaderChannel(
		fp,
		0,
		*outcome,
		[2]state.Signature{aliceSig, bobsSig})
	if err != nil {
		t.Fatal(err)
	}

	// Generate a new proposal so we test that the proposal queue is being fetched properly
	proposedGuarantee := cc.NewGuarantee(big.NewInt(1), types.Destination{2}, left.AsAllocation().Destination, right.AsAllocation().Destination)
	proposal := cc.NewAddProposal(leader.Id, proposedGuarantee, big.NewInt(1), common.Address{})
	_, err = leader.Propose(proposal, ta.Alice.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	// The store only deals with ConsensusChannels
	want := leader

	if err := ms.SetConsensusChannel(&want); err != nil {
		t.Fatalf("error setting consensus channel %v: %s", want, err.Error())
	}

	got, ok = ms.GetConsensusChannel(fp.Participants[1])

	if !ok {
		t.Fatalf("expected to find the inserted consensus channel, but didn't")
	}

	if got.Id != want.Id {
		t.Fatalf("expected to retrieve same channel Id as was passed in, but didn't")
	}

	if diff := cmp.Diff(*got, want, cmp.AllowUnexported(cc.ConsensusChannel{}, big.Int{}, cc.LedgerOutcome{}, cc.Balance{}, cc.Guarantee{}, cc.Add{}, cc.Proposal{}, cc.Remove{})); diff != "" {
		t.Fatalf("fetched result different than expected %s", diff)
	}
}

func TestGetChannelsByParticipant(t *testing.T) {
	sk := common.Hex2Bytes(`2af069c584758f9ec47c4224a8becc1983f28acfbe837bd7710b70f9fc6d5e44`)

	ms := store.NewMemStore(sk)
	c := td.Objectives.Directfund.GenericDFO().C
	want := []*channel.Channel{c}
	_ = ms.SetChannel(c)

	got, err := ms.GetChannelsByParticipant(c.Participants[0])
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(got, want, cmp.AllowUnexported(channel.Channel{}, big.Int{}, state.SignedState{})); diff != "" {
		t.Fatalf("fetched result different than expected %s", diff)
	}
}

func TestGetLastBlockNumSeenMemStore(t *testing.T) {
	sk := common.Hex2Bytes(`2af069c584758f9ec47c4224a8becc1983f28acfbe837bd7710b70f9fc6d5e44`)
	ms := store.NewMemStore(sk)

	want := uint64(15)
	_ = ms.SetLastBlockNumSeen(want)

	got, err := ms.GetLastBlockNumSeen()
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatalf("fetched result different than expected %s", diff)
	}
}

func TestGetLastBlockNumSeenDurableStore(t *testing.T) {
	pk := common.Hex2Bytes(`2af069c584758f9ec47c4224a8becc1983f28acfbe837bd7710b70f9fc6d5e44`)

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()
	durableStore, err := store.NewDurableStore(pk, dataFolder, buntdb.Config{})
	if err != nil {
		t.Fatal(err)
	}

	want := uint64(15)
	_ = durableStore.SetLastBlockNumSeen(want)

	got, err := durableStore.GetLastBlockNumSeen()
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatalf("fetched result different than expected %s", diff)
	}
}

func TestBigNumberStorage(t *testing.T) {
	pk := common.Hex2Bytes(`2af069c584758f9ec47c4224a8becc1983f28acfbe837bd7710b70f9fc6d5e44`)

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()
	durableStore, err := store.NewDurableStore(pk, dataFolder, buntdb.Config{})
	if err != nil {
		t.Fatal(err)
	}
	memStore := store.NewMemStore(pk)

	for _, store := range []store.Store{durableStore, memStore} {
		// Set the large amount to 100 * math.MaxInt64
		// 9223372036854775807 * 100 = 922337203685477580700
		largeAmount := big.NewInt(math.MaxInt64)
		largeAmount = largeAmount.Mul(largeAmount, big.NewInt(100))

		reallyLargeOutcome := outcome.Exit{outcome.SingleAssetExit{
			Asset: types.Address{},
			Allocations: outcome.Allocations{
				outcome.Allocation{
					Destination: types.AddressToDestination(ta.Alice.Address()),
					Amount:      big.NewInt(0).Set(largeAmount), // Create a copy of the big.Int in case to avoid any mutation issues
				},
			},
		}}
		s := state.State{Outcome: reallyLargeOutcome}
		c, err := channel.New(s, 0, types.Ledger)
		if err != nil {
			t.Fatalf("error constructing channel: %v", err)
		}

		err = store.SetChannel(c)
		if err != nil {
			t.Fatalf("error setting channel: %v", err)
		}

		got, ok := store.GetChannelById(c.Id)
		if !ok {
			t.Fatalf("expected to find the inserted channel, but didn't")
		}

		//  Check that the *big.Int we get back represents the same value as the one we put in
		if got.PreFundState().Outcome[0].Allocations[0].Amount.Cmp(largeAmount) != 0 {
			t.Fatalf("Expected amount to be %v but got %v", largeAmount, got.PreFundState().Outcome[0].Allocations[0].Amount)
		}
	}
}
