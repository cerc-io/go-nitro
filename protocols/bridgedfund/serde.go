package bridgedfund

import (
	"encoding/json"

	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

// jsonObjective replaces the directfund.Objective's channel pointer with the
// channel's ID, making jsonObjective suitable for serialization
type jsonObjective struct {
	Id     protocols.ObjectiveId
	Status protocols.ObjectiveStatus
	C      types.Destination
}

// MarshalJSON returns a JSON representation of the DirectFundObjective
// NOTE: Marshal -> Unmarshal is a lossy process. All channel data
// (other than Id) from the field C is discarded
func (o Objective) MarshalJSON() ([]byte, error) {
	jsonDFO := jsonObjective{
		o.Id(),
		o.Status,
		o.C.Id,
	}
	return json.Marshal(jsonDFO)
}

// UnmarshalJSON populates the calling DirectFundObjective with the
// json-encoded data
// NOTE: Marshal -> Unmarshal is a lossy process. All channel data
// (other than Id) from the field C is discarded
func (o *Objective) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var jsonDFO jsonObjective
	err := json.Unmarshal(data, &jsonDFO)
	if err != nil {
		return err
	}

	o.C = &channel.Channel{}
	o.C.Id = jsonDFO.C

	o.Status = jsonDFO.Status

	return nil
}
