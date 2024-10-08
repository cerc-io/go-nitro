package consensus_channel

import (
	"errors"
	"math/big"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/types"
)

func TestConsensusChannel(t *testing.T) {
	existingChannel := types.Destination{1}

	proposal := add(vAmount, targetChannel, alice, bob)

	outcome := func() LedgerOutcomes {
		return LedgerOutcomes{makeOutcome(
			allocation(alice, aBal),
			allocation(bob, bBal),
			guarantee(vAmount, existingChannel, alice, bob),
		)}
	}

	fingerprint := func(v Vars) string {
		h, err := v.AsState(state.TestState.FixedPart()).Hash()
		if err != nil {
			panic(err)
		}
		return h.String()
	}

	vars := Vars{TurnNum: 9, Outcome: outcome()}

	f1 := fingerprint(vars)
	// Assume only one asset
	clone1 := vars.Outcome[0].clone()

	if fingerprint(Vars{TurnNum: vars.TurnNum, Outcome: LedgerOutcomes{clone1}}) != f1 {
		t.Fatal("vars incorrectly cloned")
	}

	mutatedG := clone1.guarantees[existingChannel]
	mutatedG.amount.SetInt64(111)
	if f1 != fingerprint(vars) {
		t.Fatal("vars shares data with clone")
	}

	clone2 := vars.Outcome[0].clone()
	clone2.leader.amount.SetInt64(111)
	if f1 != fingerprint(vars) {
		t.Fatal("vars shares data with clone")
	}

	clone3 := vars.Outcome[0].clone()
	clone3.follower.amount.SetInt64(111)
	if f1 != fingerprint(vars) {
		t.Fatal("vars shares data with clone")
	}

	testApplyingAddProposalToVars := func(t *testing.T) {
		startingTurnNum := uint64(9)
		vars := Vars{TurnNum: startingTurnNum, Outcome: outcome()}

		err := vars.Add(proposal)
		if err != nil {
			t.Fatalf("unable to compute next state: %v", err)
		}

		if vars.TurnNum != startingTurnNum+1 {
			t.Fatalf("incorrect state calculation: %v", err)
		}

		expected := makeOutcome(
			allocation(alice, aBal-vAmount),
			allocation(bob, bBal),
			guarantee(vAmount, existingChannel, alice, bob),
			guarantee(vAmount, targetChannel, alice, bob),
		)

		if diff := cmp.Diff(vars.Outcome, expected, cmp.AllowUnexported(expected, Balance{}, big.Int{}, Guarantee{})); diff != "" {
			t.Fatalf("incorrect outcome: %v", diff)
		}

		// Proposing the same change again should fail
		duplicateProposal := proposal
		err = vars.Add(duplicateProposal)

		if !errors.Is(err, ErrDuplicateGuarantee) {
			t.Fatalf("expected error when adding duplicate guarantee: %v", err)
		}

		// Proposing a change that depletes a balance should fail
		vars = Vars{TurnNum: startingTurnNum, Outcome: outcome()}
		largeProposal := proposal
		leftAmount := big.NewInt(0).Set(vars.Outcome[0].leader.amount)
		largeProposal.amount = leftAmount.Add(leftAmount, big.NewInt(1))
		largeProposal.LeftDeposit = largeProposal.amount
		err = vars.Add(largeProposal)

		if !errors.Is(err, ErrInsufficientFunds) {
			t.Fatalf("expected error when adding too large a guarantee: %v", err)
		}
	}

	testApplyingRemoveProposalToVars := func(t *testing.T) {
		startingTurnNum := uint64(9)

		vars := Vars{TurnNum: startingTurnNum, Outcome: outcome()}
		aAmount, bAmount := uint64(2), uint64(3)
		proposal := remove(existingChannel, aAmount)
		err := vars.Remove(proposal)
		if err != nil {
			t.Fatalf("unable to compute next state: %v", err)
		}

		if vars.TurnNum != startingTurnNum+1 {
			t.Fatalf("incorrect state calculation: %v", err)
		}

		expected := makeOutcome(
			allocation(alice, aBal+aAmount),
			allocation(bob, bBal+bAmount),
		)

		if diff := cmp.Diff(vars.Outcome, expected, cmp.AllowUnexported(expected, Balance{}, big.Int{}, Guarantee{})); diff != "" {
			t.Fatalf("incorrect outcome: %v", diff)
		}

		// Proposing the same change again should fail since the guarantee has been removed
		duplicateProposal := proposal

		err = vars.Remove(duplicateProposal)

		if !errors.Is(err, ErrGuaranteeNotFound) {
			t.Fatalf("expected error when adding duplicate guarantee: %v", err)
		}

		// Proposing a remove that cannot be afforded by the guarantee should fail
		vars = Vars{TurnNum: startingTurnNum, Outcome: outcome()}
		largeProposal := Remove{
			Target:     existingChannel,
			LeftAmount: big.NewInt(10),
		}
		err = vars.Remove(largeProposal)
		if !errors.Is(err, ErrInvalidAmount) {
			t.Fatalf("expected error when recovering too large much from a guarantee: %v", err)
		}
	}

	initialVars := Vars{Outcome: outcome(), TurnNum: 0}
	aliceSig, _ := initialVars.AsState(fp()).Sign(alice.PrivateKey)
	bobsSig, _ := initialVars.AsState(fp()).Sign(bob.PrivateKey)
	sigs := [2]state.Signature{aliceSig, bobsSig}

	testConsensusChannelFunctionality := func(t *testing.T) {
		channel, err := newConsensusChannel(fp(), Leader, 0, outcome(), sigs)
		if err != nil {
			t.Fatalf("unable to construct a new consensus channel: %v", err)
		}

		_, err = channel.sign(initialVars, bob.PrivateKey)
		if err == nil {
			t.Fatalf("channel should check that signer is participant")
		}

		f := fingerprint(channel.current.Vars)

		latest, err := channel.latestProposedVars()
		if err != nil {
			t.Fatalf("latest proposed vars returned err: %v", err)
		}

		latest.Outcome[0].guarantees[targetChannel] = guarantee(10, targetChannel, alice, bob)
		if f != fingerprint(channel.current.Vars) {
			t.Fatalf("latestProposedVars did not return a copy")
		}

		ivansSig, _ := initialVars.AsState(fp()).Sign(ivan.PrivateKey)
		wrongSigs := [2]state.Signature{sigs[1], ivansSig}
		_, err = newConsensusChannel(fp(), Leader, 0, outcome(), wrongSigs)
		if err == nil {
			t.Fatalf("channel should check that signers are participants")
		}
	}

	testEmptyProposalClone := func(t *testing.T) {
		add := Add{}
		clonedAdd := add.Clone()
		if !reflect.DeepEqual(add, clonedAdd) {
			t.Fatalf("cloned add is not equal to original")
		}
		remove := Remove{}
		clonedRemove := remove.Clone()

		if !reflect.DeepEqual(remove, clonedRemove) {
			t.Fatalf("cloned remove is not equal to original")
		}
	}
	t.Run(`TestEmptyProposalClone`, testEmptyProposalClone)
	t.Run(`TestApplyingAddProposalToVars`, testApplyingAddProposalToVars)
	t.Run(`TestApplyingRemoveProposalToVars`, testApplyingRemoveProposalToVars)
	t.Run(`TestConsensusChannelFunctionality`, testConsensusChannelFunctionality)
}
