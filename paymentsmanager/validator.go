package paymentsmanager

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
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
	success, errCode := v.PaymentsManager.ValidateVoucher(voucherHash, signerAddress, value)

	if !success {
		return fmt.Errorf(errCode)
	}

	return nil
}
