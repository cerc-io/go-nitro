package query

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/consensus_channel"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/payments"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/virtualfund"
	"github.com/statechannels/go-nitro/types"
)

// getStatusFromChannel returns the status of the channel
func getStatusFromChannel(c *channel.Channel) ChannelStatus {
	// If the assets are liquidated for a challenged channel, it indicates completion
	if c.OnChain.FinalizesAt.Cmp(big.NewInt(0)) != 0 && !c.OnChain.Holdings.IsNonZero() {
		return Complete
	}

	if c.FinalSignedByMe() {
		if c.FinalCompleted() {
			return Complete
		}
		return Closing
	}

	if !c.PostFundComplete() {
		return Proposed
	}
	return Open
}

// getPaymentChannelBalance generates a PaymentChannelBalance from the given participants and outcome
func getPaymentChannelBalance(participants []types.Address, outcome outcome.Exit) PaymentChannelBalance {
	numParticipants := len(participants)
	// TODO: We assume single asset outcomes
	sao := outcome[0]
	asset := sao.Asset
	payer := participants[0]
	payee := participants[numParticipants-1]
	paidSoFar := big.NewInt(0).Set(sao.Allocations[1].Amount)
	remaining := big.NewInt(0).Set(sao.Allocations[0].Amount)
	return PaymentChannelBalance{
		AssetAddress:   asset,
		Payer:          payer,
		Payee:          payee,
		PaidSoFar:      (*hexutil.Big)(paidSoFar),
		RemainingFunds: (*hexutil.Big)(remaining),
	}
}

func getSwapChannelBalances(participants []types.Address, outcome outcome.Exit, myAddress common.Address) ([]SwapChannelBalance, error) {
	numParticipants := len(participants)
	var scb []SwapChannelBalance

	for _, sao := range outcome {
		asset := sao.Asset
		var me, them common.Address
		var myBalance, theirBalance *big.Int
		if participants[0] == myAddress {
			me = participants[0]
			them = participants[numParticipants-1]
			myBalance = big.NewInt(0).Set(sao.Allocations[0].Amount)
			theirBalance = big.NewInt(0).Set(sao.Allocations[1].Amount)
		} else if participants[numParticipants-1] == myAddress {
			me = participants[numParticipants-1]
			them = participants[0]
			myBalance = big.NewInt(0).Set(sao.Allocations[1].Amount)
			theirBalance = big.NewInt(0).Set(sao.Allocations[0].Amount)
		} else {
			return []SwapChannelBalance{}, fmt.Errorf("could not find my address %v in participants %v", myAddress, participants)
		}

		scb = append(scb, SwapChannelBalance{
			AssetAddress: asset,
			Me:           me,
			Them:         them,
			MyBalance:    (*hexutil.Big)(myBalance),
			TheirBalance: (*hexutil.Big)(theirBalance),
		})
	}

	return scb, nil
}

// getLatestSupportedOrPreFund returns the latest supported state of the channel
// or the prefund state if no supported state exists
func getLatestSupportedOrPreFund(ledgerChannel *channel.Channel) (state.State, error) {
	if ledgerChannel.HasSupportedState() {
		return ledgerChannel.LatestSupportedState()
	}
	return ledgerChannel.PreFundState(), nil
}

// getLedgerBalanceFromState returns the balance of the ledger channel from the given state
func getLedgerBalanceFromState(latest state.State, myAddress types.Address) ([]LedgerChannelBalance, error) {
	var balances []LedgerChannelBalance

	for _, outcome := range latest.Outcome {
		var them types.Address
		var myBalance, theirBalance *big.Int

		if latest.Participants[0] == myAddress {
			them = latest.Participants[1]
			theirBalance = outcome.Allocations[1].Amount
			myBalance = outcome.Allocations[0].Amount
		} else if latest.Participants[1] == myAddress {
			them = latest.Participants[0]
			theirBalance = outcome.Allocations[0].Amount
			myBalance = outcome.Allocations[1].Amount
		} else {
			return []LedgerChannelBalance{}, fmt.Errorf("could not find my address %v in participants %v", myAddress, latest.Participants)
		}

		balances = append(balances, LedgerChannelBalance{
			AssetAddress: outcome.Asset,
			Me:           myAddress,
			Them:         them,
			MyBalance:    (*hexutil.Big)(myBalance),
			TheirBalance: (*hexutil.Big)(theirBalance),
		})
	}

	return balances, nil
}

// GetVirtualFundObjective returns the virtual fund objective for the given channel if it exists.
func GetVirtualFundObjective(id types.Destination, store store.Store) (*virtualfund.Objective, bool) {
	// This is slightly awkward but if the virtual defunding objective is complete it won't come back if we query by channel id
	// We manually construct the objective id and query by that
	virtualFundId := protocols.ObjectiveId(virtualfund.ObjectivePrefix + id.String())
	o, err := store.GetObjectiveById(virtualFundId)
	if err != nil {
		return nil, false
	}
	return o.(*virtualfund.Objective), true
}

// GetVoucherBalance returns the amount paid and remaining for a given channel based on vouchers received.
// If not vouchers are received for the channel, it returns 0 for paid and remaining.
func GetVoucherBalance(id types.Destination, vm *payments.VoucherManager) (paid, remaining *big.Int, err error) {
	paid, remaining = big.NewInt(0), big.NewInt(0)

	if noVouchers := !vm.ChannelRegistered(id); noVouchers {
		return
	}
	paid, err = vm.Paid(id)
	if err != nil {
		return nil, nil, err
	}

	remaining, err = vm.Remaining(id)
	if err != nil {
		return nil, nil, err
	}

	return paid, remaining, nil
}

// GetPaymentChannelInfo returns the PaymentChannelInfo for the given channel
// It does this by querying the provided store and voucher manager
func GetPaymentChannelInfo(id types.Destination, store store.Store, vm *payments.VoucherManager) (PaymentChannelInfo, error) {
	if (id == types.Destination{}) {
		return PaymentChannelInfo{}, errors.New("a valid channel id must be provided")
	}
	// Otherwise we can just check the store
	c, channelFound := store.GetChannelById(id)

	if channelFound {
		paid, remaining, err := GetVoucherBalance(id, vm)
		if err != nil {
			return PaymentChannelInfo{}, err
		}

		return ConstructPaymentInfo(c, paid, remaining)
	}
	return PaymentChannelInfo{}, fmt.Errorf("could not find channel with id %v", id)
}

func GetSwapChannelInfo(id types.Destination, store store.Store) (string, error) {
	if (id == types.Destination{}) {
		return "", errors.New("a valid channel id must be provided")
	}

	c, channelFound := store.GetChannelById(id)
	swapChannnel := channel.SwapChannel{Channel: *c}
	if channelFound {
		SwapChannelInfo, err := ConstructSwapInfo(swapChannnel, *store.GetAddress())
		if err != nil {
			return "", err
		}

		marshalledSwapChannelInfo, err := json.Marshal(SwapChannelInfo)
		if err != nil {
			return "", err
		}

		return string(marshalledSwapChannelInfo), nil
	}

	return "", fmt.Errorf("could not find channel with id %v", id)
}

// GetAllLedgerChannels returns a `LedgerChannelInfo` for each ledger channel in the store.
func GetAllLedgerChannels(store store.Store, consensusAppDefinition types.Address) ([]LedgerChannelInfo, error) {
	toReturn := []LedgerChannelInfo{}
	myAddress := *store.GetAddress()

	allConsensus, err := store.GetAllConsensusChannels()
	if err != nil {
		return []LedgerChannelInfo{}, err
	}

	failedConstructions := []string{}

	for _, con := range allConsensus {
		lInfo, err := ConstructLedgerInfoFromConsensus(con, myAddress)
		if err != nil {
			failedConstructions = append(failedConstructions, fmt.Sprintf("%v: %v", con.Id, err))
			continue
		}
		toReturn = append(toReturn, lInfo)
	}
	allChannels, err := store.GetChannelsByAppDefinition(consensusAppDefinition)
	if err != nil {
		return []LedgerChannelInfo{}, err
	}
	for _, c := range allChannels {
		if c.Type != types.Ledger {
			continue
		}

		l, err := ConstructLedgerInfoFromChannel(c, myAddress)
		if err != nil {
			return []LedgerChannelInfo{}, err
		}
		toReturn = append(toReturn, l)
	}
	err = nil
	if len(failedConstructions) > 0 {
		err = fmt.Errorf("failed to construct ledger channel info for the following channels: %v", failedConstructions)
	}

	return toReturn, err
}

// GetPaymentChannelsByLedger returns a `PaymentChannelInfo` for each active payment channel funded by the given ledger channel.
func GetPaymentChannelsByLedger(ledgerId types.Destination, s store.Store, vm *payments.VoucherManager) ([]PaymentChannelInfo, error) {
	// If a ledger channel is actively funding payment channels it must be in the form of a consensus channel
	con, err := s.GetConsensusChannelById(ledgerId)
	// If the ledger channel is not a consensus channel we know that there are no payment channels funded by it
	if errors.Is(err, store.ErrNoSuchChannel) {
		return []PaymentChannelInfo{}, nil
	}
	if err != nil {
		return []PaymentChannelInfo{}, fmt.Errorf("could not find any payment channels funded by %s: %w", ledgerId, err)
	}

	toQuery := con.ConsensusVars().Outcome.FundingTargets()

	paymentChannels, err := s.GetChannelsByIds(toQuery)
	if err != nil {
		return []PaymentChannelInfo{}, fmt.Errorf("could not query the store about ids %v: %w", toQuery, err)
	}

	toReturn := []PaymentChannelInfo{}
	for _, p := range paymentChannels {
		if p.Type != types.Virtual {
			continue
		}

		paid, remaining, err := GetVoucherBalance(p.Id, vm)
		if err != nil {
			return []PaymentChannelInfo{}, err
		}

		info, err := ConstructPaymentInfo(p, paid, remaining)
		if err != nil {
			return []PaymentChannelInfo{}, err
		}
		toReturn = append(toReturn, info)
	}
	return toReturn, nil
}

// GetSwapChannelsByLedger returns marshalled `SwapChannelInfo` for each active swap channel funded by the given ledger channel.
func GetSwapChannelsByLedger(ledgerId types.Destination, s store.Store) ([]SwapChannelInfo, error) {
	// If a ledger channel is actively funding swao channels it must be in the form of a consensus channel
	con, err := s.GetConsensusChannelById(ledgerId)
	// If the ledger channel is not a consensus channel we know that there are no swap channels funded by it
	if errors.Is(err, store.ErrNoSuchChannel) {
		return []SwapChannelInfo{}, nil
	}
	if err != nil {
		return []SwapChannelInfo{}, fmt.Errorf("could not find any payment channels funded by %s: %w", ledgerId, err)
	}

	toQuery := con.ConsensusVars().Outcome.FundingTargets()

	channels, err := s.GetChannelsByIds(toQuery)
	if err != nil {
		return []SwapChannelInfo{}, fmt.Errorf("could not query the store about ids %v: %w", toQuery, err)
	}

	toReturn := []SwapChannelInfo{}

	for _, c := range channels {
		if c.Type != types.Swap {
			continue
		}

		swapChannel := channel.SwapChannel{
			Channel: *c,
		}

		info, err := ConstructSwapInfo(swapChannel, *s.GetAddress())
		if err != nil {
			return []SwapChannelInfo{}, err
		}

		toReturn = append(toReturn, info)
	}
	return toReturn, nil
}

// GetLedgerChannelInfo returns the LedgerChannelInfo for the given channel
// It does this by querying the provided store
func GetLedgerChannelInfo(id types.Destination, store store.Store) (LedgerChannelInfo, error) {
	c, ok := store.GetChannelById(id)
	myAddress := *store.GetAddress()

	if ok {
		return ConstructLedgerInfoFromChannel(c, myAddress)
	}

	con, err := store.GetConsensusChannelById(id)
	if err != nil {
		return LedgerChannelInfo{}, err
	}

	return ConstructLedgerInfoFromConsensus(con, myAddress)
}

func ConstructLedgerInfoFromConsensus(con *consensus_channel.ConsensusChannel, myAddress types.Address) (LedgerChannelInfo, error) {
	latest := con.ConsensusVars().AsState(con.FixedPart())
	balances, err := getLedgerBalanceFromState(latest, myAddress)
	if err != nil {
		return LedgerChannelInfo{}, fmt.Errorf("failed to construct ledger channel info from consensus channel: %w", err)
	}

	return LedgerChannelInfo{
		ID:          con.Id,
		Status:      Open,
		Balances:    balances,
		ChannelMode: channel.Open,
	}, nil
}

func ConstructLedgerInfoFromChannel(c *channel.Channel, myAddress types.Address) (LedgerChannelInfo, error) {
	latest, err := getLatestSupportedOrPreFund(c)
	if err != nil {
		return LedgerChannelInfo{}, err
	}
	balances, err := getLedgerBalanceFromState(latest, myAddress)
	if err != nil {
		return LedgerChannelInfo{}, fmt.Errorf("failed to construct ledger channel info from channel: %w", err)
	}

	return LedgerChannelInfo{
		ID:          c.Id,
		Status:      getStatusFromChannel(c),
		Balances:    balances,
		ChannelMode: c.OnChain.ChannelMode,
	}, nil
}

func ConstructPaymentInfo(c *channel.Channel, paid, remaining *big.Int) (PaymentChannelInfo, error) {
	status := getStatusFromChannel(c)
	// ADR 0009 allows for intermediaries to exit the protocol before receiving all signed post funds
	// So for intermediaries we return Open once they have signed their post fund state
	amIntermediary := c.MyIndex != 0 && c.MyIndex != uint(len(c.Participants)-1)
	if amIntermediary && c.PostFundSignedByMe() {
		status = Open
	}

	latest, err := getLatestSupportedOrPreFund(c)
	if err != nil {
		return PaymentChannelInfo{}, err
	}
	balance := getPaymentChannelBalance(c.Participants, latest.Outcome)

	balance.PaidSoFar.ToInt().Set(paid)

	balance.RemainingFunds.ToInt().Set(remaining)

	return PaymentChannelInfo{
		ID:      c.Id,
		Status:  status,
		Balance: balance,
	}, nil
}

func ConstructSwapInfo(c channel.SwapChannel, myAddress common.Address) (SwapChannelInfo, error) {
	status := getStatusFromChannel(&c.Channel)

	latest := c.LatestSupportedSwapChannelState()
	balances, err := getSwapChannelBalances(c.Participants, latest.Outcome, myAddress)
	if err != nil {
		return SwapChannelInfo{}, err
	}

	return SwapChannelInfo{
		ID:       c.Id,
		Status:   status,
		Balances: balances,
	}, nil
}
