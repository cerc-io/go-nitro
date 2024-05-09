package paymentsmanager

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/crypto"
)

const (
	PAYMENT_HEADER_KEY   = "x-payment"
	PAYMENT_HEADER_REGEX = "vhash:(.*),vsig:(.*)"
)

var (
	ErrHeaderMissing         = errors.New("payment header x-payment not set")
	ErrInvalidPaymentHeader  = errors.New("invalid payment header format")
	ErrUnableToRecoverSigner = errors.New("unable to recover the voucher signer")
)

// HTTPMiddleware: extracts and validates vouchers from RPC requests
func HTTPMiddleware(next http.Handler, validator VoucherValidator, queryRates map[string]*big.Int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate voucher
		r, err := extractAndValidateVoucher(r, validator, queryRates)
		if err != nil {
			if isPaymentError(err) {
				http.Error(w, err.Error(), http.StatusPaymentRequired)
			} else {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}

			return
		}

		// Let the request move ahead after voucher validation
		next.ServeHTTP(w, r)
	})
}

func extractAndValidateVoucher(r *http.Request, validator VoucherValidator, queryRates map[string]*big.Int) (*http.Request, error) {
	// Determine RPC method from the request
	isRpcCall, rpcMethod := isRpcCall(r)
	if !isRpcCall {
		return r, nil
	}

	// Determine the query cost
	queryCost := queryRates[rpcMethod]
	if queryCost == nil || queryCost.Cmp(big.NewInt(0)) == 0 {
		slog.Info("Serving a free RPC request", "method", rpcMethod)
		return r, nil
	}

	// Extract voucher details from the header
	paymentHeader := r.Header.Get(PAYMENT_HEADER_KEY)
	if paymentHeader == "" {
		return r, ErrHeaderMissing
	}

	re := regexp.MustCompile(PAYMENT_HEADER_REGEX)
	match := re.FindStringSubmatch(paymentHeader)

	var vhash, vsig string
	if match != nil {
		vhash = match[1]
		vsig = match[2]
	} else {
		return r, ErrInvalidPaymentHeader
	}

	// Determine signer from the voucher hash and signature
	vhashBytes := common.Hex2Bytes(strings.TrimPrefix(vhash, "0x"))
	signature := crypto.SplitSignature(common.Hex2Bytes(strings.TrimPrefix(vsig, "0x")))
	signer, err := crypto.RecoverEthereumMessageSigner(vhashBytes, signature)
	if err != nil {
		return r, ErrUnableToRecoverSigner
	}

	// Remove the payment header from the request
	r.Header.Del(PAYMENT_HEADER_KEY)

	err = validator.ValidateVoucher(common.HexToHash(vhash), signer, queryCost)
	if err != nil {
		return r, err
	}

	slog.Info("Serving a paid RPC request", "method", rpcMethod, "cost", queryCost, "sender", signer.Hex())
	return r, nil
}

// Helper method to parse request and determine whether it's a RPC call
// A request is a RPC call if:
//   - "Content-Type" header is set to "application/json"
//   - Request body has non-empty "jsonrpc" and "method" fields
//
// Also returns the parsed RPC method
func isRpcCall(r *http.Request) (bool, string) {
	if r.Header.Get("Content-Type") != "application/json" {
		return false, ""
	}

	var ReqBody struct {
		JsonRpc string `json:"jsonrpc"`
		Method  string `json:"method"`
	}
	bodyBytes, _ := io.ReadAll(r.Body)

	err := json.Unmarshal(bodyBytes, &ReqBody)
	if err != nil || ReqBody.JsonRpc == "" || ReqBody.Method == "" {
		return false, ""
	}

	// Reassign request body as io.ReadAll consumes it
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	return true, ReqBody.Method
}

func isPaymentError(err error) bool {
	return strings.HasPrefix(err.Error(), ERR_PAYMENT)
}
