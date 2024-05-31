package node_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/protocols/directdefund"
	"github.com/statechannels/go-nitro/types"
)

func TestDirectDefundWithChallenge(t *testing.T) {
	testCase := TestCase{
		Description:       "Direct defund with Challenge",
		Chain:             AnvilChain,
		MessageService:    TestMessageService,
		ChallengeDuration: 10,
		LogName:           "challenge_integration",
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
		},
	}

	dataFolder, cleanup := testhelpers.GenerateTempStoreFolder()
	defer cleanup()

	infra := setupSharedInfra(testCase)
	defer infra.Close(t)

	nodeA, _, _, storeA, _ := setupIntegrationNode(testCase, testCase.Participants[0], infra, []string{}, dataFolder)
	defer nodeA.Close()
	nodeB, _, _, storeB, _ := setupIntegrationNode(testCase, testCase.Participants[1], infra, []string{}, dataFolder)
	defer nodeB.Close()

	ledgerChannel := openLedgerChannel(t, nodeA, nodeB, types.Address{}, uint32(testCase.ChallengeDuration))
	response, err := nodeA.CloseLedgerChannel(ledgerChannel, true)
	if err != nil {
		t.Log(err)
	}

	time.Sleep(5 * time.Second)
	objectiveA, _ := storeA.GetObjectiveByChannelId(ledgerChannel)
	objectiveB, _ := storeB.GetObjectiveByChannelId(ledgerChannel)
	objA, _ := objectiveA.(*directdefund.Objective)
	objB, _ := objectiveB.(*directdefund.Objective)

	testhelpers.Assert(t, objA.C.GetChannelStatus() == channel.Challenge, "Expected channel status to be challenge")
	testhelpers.Assert(t, objB.C.GetChannelStatus() == channel.Challenge, "Expected channel status to be challenge")

	<-nodeA.ObjectiveCompleteChan(response)

	// Check assets are liquidated
	balanceNodeA, _ := infra.anvilChain.GetAccountBalance(testCase.Participants[0].Address())
	balanceNodeB, _ := infra.anvilChain.GetAccountBalance(testCase.Participants[1].Address())
	t.Log("Balance of Alice", balanceNodeA, "\nBalance of Bob", balanceNodeB)
	// Assert balance equals ledger channel deposit since no payment has been made
	testhelpers.Assert(t, balanceNodeA.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "Balance of Alice (%v) should be equal to ledgerChannelDeposit (%v)", balanceNodeA, ledgerChannelDeposit)
	testhelpers.Assert(t, balanceNodeB.Cmp(big.NewInt(ledgerChannelDeposit)) == 0, "Balance of Bob (%v) should be equal to ledgerChannelDeposit (%v)", balanceNodeB, ledgerChannelDeposit)
}
