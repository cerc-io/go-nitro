package swapdefund

import (
	"encoding/json"
	"fmt"

	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/consensus_channel"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

// jsonObjective replaces the virtualfund Objective's channel pointers
// with the channel's respective IDs, making jsonObjective suitable for serialization
type jsonObjective struct {
	Status protocols.ObjectiveStatus
	S      types.Destination

	ToMyLeft  types.Destination
	ToMyRight types.Destination
	MyRole    uint
}

// MarshalJSON returns a JSON representation of the SwapDefundObjective
//
// NOTE: Marshal -> Unmarshal is a lossy process. All channel data
// (other than Id) from the fields ToMyLeft,ToMyRight are discarded
func (o Objective) MarshalJSON() ([]byte, error) {
	var left types.Destination
	var right types.Destination

	if o.ToMyLeft != nil {
		left = o.ToMyLeft.Id
	}

	if o.ToMyRight != nil {
		right = o.ToMyRight.Id
	}

	jsonVFO := jsonObjective{
		Status:    o.Status,
		S:         o.VId(),
		ToMyLeft:  left,
		ToMyRight: right,
		MyRole:    o.MyRole,
	}
	return json.Marshal(jsonVFO)
}

// UnmarshalJSON populates the calling SwapDefundObjective with the
// json-encoded data
//
// NOTE: Marshal -> Unmarshal is a lossy process. All channel data
// (other than Id) from the fields ToMyLeft,ToMyRight are discarded
func (o *Objective) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var jsonVFO jsonObjective
	if err := json.Unmarshal(data, &jsonVFO); err != nil {
		return fmt.Errorf("failed to unmarshal the SwapDefundObjective: %w", err)
	}
	empty := types.Destination{}

	if jsonVFO.ToMyLeft != empty {
		o.ToMyLeft = &consensus_channel.ConsensusChannel{}
		o.ToMyLeft.Id = jsonVFO.ToMyLeft
	}
	if jsonVFO.ToMyRight != empty {
		o.ToMyRight = &consensus_channel.ConsensusChannel{}
		o.ToMyRight.Id = jsonVFO.ToMyRight
	}

	o.Status = jsonVFO.Status

	o.MyRole = jsonVFO.MyRole

	o.S = &channel.SwapChannel{}
	o.S.Id = jsonVFO.S

	return nil
}
