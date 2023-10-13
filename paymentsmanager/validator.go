package paymentsmanager

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/rpc"
)

var (
	ERR_PAYMENT              = "Payment error:"
	ERR_PAYMENT_NOT_RECEIVED = fmt.Errorf("%s payment not received", ERR_PAYMENT)
	ERR_AMOUNT_INSUFFICIENT  = fmt.Errorf("%s amount insufficient", ERR_PAYMENT)
)

// Voucher validator interface to be satisfied by implementations
// using in / out of process Nitro nodes
type VoucherValidator interface {
	ValidateVoucher(voucherHash common.Hash, signerAddress common.Address, value *big.Int) error
}

var _ VoucherValidator = &InProcessVoucherValidator{}

// When go-nitro is running in-process
type InProcessVoucherValidator struct {
	PaymentsManager
}

func (v InProcessVoucherValidator) ValidateVoucher(voucherHash common.Hash, signerAddress common.Address, value *big.Int) error {
	isPaymentReceived, isOfSufficientValue := v.PaymentsManager.ValidateVoucher(voucherHash, signerAddress, value)

	if !isPaymentReceived {
		return ERR_PAYMENT_NOT_RECEIVED
	}

	if !isOfSufficientValue {
		return ERR_AMOUNT_INSUFFICIENT
	}

	return nil
}

var _ VoucherValidator = &RemoteVoucherValidator{}

// When go-nitro is running remotely
type RemoteVoucherValidator struct {
	client rpc.RpcClientApi //nolint:unused
}

func (r RemoteVoucherValidator) ValidateVoucher(voucherHash common.Hash, signerAddress common.Address, value *big.Int) error {
	// TODO: Implement
	return nil
}
