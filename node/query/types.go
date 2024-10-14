package query

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/payments"
	"github.com/statechannels/go-nitro/types"
)

type ChannelStatus string

// TODO: Think through statuses
const (
	Proposed ChannelStatus = "Proposed"
	Open     ChannelStatus = "Open"
	Closing  ChannelStatus = "Closing"
	Complete ChannelStatus = "Complete"
)

// PaymentChannelBalance contains the balance of a uni-directional payment channel
type PaymentChannelBalance struct {
	AssetAddress   types.Address
	Payee          types.Address
	Payer          types.Address
	PaidSoFar      *hexutil.Big
	RemainingFunds *hexutil.Big
}

type SwapChannelBalance struct {
	AssetAddress types.Address
	Me           types.Address
	Them         types.Address
	MyBalance    *hexutil.Big
	TheirBalance *hexutil.Big
}

// PaymentChannelInfo contains balance and status info about a payment channel
type PaymentChannelInfo struct {
	ID      types.Destination
	Status  ChannelStatus
	Balance PaymentChannelBalance
}

type SwapChannelInfo struct {
	ID       types.Destination
	Status   ChannelStatus
	Balances []SwapChannelBalance
}

// LedgerChannelInfo contains balance and status info about a ledger channel
type LedgerChannelInfo struct {
	ID          types.Destination
	Status      ChannelStatus
	Balances    []LedgerChannelBalance
	ChannelMode channel.ChannelMode
}

type SwapInfo struct {
	Swap   payments.Swap
	Status types.SwapStatus
}

// LedgerChannelBalance contains the balance of a ledger channel
type LedgerChannelBalance struct {
	AssetAddress types.Address
	Me           types.Address
	Them         types.Address
	MyBalance    *hexutil.Big
	TheirBalance *hexutil.Big
}

// Equal returns true if the other LedgerChannelBalance is equal to this one
func (lcb LedgerChannelBalance) Equal(other LedgerChannelBalance) bool {
	return lcb.AssetAddress == other.AssetAddress &&
		lcb.Them == other.Them &&
		lcb.Me == other.Me &&
		lcb.TheirBalance.ToInt().Cmp(other.TheirBalance.ToInt()) == 0 &&
		lcb.MyBalance.ToInt().Cmp(other.MyBalance.ToInt()) == 0
}

// Equal returns true if the other LedgerChannelInfo is equal to this one
func (li LedgerChannelInfo) Equal(other LedgerChannelInfo) bool {
	areBalancesEqual := true

	if len(li.Balances) != len(other.Balances) {
		areBalancesEqual = false
	} else {
		for i, balance := range li.Balances {
			if !balance.Equal(other.Balances[i]) {
				areBalancesEqual = false
			}
		}
	}

	return li.ID == other.ID && li.Status == other.Status && areBalancesEqual && li.ChannelMode == other.ChannelMode
}

// Equal returns true if the other PaymentChannelInfo is equal to this one
func (pci PaymentChannelInfo) Equal(other PaymentChannelInfo) bool {
	return pci.ID == other.ID && pci.Status == other.Status && pci.Balance.Equal(other.Balance)
}

// Equal returns true if the other PaymentChannelBalance is equal to this one
func (pcb PaymentChannelBalance) Equal(other PaymentChannelBalance) bool {
	return pcb.AssetAddress == other.AssetAddress &&
		pcb.Payee == other.Payee &&
		pcb.Payer == other.Payer &&
		pcb.PaidSoFar.ToInt().Cmp(other.PaidSoFar.ToInt()) == 0 &&
		pcb.RemainingFunds.ToInt().Cmp(other.RemainingFunds.ToInt()) == 0
}

// Equal returns true if the other SwapChannelInfo is equal to this one
func (sci SwapChannelInfo) Equal(other SwapChannelInfo) bool {
	areBalancesEqual := true
	if len(sci.Balances) != len(other.Balances) {
		areBalancesEqual = false
	} else {
		for i, balance := range sci.Balances {
			if !balance.Equal(other.Balances[i]) {
				areBalancesEqual = false
			}
		}
	}

	return sci.ID == other.ID && sci.Status == other.Status && areBalancesEqual
}

// Equal returns true if the other SwapChannelBalance is equal to this one
func (scb SwapChannelBalance) Equal(other SwapChannelBalance) bool {
	return scb.AssetAddress == other.AssetAddress &&
		scb.Them == other.Them &&
		scb.Me == other.Me &&
		scb.TheirBalance.ToInt().Cmp(other.TheirBalance.ToInt()) == 0 &&
		scb.MyBalance.ToInt().Cmp(other.MyBalance.ToInt()) == 0
}
