package bridge

import (
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/directdefund"
	"github.com/statechannels/go-nitro/protocols/directfund"
	"github.com/statechannels/go-nitro/protocols/swapdefund"
	"github.com/statechannels/go-nitro/protocols/swapfund"
	"github.com/statechannels/go-nitro/protocols/virtualdefund"
	"github.com/statechannels/go-nitro/protocols/virtualfund"
)

type (
	NodeL1PermissivePolicy struct{}
	NodeL2PermissivePolicy struct{}
)

func (pp *NodeL1PermissivePolicy) ShouldApprove(o protocols.Objective) bool {
	// L1 node rejects objectives if they involve virtual funding, virtual defunding, or direct defunding
	if virtualfund.IsVirtualFundObjective(o.Id()) || virtualdefund.IsVirtualDefundObjective(o.Id()) || directdefund.IsDirectDefundObjective(o.Id()) || swapfund.IsSwapFundObjective(o.Id()) || swapdefund.IsSwapDefundObjective(o.Id()) {
		return false
	}

	return o.GetStatus() == protocols.Unapproved
}

func (pp *NodeL2PermissivePolicy) ShouldApprove(o protocols.Objective) bool {
	// L2 node rejects objectives if they involve direct funding
	if directfund.IsDirectFundObjective(o.Id()) || directdefund.IsDirectDefundObjective(o.Id()) {
		return false
	}

	return o.GetStatus() == protocols.Unapproved
}
