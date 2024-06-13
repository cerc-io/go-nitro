package directdefund

import (
	"encoding/json"

	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

// jsonObjective replaces the directdefund.Objective's channel pointer with
// the channel's ID, making jsonObjective suitable for serialization
type jsonObjective struct {
	Status                         protocols.ObjectiveStatus
	C                              types.Destination
	FinalTurnNum                   uint64
	TransactionSumbmitted          bool
	IsChallenge                    bool
	ChallengeTransactionSubmitted  bool
	IsCheckpoint                   bool
	CheckpointTransactionSubmitted bool
	LatestBlockTime                uint64
}

// MarshalJSON returns a JSON representation of the DirectDefundObjective
// NOTE: Marshal -> Unmarshal is a lossy process. All channel data
// (other than Id) from the field C is discarded
func (o Objective) MarshalJSON() ([]byte, error) {
	jsonDDFO := jsonObjective{
		o.Status,
		o.C.Id,
		o.finalTurnNum,
		o.withdrawTransactionSubmitted,
		o.IsChallenge,
		o.challengeTransactionSubmitted,
		o.checkpointTransactionSubmitted,
		o.IsCheckpoint,
		o.LatestBlockTime,
	}

	return json.Marshal(jsonDDFO)
}

// UnmarshalJSON populates the calling DirectDefundObjective with the
// json-encoded data
// NOTE: Marshal -> Unmarshal is a lossy process. All channel data
// (other than Id) from the field C is discarded
func (o *Objective) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var jsonDDFO jsonObjective
	err := json.Unmarshal(data, &jsonDDFO)
	if err != nil {
		return err
	}

	o.C = &channel.Channel{}

	o.Status = jsonDDFO.Status
	o.C.Id = jsonDDFO.C
	o.finalTurnNum = jsonDDFO.FinalTurnNum
	o.withdrawTransactionSubmitted = jsonDDFO.TransactionSumbmitted
	o.IsChallenge = jsonDDFO.IsChallenge
	o.challengeTransactionSubmitted = jsonDDFO.ChallengeTransactionSubmitted
	o.checkpointTransactionSubmitted = jsonDDFO.CheckpointTransactionSubmitted
	o.IsCheckpoint = jsonDDFO.IsCheckpoint
	o.LatestBlockTime = jsonDDFO.LatestBlockTime

	return nil
}
