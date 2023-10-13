package paymentsmanager

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

var (
	ErrPayment            = "Payment error:"
	ErrPaymentNotReceived = fmt.Errorf("%s payment not received", ErrPayment)
	ErrAmountInsufficient = fmt.Errorf("%s amount insufficient", ErrPayment)
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
		return ErrPaymentNotReceived
	}

	if !isOfSufficientValue {
		return ErrAmountInsufficient
	}

	return nil
}

var _ VoucherValidator = &RemoteVoucherValidator{}

// When go-nitro is running remotely
type RemoteVoucherValidator struct {
	// client rpc.RpcClientApi
}

func (r RemoteVoucherValidator) ValidateVoucher(voucherHash common.Hash, signerAddress common.Address, value *big.Int) error {
	// TODO: Implement
	return nil
}
