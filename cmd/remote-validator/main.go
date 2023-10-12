package main

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/rpc"
	"github.com/statechannels/go-nitro/rpc/transport/http"
)

const (
	NITRO_ENDPOINT = "localhost:4006/api/v1"
)

func main() {
	clientConnection, _ := http.NewHttpTransportAsClient(NITRO_ENDPOINT, 10*time.Millisecond)

	rpcClient, _ := rpc.NewRpcClient(clientConnection)

	VoucherHash := common.Hash{32}
	signerAddress := common.Address{20}

	value := big.NewInt(100)

	remVal := rpc.RemoteVoucherValidator{Client: rpcClient}

	err := remVal.ValidateVoucher(VoucherHash, signerAddress, value)
	if err != nil {
		panic(err)
	}
}
