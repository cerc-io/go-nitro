package channel

import (
	"errors"

	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/types"
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
			return &SwapChannel{}, errors.New("a swap channel's initial state should only have two allocations")
		}
	}

	c, err := New(s, myIndex, types.Swap)

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
