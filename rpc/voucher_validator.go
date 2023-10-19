package rpc

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/paymentsmanager"
)

var _ paymentsmanager.VoucherValidator = &RemoteVoucherValidator{}

// When go-nitro is running remotely
type RemoteVoucherValidator struct {
	Client RpcClientApi
}

func (r RemoteVoucherValidator) ValidateVoucher(voucherHash common.Hash, signerAddress common.Address, value *big.Int) error {
	res, err := r.Client.ValidateVoucher(voucherHash, signerAddress, value.Uint64())
	if err != nil {
		return err
	}

	if !res.Success {
		return fmt.Errorf(res.ErrorCode)
	}

	return nil
}
