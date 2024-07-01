package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/statechannels/go-nitro/bridge"
	"github.com/statechannels/go-nitro/cmd/utils"
	"github.com/statechannels/go-nitro/internal/chain"
	"github.com/statechannels/go-nitro/internal/logging"
)

// TODO: Get these values from CLI
const (
	CHAIN_PK             = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
	STATE_CHANNEL_PK     = "0279651921cd800ac560c21ceea27aab0107b67daf436cdd25ce84cad30159b4"
	L1_CHAIN_URL         = "ws://127.0.0.1:8545"
	L2_CHAIN_URL         = "ws://127.0.0.1:8546"
	L1_CHAIN_START_BLOCK = 0
	L2_CHAIN_START_BLOCK = 0
	NA_ADDRESS           = "NA_ADDRESS"
	VPA_ADDRESS          = "VPA_ADDRESS"
	CA_ADDRESS           = "CA_ADDRESS"
	BRIDGE_ADDRESS       = "BRIDGE_ADDRESS"
	DURABLE_STORE_FOLDER = "./data"
	BRIDGE_PUBLIC_IP     = "127.0.0.1"
	NODEL1_MSG_PORT      = 3005
	NODEL2_MSG_PORT      = 3006
)

func main() {
	// TODO: Deploy contracts from outside and get contract addresses using CLI
	contractAddresses, err := chain.DeployContracts(context.Background(), L1_CHAIN_URL, "", CHAIN_PK)
	if err != nil {
		log.Fatal(err)
	}

	bridgeAddress, err := chain.DeployL2Contract(context.Background(), L2_CHAIN_URL, "", CHAIN_PK)
	if err != nil {
		log.Fatal(err)
	}

	bridgeConfig := bridge.BridgeConfig{
		L1ChainUrl:        L1_CHAIN_URL,
		L2ChainUrl:        L2_CHAIN_URL,
		L1ChainStartBlock: L1_CHAIN_START_BLOCK,
		L2ChainStartBlock: L2_CHAIN_START_BLOCK,
		ChainPK:           CHAIN_PK,
		StateChannelPK:    STATE_CHANNEL_PK,
		NaAddress:         contractAddresses.NaAddress.String(),
		VpaAddress:        contractAddresses.VpaAddress.String(),
		CaAddress:         contractAddresses.CaAddress.String(),
		BridgeAddress:     bridgeAddress.String(),
		DurableStoreDir:   DURABLE_STORE_FOLDER,
		BridgePublicIp:    BRIDGE_PUBLIC_IP,
		NodeL1MsgPort:     NODEL1_MSG_PORT,
		NodeL2MsgPort:     NODEL2_MSG_PORT,
	}

	logging.SetupDefaultLogger(os.Stdout, slog.LevelDebug)
	bridge := bridge.New(bridgeConfig)

	err = bridge.Start()
	if err != nil {
		log.Fatal(err)
	}

	defer bridge.Close()
	utils.WaitForKillSignal()
}
