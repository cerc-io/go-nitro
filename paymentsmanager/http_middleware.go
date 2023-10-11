package paymentsmanager

import (
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
)

func extractAndValidateVoucher(r *http.Request, validator VoucherValidator) (*http.Request, error) {
	// TODO: Determine RPC method from the request
	// TODO: Extract voucher details from the header

	voucherHash := common.HexToHash("")
	signer := common.HexToAddress("")
	amount := big.NewInt(0)

	return r, validator.ValidateVoucher(voucherHash, signer, amount)
}

// HTTPMiddleware http connection metric reader
func HTTPMiddleware(next http.Handler, validator VoucherValidator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authenticate payments
		r, err := extractAndValidateVoucher(r, validator)
		if err != nil {
			// TODO: Throw payment related error
			// w.WriteHeader(http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)
	})
}
