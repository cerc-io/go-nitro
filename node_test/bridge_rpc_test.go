package node_test

import (
	"crypto/tls"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/statechannels/go-nitro/bridge"
	"github.com/statechannels/go-nitro/internal/logging"
	internalRpc "github.com/statechannels/go-nitro/internal/rpc"
	"github.com/statechannels/go-nitro/rpc"
	"github.com/statechannels/go-nitro/rpc/transport/http"
)

const BRIDGE_RPC_PORT = 4006

func setupBridgeWithRPCClient(
	t *testing.T,
	bridgeConfig bridge.BridgeConfig,
) rpc.RpcClientApi {
	logging.SetupDefaultLogger(os.Stdout, slog.LevelDebug)
	bridge := bridge.New()

	_, _, _, _, err := bridge.Start(bridgeConfig)
	if err != nil {
		log.Fatal(err)
	}

	cert, err := tls.LoadX509KeyPair("../tls/statechannels.org.pem", "../tls/statechannels.org_key.pem")
	if err != nil {
		panic(err)
	}

	bridgeRpcServer, err := internalRpc.InitializeBridgeRpcServer(bridge, BRIDGE_RPC_PORT, false, &cert)
	if err != nil {
		panic(err)
	}

	clientConnection, err := http.NewHttpTransportAsClient(bridgeRpcServer.Url(), true, 10*time.Millisecond)
	if err != nil {
		panic(err)
	}

	rpcClient, err := rpc.NewRpcClient(clientConnection)
	if err != nil {
		panic(err)
	}

	// TODO: Add cleanup function to close bridge server and client

	return rpcClient
}

func TestBridgeFlow() {
	// TODO: Check if bridge client is really required

	// TODO: Use setup function to setup bridge server and client
	// TODO: use node setup function to setup L1 and L2 nodes
	// TODO: Perform directfund between L1 node and bridge using L1 node's RPC client
	// TODO: Check that bridge channel is established
	// TODO: Create virtual channel, make payments and close virtual channel
	// TODO: Close bridge channel using L2 node's RPC client
	// TODO: Check that corresponding ledger channel is closed
}
