package protocols

import (
	"fmt"

	"github.com/statechannels/go-nitro/channel/consensus_channel"
	"github.com/statechannels/go-nitro/channel/state/outcome"
)

func FromExitOutcomeArr(assetExit outcome.Exit) ([]consensus_channel.LedgerOutcome, error) {
	var outcomeArr []consensus_channel.LedgerOutcome

	for _, sae := range assetExit {
		outcome, err := consensus_channel.FromExit(sae)
		if err != nil {
			return []consensus_channel.LedgerOutcome{}, fmt.Errorf("could not create ledger outcome from channel exit: %w", err)
		}

		outcomeArr = append(outcomeArr, outcome)
	}

	return outcomeArr, nil
}
