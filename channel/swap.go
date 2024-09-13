package channel

import (
	"errors"

	"github.com/statechannels/go-nitro/channel/state"
)

type SwapChannel struct {
	Channel
}

func NewSwapChannel(s state.State, myIndex uint) (*SwapChannel, error) {
	if int(myIndex) >= len(s.Participants) {
		return &SwapChannel{}, errors.New("myIndex not in range of the supplied participants")
	}

	for _, assetExit := range s.Outcome {
		if len(assetExit.Allocations) != 2 {
			return &SwapChannel{}, errors.New("a virtual channel's initial state should only have two allocations")
		}
	}

	c, err := New(s, myIndex, Virtual)

	return &SwapChannel{*c}, err
}

// Clone returns a pointer to a new, deep copy of the receiver, or a nil pointer if the receiver is nil.
func (v *SwapChannel) Clone() *SwapChannel {
	if v == nil {
		return nil
	}
	w := SwapChannel{*v.Channel.Clone()}
	return &w
}
