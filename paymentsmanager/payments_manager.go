package paymentsmanager

import (
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/payments"
	"github.com/statechannels/go-nitro/types"
	"golang.org/x/exp/slog"
)

const (
	DEFAULT_LRU_CACHE_MAX_ACCOUNTS             = 1000
	DEFAULT_LRU_CACHE_ACCOUNT_TTL              = 30 * 60 // 30mins
	DEFAULT_LRU_CACHE_MAX_VOUCHERS_PER_ACCOUNT = 1000
	DEFAULT_LRU_CACHE_VOUCHER_TTL              = 5 * 60 // 5mins
	DEFAULT_LRU_CACHE_MAX_PAYMENT_CHANNELS     = 10000
	DEFAULT_LRU_CACHE_PAYMENT_CHANNEL_TTL      = DEFAULT_LRU_CACHE_ACCOUNT_TTL

	DEFAULT_VOUCHER_CHECK_INTERVAL = 2
	DEFAULT_VOUCHER_CHECK_ATTEMPTS = 5
)

type InFlightVoucher struct {
	voucher payments.Voucher
	amount  *big.Int
}

// Struct representing the payments manager service
type PaymentsManager struct {
	nitro *node.Node

	// In-memory LRU cache of vouchers received on payment channels
	// Map: payer -> voucher hash -> InFlightVoucher (voucher, delta amount)
	receivedVouchersCache *expirable.LRU[string, *expirable.LRU[string, InFlightVoucher]]

	// LRU map to keep track of amounts paid so far on payment channels
	// Map: channel id -> amount paid so far
	paidSoFarOnChannel *expirable.LRU[string, *big.Int]

	// Used to signal shutdown of the service
	quitChan chan bool
}

func NewPaymentsManager(nitro *node.Node) (PaymentsManager, error) {
	pm := PaymentsManager{nitro: nitro}

	pm.receivedVouchersCache = expirable.NewLRU[string, *expirable.LRU[string, InFlightVoucher]](
		DEFAULT_LRU_CACHE_MAX_ACCOUNTS,
		nil,
		time.Second*DEFAULT_LRU_CACHE_ACCOUNT_TTL,
	)

	pm.paidSoFarOnChannel = expirable.NewLRU[string, *big.Int](
		DEFAULT_LRU_CACHE_MAX_PAYMENT_CHANNELS,
		nil,
		time.Second*DEFAULT_LRU_CACHE_PAYMENT_CHANNEL_TTL,
	)

	pm.quitChan = make(chan bool)

	// Load existing open payment channels with amount paid so far from the stored state
	err := pm.loadPaymentChannels()
	if err != nil {
		return PaymentsManager{}, err
	}

	return pm, nil
}

func (pm *PaymentsManager) Start(wg *sync.WaitGroup) {
	slog.Info("starting payments manager...")

	wg.Add(1)
	go func() {
		defer wg.Done()
		pm.run()
	}()
}

func (pm *PaymentsManager) Stop() error {
	slog.Info("stopping payments manager...")
	close(pm.quitChan)
	return nil
}

func (pm *PaymentsManager) ValidateVoucher(voucherHash common.Hash, signerAddress common.Address, value *big.Int) (bool, bool) {
	// Check the payments map for required voucher
	var isPaymentReceived, isOfSufficientValue bool
	for i := 0; i < DEFAULT_VOUCHER_CHECK_ATTEMPTS; i++ {
		isPaymentReceived, isOfSufficientValue = pm.checkVoucherInCache(voucherHash, signerAddress, value)

		if isPaymentReceived {
			return true, isOfSufficientValue
		}

		// Retry after an interval if voucher not found
		slog.Info("Payment from %s not found, retrying after %d sec...", signerAddress, DEFAULT_VOUCHER_CHECK_INTERVAL)
		time.Sleep(DEFAULT_VOUCHER_CHECK_INTERVAL * time.Second)
	}

	return false, false
}

// Check for a given payment voucher in LRU cache map
// Returns whether the voucher was found, whether it was of sufficient value
func (pm *PaymentsManager) checkVoucherInCache(voucherHash common.Hash, signerAddress common.Address, minRequiredValue *big.Int) (bool, bool) {
	vouchersMap, ok := pm.receivedVouchersCache.Get(signerAddress.Hex())
	if !ok {
		return false, false
	}

	receivedVoucher, ok := vouchersMap.Get(voucherHash.Hex())
	if !ok {
		return false, false
	}

	if receivedVoucher.amount.Cmp(minRequiredValue) < 0 {
		return true, false
	}

	// Delete the voucher from map after consuming it
	vouchersMap.Remove(voucherHash.Hex())
	return true, true
}

func (pm *PaymentsManager) run() {
	slog.Info("starting voucher subscription...")
	for {
		select {
		case voucher := <-pm.nitro.ReceivedVouchers():
			payer, err := pm.getChannelCounterparty(voucher.ChannelId)
			if err != nil {
				// TODO: Handle
				panic(err)
			}

			paidSoFar, ok := pm.paidSoFarOnChannel.Get(voucher.ChannelId.String())
			if !ok {
				paidSoFar = big.NewInt(0)
			}

			paymentAmount := big.NewInt(0).Sub(voucher.Amount, paidSoFar)
			pm.paidSoFarOnChannel.Add(voucher.ChannelId.String(), voucher.Amount)
			slog.Info("Received a voucher", "payer", payer.String(), "amount", paymentAmount.String())

			vouchersMap, ok := pm.receivedVouchersCache.Get(payer.Hex())
			if !ok {
				vouchersMap = expirable.NewLRU[string, InFlightVoucher](
					DEFAULT_LRU_CACHE_MAX_VOUCHERS_PER_ACCOUNT,
					nil,
					time.Second*DEFAULT_LRU_CACHE_VOUCHER_TTL,
				)

				pm.receivedVouchersCache.Add(payer.Hex(), vouchersMap)
			}

			voucherHash, err := voucher.Hash()
			if err != nil {
				// TODO: Handle
				panic(err)
			}

			vouchersMap.Add(voucherHash.Hex(), InFlightVoucher{voucher: voucher, amount: paymentAmount})
		case <-pm.quitChan:
			slog.Info("stopping voucher subscription loop")
			return
		}
	}
}

func (pm *PaymentsManager) getChannelCounterparty(channelId types.Destination) (common.Address, error) {
	paymentChannel, err := pm.nitro.GetPaymentChannel(channelId)
	if err != nil {
		return common.Address{}, err
	}

	return paymentChannel.Balance.Payer, nil
}

func (pm *PaymentsManager) loadPaymentChannels() error {
	// TODO: Implement
	return nil
}
