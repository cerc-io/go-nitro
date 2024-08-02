package mirrorbridgeddefund

import (
	"encoding/json"

	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

// jsonObjective replaces the mirrorbridgeddefund.Objective's channel pointer with
// the channel's ID, making jsonObjective suitable for serialization
type jsonObjective struct {
	Status                        protocols.ObjectiveStatus
	C                             types.Destination
	MirrorTransactionSubmitted    bool
	L2SignedState                 state.SignedState
	IsChallenge                   bool
	ChallengeTransactionSubmitted bool
}

// MarshalJSON returns a JSON representation of the MirrorBridgedDefundObjective
// NOTE: Marshal -> Unmarshal is a lossy process. All channel data
// (other than Id) from the field C is discarded
func (o Objective) MarshalJSON() ([]byte, error) {
	jsonDDFO := jsonObjective{
		o.Status,
		o.C.Id,
		o.mirrorTransactionSubmitted,
		o.l2SignedState,
		o.isChallenge,
		o.challengeTransactionSubmitted,
	}

	return json.Marshal(jsonDDFO)
}

// UnmarshalJSON populates the calling MirrorBridgedDefundObjective with the
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
	o.mirrorTransactionSubmitted = jsonDDFO.MirrorTransactionSubmitted
	o.l2SignedState = jsonDDFO.L2SignedState
	o.isChallenge = jsonDDFO.IsChallenge
	o.challengeTransactionSubmitted = jsonDDFO.ChallengeTransactionSubmitted

	return nil
}
