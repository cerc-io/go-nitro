package directfund

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/go-cmp/cmp"
	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/consensus_channel"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	"github.com/statechannels/go-nitro/payments"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

const TEST_CHAIN_ID = 1337

var alice, bob testactors.Actor = testactors.Alice, testactors.Bob

var testState = state.State{
	Participants:      []types.Address{alice.Address(), bob.Address()},
	ChannelNonce:      37140676580,
	AppDefinition:     common.HexToAddress(`0x5e29E5Ab8EF33F050c7cc10B5a0456D975C5F88d`),
	ChallengeDuration: 60,
	AppData:           []byte{},
	Outcome: outcome.Exit{
		outcome.SingleAssetExit{
			Asset: types.Address{},
			Allocations: outcome.Allocations{
				outcome.Allocation{
					Destination: bob.Destination(), // Bob is first so we can easily test WaitingForMyTurnToFund
					Amount:      big.NewInt(5),
				},
				outcome.Allocation{
					Destination: alice.Destination(),
					Amount:      big.NewInt(5),
				},
			},
		},
	},
	TurnNum: 0,
	IsFinal: false,
}

// TestNew tests the constructor using a TestState fixture
func TestNew(t *testing.T) {
	getByParticipant := func(id types.Address) ([]*channel.Channel, error) {
		return []*channel.Channel{}, nil
	}
	getByConsensus := func(id types.Address) (*consensus_channel.ConsensusChannel, bool) {
		return nil, false
	}
	request := NewObjectiveRequest(
		testState.Participants[1],
		testState.ChallengeDuration,
		testState.Outcome,
		0,
		testState.AppDefinition,
	)
	// Assert that valid constructor args do not result in error
	if _, err := NewObjective(request, false, testState.Participants[0], big.NewInt(TEST_CHAIN_ID), getByParticipant, getByConsensus); err != nil {
		t.Error(err)
	}

	getByParticipantHasChannel := func(id types.Address) ([]*channel.Channel, error) {
		c, _ := channel.New(testState, 0, types.Ledger)
		return []*channel.Channel{c}, nil
	}

	if _, err := NewObjective(request, false, testState.Participants[0], big.NewInt(TEST_CHAIN_ID), getByParticipantHasChannel, getByConsensus); err == nil {
		t.Errorf("Expected an error when constructing with an objective when an existing channel exists")
	}

	getByConsensusHasChannel := func(id types.Address) (*consensus_channel.ConsensusChannel, bool) {
		return nil, true
	}
	if _, err := NewObjective(request, false, testState.Participants[0], big.NewInt(TEST_CHAIN_ID), getByParticipant, getByConsensusHasChannel); err == nil {
		t.Errorf("Expected an error when constructing with an objective when an existing channel consensus channel exists")
	}
}

func TestConstructFromPayload(t *testing.T) {
	ss := state.NewSignedState(testState)
	id := protocols.ObjectiveId(ObjectivePrefix + testState.ChannelId().String())
	op, err := protocols.CreateObjectivePayload(id, SignedStatePayload, ss)
	testhelpers.Ok(t, err)
	// Assert that valid constructor args do not result in error
	if _, err := ConstructFromPayload(false, op, testState.Participants[0]); err != nil {
		t.Error(err)
	}

	// Construct a final state
	finalState := testState.Clone()
	finalState.IsFinal = true

	op, err = protocols.CreateObjectivePayload(id, SignedStatePayload, state.NewSignedState(finalState))
	testhelpers.Ok(t, err)
	if _, err := ConstructFromPayload(false, op, testState.Participants[0]); err == nil {
		t.Error("expected an error when constructing with an initial state marked final, but got nil")
	}

	nonParticipant := common.HexToAddress("0x5b53f71453aeCb03D837bfe170570d40aE736CB4")
	if _, err := ConstructFromPayload(false, op, nonParticipant); err == nil {
		t.Error("expected an error when constructing with a participant not in the channel, but got nil")
	}
}

func TestUpdate(t *testing.T) {
	id := protocols.ObjectiveId(ObjectivePrefix + testState.ChannelId().String())
	op, err := protocols.CreateObjectivePayload(id, SignedStatePayload, state.NewSignedState(testState))
	testhelpers.Ok(t, err)
	// Construct various variables for use in TestUpdate
	s, _ := ConstructFromPayload(false, op, testState.Participants[0])

	var stateToSign state.State = s.C.PreFundState()
	correctSignatureByParticipant, _ := stateToSign.Sign(alice.PrivateKey)

	// Assert that Updating the objective with such an event returns an error
	// TODO is this the behaviour we want? Below with the signatures, we prefer a log + NOOP (no error)
	op, err = protocols.CreateObjectivePayload("some-id", SignedStatePayload, testState)
	testhelpers.Ok(t, err)

	if _, err := s.Update(op); err == nil {
		t.Error(`ChannelId mismatch -- expected an error but did not get one`)
	}

	// Next, attempt to update the objective with correct signature by a participant on a relevant state
	// Assert that this results in an appropriate change in the extended state of the objective
	ss := state.NewSignedState(stateToSign)
	err = ss.AddSignature(correctSignatureByParticipant)
	if err != nil {
		t.Error(err)
	}

	op, err = protocols.CreateObjectivePayload(s.Id(), SignedStatePayload, ss)
	testhelpers.Ok(t, err)

	updatedObjective, err := s.Update(op)
	if err != nil {
		t.Error(err)
	}
	updated := updatedObjective.(*Objective)
	if updated.C.PreFundSignedByMe() != true {
		t.Error(`Objective data not updated as expected`)
	}

	// Finally, add some Holdings information to the event
	// Updating the objective with this event should overwrite the holdings that are stored
	newFunding := types.Funds{
		common.Address{}: big.NewInt(3),
	}
	highBlockNum := uint64(200)
	_, err = updated.C.UpdateWithChainEvent(
		chainservice.NewDepositedEvent(
			types.Destination{}, chainservice.Block{BlockNum: highBlockNum}, 0, common.Address{}, big.NewInt(3), common.Hash{},
		),
	)
	if err != nil {
		t.Error(err)
	}
	updated = updatedObjective.(*Objective)
	if !updated.C.OnChain.Holdings.Equal(newFunding) {
		t.Error(`Objective data not updated as expected`, updated.C.OnChain.Holdings, newFunding)
	}
}

func compareSideEffect(a, b protocols.SideEffects) string {
	return cmp.Diff(a, b, cmp.AllowUnexported(a, state.SignedState{}, consensus_channel.Add{}, consensus_channel.Guarantee{}, consensus_channel.Remove{}, protocols.Message{}, payments.Voucher{}))
}

func TestCrank(t *testing.T) {
	id := protocols.ObjectiveId(ObjectivePrefix + testState.ChannelId().String())
	op, err := protocols.CreateObjectivePayload(id, SignedStatePayload, state.NewSignedState(testState))
	testhelpers.Ok(t, err)

	// BEGIN test data preparation
	s, _ := ConstructFromPayload(false, op, testState.Participants[0])
	correctSignatureByAliceOnPreFund, _ := s.C.PreFundState().Sign(alice.PrivateKey)
	correctSignatureByBobOnPreFund, _ := s.C.PreFundState().Sign(bob.PrivateKey)

	correctSignatureByAliceOnPostFund, _ := s.C.PostFundState().Sign(alice.PrivateKey)
	correctSignatureByBobOnPostFund, _ := s.C.PostFundState().Sign(bob.PrivateKey)

	// Prepare expected side effects
	preFundSS := state.NewSignedState(s.C.PreFundState())
	_ = preFundSS.AddSignature(correctSignatureByAliceOnPreFund)
	msgs, err := protocols.CreateObjectivePayloadMessage(s.Id(), preFundSS, SignedStatePayload, s.otherParticipants()...)
	testhelpers.Ok(t, err)

	expectedPreFundSideEffects := protocols.SideEffects{MessagesToSend: msgs}

	postFundSS := state.NewSignedState(s.C.PostFundState())
	_ = postFundSS.AddSignature(correctSignatureByAliceOnPostFund)
	msgs, err = protocols.CreateObjectivePayloadMessage(s.Id(), postFundSS, SignedStatePayload, s.otherParticipants()...)
	testhelpers.Ok(t, err)
	expectedPostFundSideEffects := protocols.SideEffects{MessagesToSend: msgs}
	expectedFundingSideEffects := protocols.SideEffects{
		TransactionsToSubmit: []protocols.ChainTransaction{
			protocols.NewDepositTransaction(s.C.Id, types.Funds{
				testState.Outcome[0].Asset: testState.Outcome[0].Allocations[0].Amount,
			}),
		},
	}
	// END test data preparation

	// Assert that cranking an unapproved objective returns an error
	if _, _, _, err := s.Crank(&alice.PrivateKey); err == nil {
		t.Error(`Expected error when cranking unapproved objective, but got nil`)
	}

	// Approve the objective, so that the rest of the test cases can run.
	o := s.Approve().(*Objective)

	// To test the finite state progression, we are going to progressively mutate o
	// And then crank it to see
	//  - which "pause point" (WaitingFor) we end up at,
	//  - what side effects are declared.

	// Initial Crank
	_, sideEffects, waitingFor, err := o.Crank(&alice.PrivateKey)
	if err != nil {
		t.Error(err)
	}
	if waitingFor != WaitingForCompletePrefund {
		t.Fatalf(`WaitingFor: expected %v, got %v`, WaitingForCompletePrefund, waitingFor)
	}

	if diff := compareSideEffect(expectedPreFundSideEffects, sideEffects); diff != "" {
		t.Errorf("Side effects mismatch (-want +got):\n%s", diff)
	}

	// Manually progress the extended state by collecting prefund signatures
	o.C.AddStateWithSignature(o.C.PreFundState(), correctSignatureByAliceOnPreFund)
	o.C.AddStateWithSignature(o.C.PreFundState(), correctSignatureByBobOnPreFund)

	// Cranking should move us to the next waiting point
	_, _, waitingFor, err = o.Crank(&alice.PrivateKey)
	if err != nil {
		t.Error(err)
	}
	if waitingFor != WaitingForMyTurnToFund {
		t.Fatalf(`WaitingFor: expected %v, got %v`, WaitingForMyTurnToFund, waitingFor)
	}

	// Manually make the first "deposit"
	o.C.OnChain.Holdings[testState.Outcome[0].Asset] = testState.Outcome[0].Allocations[0].Amount
	updated, sideEffects, waitingFor, err := o.Crank(&alice.PrivateKey)

	if !updated.(*Objective).transactionSubmitted {
		t.Fatalf("Expected transactionSubmitted flag to be set to true")
	}
	if err != nil {
		t.Error(err)
	}
	if waitingFor != WaitingForCompleteFunding {
		t.Fatalf(`WaitingFor: expected %v, got %v`, WaitingForCompleteFunding, waitingFor)
	}

	if diff := cmp.Diff(expectedFundingSideEffects, sideEffects, cmp.AllowUnexported(expectedFundingSideEffects, protocols.ChainTransactionBase{})); diff != "" {
		t.Fatalf("Side effects mismatch (-want +got):\n%s", diff)
	}

	// Manually make the second "deposit"
	totalAmountAllocated := testState.Outcome[0].TotalAllocated()
	o.C.OnChain.Holdings[testState.Outcome[0].Asset] = totalAmountAllocated
	_, sideEffects, waitingFor, err = o.Crank(&alice.PrivateKey)
	if err != nil {
		t.Error(err)
	}
	if waitingFor != WaitingForCompletePostFund {
		t.Fatalf(`WaitingFor: expected %v, got %v`, WaitingForCompletePostFund, waitingFor)
	}
	if diff := compareSideEffect(expectedPostFundSideEffects, sideEffects); diff != "" {
		t.Errorf("Side effects mismatch (-want +got):\n%s", diff)
	}

	// Manually progress the extended state by collecting postfund signatures
	o.C.AddStateWithSignature(o.C.PostFundState(), correctSignatureByAliceOnPostFund)
	o.C.AddStateWithSignature(o.C.PostFundState(), correctSignatureByBobOnPostFund)

	// This should be the final crank
	o.C.OnChain.Holdings[testState.Outcome[0].Asset] = totalAmountAllocated
	_, _, waitingFor, err = o.Crank(&alice.PrivateKey)
	if err != nil {
		t.Error(err)
	}
	if waitingFor != WaitingForNothing {
		t.Fatalf(`WaitingFor: expected %v, got %v`, WaitingForNothing, waitingFor)
	}
}

func TestClone(t *testing.T) {
	compareObjectives := func(a, b protocols.Objective) string {
		return cmp.Diff(&a, &b, cmp.AllowUnexported(Objective{}, channel.Channel{}, big.Int{}, state.SignedState{}))
	}

	id := protocols.ObjectiveId(ObjectivePrefix + testState.ChannelId().String())
	op, err := protocols.CreateObjectivePayload(id, SignedStatePayload, state.NewSignedState(testState))
	testhelpers.Ok(t, err)

	s, _ := ConstructFromPayload(false, op, testState.Participants[0])

	clone := s.clone()

	if diff := compareObjectives(&s, &clone); diff != "" {
		t.Errorf("Clone: mismatch (-want +got):\n%s", diff)
	}
}

func TestMarshalJSON(t *testing.T) {
	id := protocols.ObjectiveId(ObjectivePrefix + testState.ChannelId().String())
	op, err := protocols.CreateObjectivePayload(id, SignedStatePayload, state.NewSignedState(testState))
	testhelpers.Ok(t, err)

	dfo, _ := ConstructFromPayload(false, op, testState.Participants[0])

	encodedDfo, err := json.Marshal(dfo)
	if err != nil {
		t.Fatalf("error encoding direct-fund objective %v", dfo)
	}

	got := Objective{}
	if err := got.UnmarshalJSON(encodedDfo); err != nil {
		t.Fatalf("error unmarshaling test direct fund objective: %s", err.Error())
	}

	if !got.myDepositSafetyThreshold.Equal(dfo.myDepositSafetyThreshold) {
		t.Fatalf("expected myDepositSafetyThreshhold %v but got %v",
			dfo.myDepositSafetyThreshold, got.myDepositSafetyThreshold)
	}
	if !got.myDepositTarget.Equal(dfo.myDepositTarget) {
		t.Fatalf("expected myDepositTarget %v but got %v",
			dfo.myDepositTarget, got.myDepositTarget)
	}
	if !got.fullyFundedThreshold.Equal(dfo.fullyFundedThreshold) {
		t.Fatalf("expected fullyFundedThreshold %v but got %v",
			dfo.fullyFundedThreshold, got.fullyFundedThreshold)
	}
	if !(got.Status == dfo.Status) {
		t.Fatalf("expected Status %v but got %v", dfo.Status, got.Status)
	}
	if got.C.Id != dfo.C.Id {
		t.Fatalf("expected channel Id %s but got %s", dfo.C.Id, got.C.Id)
	}
}

func TestApproveReject(t *testing.T) {
	id := protocols.ObjectiveId(ObjectivePrefix + testState.ChannelId().String())
	op, err := protocols.CreateObjectivePayload(id, SignedStatePayload, state.NewSignedState(testState))
	testhelpers.Ok(t, err)

	o, err := ConstructFromPayload(false, op, testState.Participants[0])
	testhelpers.Ok(t, err)

	approved := o.Approve()
	if approved.GetStatus() != protocols.Approved {
		t.Errorf("Expected approved status, got %v", approved.GetStatus())
	}
	rejected, sideEffects := o.Reject()
	if rejected.GetStatus() != protocols.Rejected {
		t.Errorf("Expected rejceted status, got %v", approved.GetStatus())
	}
	if len(sideEffects.MessagesToSend) != 1 {
		t.Errorf("Expected to send one message")
	}
}
