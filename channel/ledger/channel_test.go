package ledger

import (
	"errors"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/types"
)

func TestProposals(t *testing.T) {
	var alice = types.Destination(common.HexToHash("0x0a"))
	var bob = types.Destination(common.HexToHash("0x0b"))

	allocation := func(d types.Destination, a uint64) Balance {
		return Balance{destination: d, amount: *big.NewInt(int64(a))}
	}

	guarantee := func(amount uint64, target, left, right types.Destination) Guarantee {
		return Guarantee{
			target: target,
			amount: *big.NewInt(int64(amount)),
			left:   left,
			right:  right,
		}
	}

	makeOutcome := func(left, right Balance, guarantees ...Guarantee) LedgerOutcome {
		mappedGuarantees := make(map[types.Destination]Guarantee)
		for _, g := range guarantees {
			mappedGuarantees[g.target] = g
		}
		return LedgerOutcome{left: left, right: right, guarantees: mappedGuarantees}
	}

	add := func(turnNum, amount uint64, vId, left, right types.Destination) Add {
		return Add{
			turnNum: turnNum,
			Guarantee: Guarantee{
				amount: *big.NewInt(int64(amount)),
				target: vId,
				left:   left,
				right:  right,
			},
		}
	}

	existingChannel := types.Destination{1}
	targetChannel := types.Destination{2}
	aBal := uint64(200)
	bBal := uint64(300)
	vAmount := uint64(5)

	proposal := add(10, vAmount, targetChannel, alice, bob)

	outcome := makeOutcome(
		allocation(alice, aBal),
		allocation(bob, bBal),
		guarantee(vAmount, existingChannel, alice, bob),
	)

	before := Vars{TurnNum: 9, Outcome: outcome}

	after, err := before.Add(proposal)

	if err != nil {
		t.Error("unable to compute next state: ", err)
	}

	if after.TurnNum != before.TurnNum+1 {
		t.Error("incorrect state calculation", err)
	}

	expected := makeOutcome(
		allocation(alice, aBal-vAmount),
		allocation(bob, bBal),
		guarantee(vAmount, existingChannel, alice, bob),
		guarantee(vAmount, targetChannel, alice, bob),
	)

	if !reflect.DeepEqual(after.Outcome, expected) {
		t.Log(after.Outcome)
		t.Log(expected)
		t.Error("incorrect outcome", err)
	}

	largeProposal := proposal
	leftAmount := before.Outcome.left.amount
	largeProposal.amount = *leftAmount.Add(&leftAmount, big.NewInt(1))

	_, err = before.Add(largeProposal)
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Error("expected error when adding too large a guarantee")
	}

	duplicateProposal := proposal
	duplicateProposal.turnNum += 1
	_, err = after.Add(duplicateProposal)

	if !errors.Is(err, ErrDuplicateGuarantee) {
		t.Log(err)
		t.Error("expected error when adding duplicate guarantee")
	}

}
