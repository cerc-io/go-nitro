package node_test

import (
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
		ChallengeDuration: 30,
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
	_, err := nodeA.CloseLedgerChannel(ledgerChannel, true)
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
}
