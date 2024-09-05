package node_test

import (
	"crypto/tls"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/bridge"
	"github.com/statechannels/go-nitro/internal/logging"
	internalRpc "github.com/statechannels/go-nitro/internal/rpc"
	"github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	Token "github.com/statechannels/go-nitro/node/engine/chainservice/erc20"
	"github.com/statechannels/go-nitro/rpc"
	"github.com/statechannels/go-nitro/rpc/transport"
	"github.com/statechannels/go-nitro/rpc/transport/http"
)

const BRIDGE_RPC_PORT = 4006

func setupBridgeWithRPCClient(
	t *testing.T,
	bridgeConfig bridge.BridgeConfig,
) (rpc.RpcClientApi, string, string) {
	logging.SetupDefaultLogger(os.Stdout, slog.LevelDebug)
	bridge := bridge.New()

	_, _, nodeL1MultiAddress, nodeL2MultiAddress, err := bridge.Start(bridgeConfig)
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

	return rpcClient, nodeL1MultiAddress, nodeL2MultiAddress
}

func TestBridgeFlow(t *testing.T) {
	// TODO: Check if bridge client is really required
	tcL1 := TestCase{
		Chain:             AnvilChainL1,
		MessageService:    P2PMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
			{StoreType: MemStore, Actor: testactors.Irene},
		},
		deployerIndex: 1,
	}

	tcL2 := TestCase{
		Chain:             AnvilChainL2,
		MessageService:    P2PMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.BobPrime},
			{StoreType: MemStore, Actor: testactors.AlicePrime},
			{StoreType: MemStore, Actor: testactors.Irene},
		},
		ChainPort:     "8546",
		deployerIndex: 0,
	}

	dataFolder, _ := testhelpers.GenerateTempStoreFolder()

	infraL1 := setupSharedInfra(tcL1)

	infraL2 := setupSharedInfra(tcL2)

	_, err := Token.NewToken(infraL1.anvilChain.ContractAddresses.TokenAddress, infraL1.anvilChain.EthClient)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Token.NewToken(infraL2.anvilChain.ContractAddresses.TokenAddress, infraL2.anvilChain.EthClient)
	if err != nil {
		t.Fatal(err)
	}

	bridgeConfig := bridge.BridgeConfig{
		L1ChainUrl:        infraL1.anvilChain.ChainUrl,
		L2ChainUrl:        infraL2.anvilChain.ChainUrl,
		L1ChainStartBlock: 0,
		L2ChainStartBlock: 0,
		ChainPK:           infraL1.anvilChain.ChainPks[tcL1.Participants[1].ChainAccountIndex],
		StateChannelPK:    common.Bytes2Hex(tcL1.Participants[1].PrivateKey),
		NaAddress:         infraL1.anvilChain.ContractAddresses.NaAddress.String(),
		VpaAddress:        infraL1.anvilChain.ContractAddresses.VpaAddress.String(),
		CaAddress:         infraL1.anvilChain.ContractAddresses.CaAddress.String(),
		BridgeAddress:     infraL2.anvilChain.ContractAddresses.BridgeAddress.String(),
		DurableStoreDir:   dataFolder,
		BridgePublicIp:    DEFAULT_PUBLIC_IP,
		NodeL1MsgPort:     int(tcL1.Participants[1].Port),
		NodeL2MsgPort:     int(tcL2.Participants[0].Port),
		Assets: []bridge.Asset{
			{
				L1AssetAddress: infraL1.anvilChain.ContractAddresses.TokenAddress.String(),
				L2AssetAddress: infraL2.anvilChain.ContractAddresses.TokenAddress.String(),
			},
		},
	}

	nodeAMockChainservice := chainservice.NewMockChainService(infraL1.mockChain, tcL1.Participants[0].Address())
	nodeAPrimeMockChainservice := chainservice.NewMockChainService(infraL2.mockChain, tcL2.Participants[1].Address())
	// TODO: Use setup function to setup bridge server and client
	bridgeClient, nodeL1MultiAddress, nodeL2MultiAddress := setupBridgeWithRPCClient(t, bridgeConfig)
	// TODO: use node setup function to setup L1 and L2 nodes
	nodeARpcClient, _, _ := setupNitroNodeWithRPCClient(t, tcL1.Participants[0].PrivateKey, int(tcL1.Participants[0].Port), int(tcL1.Participants[0].WSPort), 4007, nodeAMockChainservice, transport.Http, []string{nodeL1MultiAddress})

	nodeAPrimeRpcClient, _, _ := setupNitroNodeWithRPCClient(t, tcL2.Participants[1].PrivateKey, int(tcL2.Participants[1].Port), int(tcL2.Participants[1].WSPort), 4008, nodeAPrimeMockChainservice, transport.Http, []string{nodeL2MultiAddress})

	// TODO: Use clients to perform following flow
	// TODO: Perform directfund between L1 node and bridge using L1 node's RPC client
	// TODO: Check that bridge channel is established
	// TODO: Create virtual channel, make payments and close virtual channel
	// TODO: Close bridge channel using L2 node's RPC client
	// TODO: Check that corresponding ledger channel is closed
}
