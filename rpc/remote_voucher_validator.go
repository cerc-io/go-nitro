package rpc

import (
	"log/slog"
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
	res, _ := r.Client.ValidateVoucher(voucherHash, signerAddress, value)
	slog.Info("Response from server after validatin", "res", res)
	return nil
}
