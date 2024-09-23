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
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/mirrorbridgeddefund"
	"github.com/statechannels/go-nitro/rpc"
	"github.com/statechannels/go-nitro/rpc/transport"
	"github.com/statechannels/go-nitro/rpc/transport/http"
	"github.com/statechannels/go-nitro/types"
)

const BRIDGE_RPC_PORT = 4006

func setupBridgeWithRPCClient(
	t *testing.T,
	bridgeConfig bridge.BridgeConfig,
) (rpc.RpcClientApi, string, string, func()) {
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

	cleanupFn := func() {
		bridge.Close()
		rpcClient.Close()
	}

	return rpcClient, nodeL1MultiAddress, nodeL2MultiAddress, cleanupFn
}

func TestBridgeFlow(t *testing.T) {
	payAmount := uint(5)
	virtualChannelDeposit := uint(100)
	tcL1 := TestCase{
		Chain:             AnvilChain,
		MessageService:    P2PMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test_l1",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.Alice},
			{StoreType: MemStore, Actor: testactors.Bob},
		},
		deployerIndex: 1,
	}

	tcL2 := TestCase{
		Chain:             LaconicdChain,
		MessageService:    P2PMessageService,
		MessageDelay:      0,
		LogName:           "Bridge_test_l2",
		ChallengeDuration: 5,
		Participants: []TestParticipant{
			{StoreType: MemStore, Actor: testactors.AlicePrime},
			{StoreType: MemStore, Actor: testactors.BobPrime},
		},
		ChainPort:     "8546",
		deployerIndex: 1,
	}

	dataFolder, _ := testhelpers.GenerateTempStoreFolder()

	infraL1 := setupSharedInfra(tcL1)
	defer infraL1.Close(t)

	infraL2 := setupSharedInfra(tcL2)
	defer infraL2.Close(t)

	bridgeConfig := bridge.BridgeConfig{
		L1ChainUrl:        infraL1.anvilChain.ChainUrl,
		L1ChainStartBlock: 0,
		ChainPK:           infraL1.anvilChain.ChainPks[tcL1.Participants[1].ChainAccountIndex],
		StateChannelPK:    common.Bytes2Hex(tcL1.Participants[1].PrivateKey),
		NaAddress:         infraL1.anvilChain.ContractAddresses.NaAddress.String(),
		VpaAddress:        infraL1.anvilChain.ContractAddresses.VpaAddress.String(),
		CaAddress:         infraL1.anvilChain.ContractAddresses.CaAddress.String(),
		DurableStoreDir:   dataFolder,
		BridgePublicIp:    DEFAULT_PUBLIC_IP,
		NodeL1MsgPort:     int(tcL1.Participants[1].Port),
		NodeL2MsgPort:     int(tcL2.Participants[1].Port),
	}

	nodeAChainservice, err := chainservice.NewEthChainService(chainservice.ChainOpts{
		ChainUrl:           infraL1.anvilChain.ChainUrl,
		ChainStartBlockNum: 0,
		ChainAuthToken:     infraL1.anvilChain.ChainAuthToken,
		NaAddress:          infraL1.anvilChain.ContractAddresses.NaAddress,
		VpaAddress:         infraL1.anvilChain.ContractAddresses.VpaAddress,
		CaAddress:          infraL1.anvilChain.ContractAddresses.CaAddress,
		ChainPk:            infraL1.anvilChain.ChainPks[tcL1.Participants[0].ChainAccountIndex],
	})
	if err != nil {
		panic(err)
	}

	nodeAPrimeChainservice, err := chainservice.NewLaconicdChainService(chainservice.LaconicdChainOpts{
		VpaAddress: infraL1.anvilChain.ContractAddresses.VpaAddress,
		CaAddress:  infraL1.anvilChain.ContractAddresses.CaAddress,
	})

	bridgeClient, nodeL1MultiAddress, nodeL2MultiAddress, cleanUp := setupBridgeWithRPCClient(t, bridgeConfig)
	defer cleanUp()
	bridgeAddress, _ := bridgeClient.Address()
	nodeARpcClient, _, cleanUp := setupNitroNodeWithRPCClient(t, tcL1.Participants[0].PrivateKey, int(tcL1.Participants[0].Port), int(tcL1.Participants[0].WSPort), 4007, nodeAChainservice, transport.Http, []string{nodeL1MultiAddress})
	defer cleanUp()
	nodeAAddress, _ := nodeARpcClient.Address()

	nodeAPrimeRpcClient, _, cleanUp := setupNitroNodeWithRPCClient(t, tcL2.Participants[0].PrivateKey, int(tcL2.Participants[0].Port), int(tcL2.Participants[0].WSPort), 4008, nodeAPrimeChainservice, transport.Http, []string{nodeL2MultiAddress})
	defer cleanUp()

	var l1LedgerChannelId types.Destination
	var l2LedgerChannelId types.Destination

	t.Run("Create ledger channel on L1 and mirror it on L2", func(t *testing.T) {
		outcome := simpleOutcome(nodeAAddress, bridgeAddress, 100, 100)
		res, err := nodeARpcClient.CreateLedgerChannel(bridgeAddress, 100, outcome)
		if err != nil {
			panic(err)
		}
		l1LedgerChannelId = res.ChannelId

		// Wait for mirror channel creation
		l2LedgerChannelId = <-bridgeClient.CreatedMirrorChannel()

		expectedMirrorChannel := createLedgerInfo(l2LedgerChannelId, simpleOutcome(bridgeAddress, nodeAAddress, 100, 100), query.Open, nodeAAddress)
		actualMirrorChannel, err := nodeAPrimeRpcClient.GetLedgerChannel(l2LedgerChannelId)
		checkError(t, err, "client.GetLedgerChannel")
		checkQueryInfo(t, expectedMirrorChannel, actualMirrorChannel)
	})

	t.Run("Create virtual channel on mirrored ledger channel and make payments", func(t *testing.T) {
		initialOutcome := simpleOutcome(nodeAAddress, bridgeAddress, virtualChannelDeposit, 0)
		virtualChannelResponse, err := nodeAPrimeRpcClient.CreatePaymentChannel(
			nil,
			bridgeAddress,
			100,
			initialOutcome,
		)
		checkError(t, err, "client.CreatePaymentChannel")
		<-nodeAPrimeRpcClient.ObjectiveCompleteChan(virtualChannelResponse.Id)
		_, err = nodeAPrimeRpcClient.Pay(virtualChannelResponse.ChannelId, uint64(payAmount))
		checkError(t, err, "client.Pay")

		outcomeAfterPayment := simpleOutcome(nodeAAddress, bridgeAddress, virtualChannelDeposit-payAmount, payAmount)
		expectedVirtualChannel := createPaychInfo(
			virtualChannelResponse.ChannelId,
			outcomeAfterPayment,
			query.Open,
		)
		actualVirtualChannel, err := nodeAPrimeRpcClient.GetPaymentChannel(virtualChannelResponse.ChannelId)
		checkError(t, err, "client.GetPaymentChannel")
		checkQueryInfo(t, expectedVirtualChannel, actualVirtualChannel)

		virtualDefundResponse, err := nodeAPrimeRpcClient.ClosePaymentChannel(virtualChannelResponse.ChannelId)
		checkError(t, err, "client.ClosePaymentChannel")
		<-nodeAPrimeRpcClient.ObjectiveCompleteChan(virtualDefundResponse)
	})

	t.Run("Exit to L1 using updated L2 ledger channel state after making payments", func(t *testing.T) {
		// Bridged defund is currently disabled
		t.Skip()
		_, err = nodeAPrimeRpcClient.CloseBridgeChannel(l2LedgerChannelId)
		checkError(t, err, "client.CloseBridgeChannel")

		<-nodeARpcClient.ObjectiveCompleteChan(protocols.ObjectiveId(mirrorbridgeddefund.ObjectivePrefix + l1LedgerChannelId.String()))

		expectedMirrorChannel := createLedgerInfo(l1LedgerChannelId, simpleOutcome(nodeAAddress, bridgeAddress, 99, 101), query.Complete, nodeAAddress)
		actualMirrorChannel, err := nodeARpcClient.GetLedgerChannel(l1LedgerChannelId)
		checkError(t, err, "client.GetLedgerChannel")
		checkQueryInfo(t, expectedMirrorChannel, actualMirrorChannel)
	})
}
