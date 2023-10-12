package paymentsmanager

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/rpc"
)

var (
	ERR_PAYMENT              = "Payment error:"
	ERR_PAYMENT_NOT_RECEIVED = errors.New(fmt.Sprintf("%s payment not received", ERR_PAYMENT))
	ERR_AMOUNT_INSUFFICIENT  = errors.New(fmt.Sprintf("%s amount insufficient", ERR_PAYMENT))
)

type VoucherValidator interface {
	ValidateVoucher(voucherHash common.Hash, signerAddress common.Address, value *big.Int) error
}

// When go-nitro is running in-process
type InProcessValidator struct {
	PaymentsManager
}

func (v InProcessValidator) ValidateVoucher(voucherHash common.Hash, signerAddress common.Address, value *big.Int) error {
	isPaymentReceived, isOfSufficientValue := v.PaymentsManager.ValidateVoucher(voucherHash, signerAddress, value)

	if !isPaymentReceived {
		return ERR_PAYMENT_NOT_RECEIVED
	}

	if !isOfSufficientValue {
		return ERR_AMOUNT_INSUFFICIENT
	}

	return nil
}

// When go-nitro is running remotely
type RemoteValidator struct {
	client rpc.RpcClientApi
}

func (r RemoteValidator) ValidateVoucher(voucherHash common.Hash, signerAddress common.Address, value *big.Int) error {
	// TODO: Implement
	return nil
}