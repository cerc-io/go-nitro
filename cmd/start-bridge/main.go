package main

import (
	"context"
	"log/slog"
	"os"
	"os/exec"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/bridge"
	"github.com/statechannels/go-nitro/cmd/utils"
	"github.com/statechannels/go-nitro/internal/chain"
	"github.com/statechannels/go-nitro/internal/logging"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	p2pms "github.com/statechannels/go-nitro/node/engine/messageservice/p2p-message-service"
	"github.com/statechannels/go-nitro/node/engine/store"
)

func run() ([]*exec.Cmd, error) {
	runningCmd := []*exec.Cmd{}

	const CHAIN_PK = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
	const STATE_CHANNEL_PK = "0279651921cd800ac560c21ceea27aab0107b67daf436cdd25ce84cad30159b4"

	// TODO: Remove start chain and deploy contract code after implementing CLI
	// start 2 anvil chains
	anvilCmdL1, err := chain.StartAnvil("8545")
	if err != nil {
		return runningCmd, err
	}
	runningCmd = append(runningCmd, anvilCmdL1)

	anvilCmdL2, err := chain.StartAnvil("8546")
	if err != nil {
		return runningCmd, err
	}
	runningCmd = append(runningCmd, anvilCmdL2)

	// Deploy contracts
	contractAddresses, err := chain.DeployContracts(context.Background(), "ws://127.0.0.1:8545", "", CHAIN_PK)
	if err != nil {
		return runningCmd, err
	}

	bridgeAddress, err := chain.DeployL2Contracts(context.Background(), "ws://127.0.0.1:8546", "", CHAIN_PK)
	if err != nil {
		return runningCmd, err
	}

	chainOptsL1 := chainservice.ChainOpts{
		ChainUrl:           "ws://127.0.0.1:8545",
		ChainStartBlockNum: 0,
		ChainAuthToken:     "",
		ChainPk:            CHAIN_PK,
		NaAddress:          contractAddresses.NaAddress,
		VpaAddress:         contractAddresses.VpaAddress,
		CaAddress:          contractAddresses.CaAddress,
	}

	chainOptsL2 := chainservice.L2ChainOpts{
		ChainUrl:           "ws://127.0.0.1:8546",
		ChainStartBlockNum: 0,
		ChainAuthToken:     "",
		ChainPk:            CHAIN_PK,
		BridgeAddress:      bridgeAddress,
	}

	storeOptsL1 := store.StoreOpts{
		PkBytes:            common.Hex2Bytes(STATE_CHANNEL_PK),
		UseDurableStore:    true,
		DurableStoreFolder: "./data/l1-nitro-store",
	}

	storeOptsL2 := store.StoreOpts{
		PkBytes:            common.Hex2Bytes(STATE_CHANNEL_PK),
		UseDurableStore:    true,
		DurableStoreFolder: "./data/l2-nitro-store",
	}

	messageOptsL1 := p2pms.MessageOpts{
		PkBytes:   common.Hex2Bytes(STATE_CHANNEL_PK),
		Port:      3005,
		BootPeers: nil,
		PublicIp:  "127.0.0.1",
	}

	messageOptsL2 := p2pms.MessageOpts{
		PkBytes:   common.Hex2Bytes(STATE_CHANNEL_PK),
		Port:      3006,
		BootPeers: nil,
		PublicIp:  "127.0.0.1",
	}

	bridgeConfig := bridge.BridgeConfig{
		ChainOptsL1:   chainOptsL1,
		StoreOptsL1:   storeOptsL1,
		MessageOptsL1: messageOptsL1,
		StoreOptsL2:   storeOptsL2,
		MessageOptsL2: messageOptsL2,
		ChainOptsL2:   chainOptsL2,
	}

	logging.SetupDefaultLogger(os.Stdout, slog.LevelDebug)
	bridge := bridge.New(bridgeConfig)

	err = bridge.Start()
	if err != nil {
		return runningCmd, err
	}

	defer bridge.Close()
	utils.WaitForKillSignal()

	return nil, nil
}

func main() {
	runningCmd, err := run()
	if err != nil {
		utils.StopCommands(runningCmd...)
		panic(err)
	}
}
