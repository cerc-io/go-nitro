package serde

import (
	"github.com/statechannels/go-nitro/types"
)

func ValidatePaymentRequest(req PaymentRequest) error {
	if req.Amount == 0 {
		return InvalidParamsError
	}
	if (req.Channel == types.Destination{}) {
		return InvalidParamsError
	}
	return nil
}

func ValidateSwapInitiateRequest(req SwapInitiateRequest) error {
	if req.SwapAssetsData.AmountIn == 0 || req.SwapAssetsData.AmountOut == 0 {
		return InvalidParamsError
	}
	if (req.Channel == types.Destination{}) {
		return InvalidParamsError
	}
	return nil
}

func ValidateGetPaymentChannelRequest(req GetPaymentChannelRequest) error {
	if (req.Id == types.Destination{}) {
		return InvalidParamsError
	}
	return nil
}

func ValidateGetSwapChannelRequest(req GetSwapChannelRequest) error {
	if (req.Id == types.Destination{}) {
		return InvalidParamsError
	}
	return nil
}

func ValidateGetPaymentChannelsByLedgerRequest(req GetPaymentChannelsByLedgerRequest) error {
	if (req.LedgerId == types.Destination{}) {
		return InvalidParamsError
	}
	return nil
}

func ValidateGetSignedStateRequest(req GetSignedStateRequest) error {
	if (req.Id == types.Destination{}) {
		return InvalidParamsError
	}
	return nil
}
