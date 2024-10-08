package node_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/go-cmp/cmp"

	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	"github.com/statechannels/go-nitro/internal/logging"
	interRpc "github.com/statechannels/go-nitro/internal/rpc"
	ta "github.com/statechannels/go-nitro/internal/testactors"
	"github.com/statechannels/go-nitro/internal/testdata"
	"github.com/statechannels/go-nitro/internal/testhelpers"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/node/engine"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	p2pms "github.com/statechannels/go-nitro/node/engine/messageservice/p2p-message-service"
	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/paymentsmanager"
	"github.com/statechannels/go-nitro/protocols/directfund"
	"github.com/statechannels/go-nitro/protocols/virtualfund"
	"github.com/statechannels/go-nitro/rpc"
	"github.com/statechannels/go-nitro/rpc/transport"
	"github.com/statechannels/go-nitro/rpc/transport/http"
	natstrans "github.com/statechannels/go-nitro/rpc/transport/nats"
	"github.com/statechannels/go-nitro/types"

	"github.com/statechannels/go-nitro/crypto"
)

func simpleOutcome(a, b types.Address, aBalance, bBalance uint) outcome.Exit {
	return testdata.Outcomes.Create(a, b, uint64(aBalance), uint64(bBalance), types.Address{})
}

func TestRpcWithNats(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		executeNRpcTestWrapper(t, "nats", n, false)
	}
}

func TestRpcWithHttp(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		executeNRpcTestWrapper(t, transport.Http, n, false)
	}
}

func TestRPCWithManualVoucherExchange(t *testing.T) {
	executeNRpcTestWrapper(t, transport.Http, 4, true)
	executeNRpcTestWrapper(t, transport.Nats, 4, true)
}

func executeNRpcTestWrapper(t *testing.T, connectionType transport.TransportType, n int, manualVoucherExchange bool) {
	testName := fmt.Sprintf("%d_clients", n)
	t.Run(testName, func(t *testing.T) {
		executeNRpcTest(t, connectionType, n, manualVoucherExchange)
	})
}

func executeNRpcTest(t *testing.T, connectionType transport.TransportType, n int, manualVoucherExchange bool) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Test panicked: %v", r)
			t.FailNow()
		}
	}()

	if n < 2 {
		t.Errorf("n must be at least 2: alice and bob")
		return
	}

	//////////////////////
	// Setup
	//////////////////////

	manVoucherStr := "_with_manual_voucher_exchange"
	if !manualVoucherExchange {
		manVoucherStr = ""
	}
	logFile := fmt.Sprintf("test_%d_rpc_clients_over_%s%s.log", n, connectionType, manVoucherStr)
	logging.SetupDefaultFileLogger(logFile, slog.LevelDebug)

	slog.Info("Starting test", "num-clients", n)

	chain := chainservice.NewMockChain()
	defer chain.Close()

	// create n actors
	actors := make([]ta.Actor, n)
	for i := 0; i < n; i++ {
		sk := `000000000000000000000000000000000000000000000000000000000000000` + strconv.Itoa(i+1)
		actors[i] = ta.Actor{
			PrivateKey: common.Hex2Bytes(sk),
		}
	}
	slog.Info("Actors created", "num-actors", n)

	chainServices := make([]*chainservice.MockChainService, n)
	for i := 0; i < n; i++ {
		chainServices[i] = chainservice.NewMockChainService(chain, actors[i].Address())
	}

	clients := make([]rpc.RpcClientApi, n)
	msgServices := make([]*p2pms.P2PMessageService, n)
	bootPeers := []string{}
	// Set up the intermediaries
	if n > 2 {
		for i := 1; i < n-1; i++ {
			rpcClient, msg, cleanup := setupNitroNodeWithRPCClient(t, actors[i].PrivateKey, 3105+i, 6105+i, 4105+i, chainServices[i], connectionType, []string{})
			clients[i] = rpcClient
			msgServices[i] = msg
			bootPeers = append(bootPeers, msg.MultiAddr)
			defer cleanup()
		}
	}

	// Set up the first and last client
	for i := 0; i < n; i = i + (n - 1) {
		rpcClient, msg, cleanup := setupNitroNodeWithRPCClient(t, actors[i].PrivateKey, 3105+i, 6105+i, 4105+i, chainServices[i], connectionType, bootPeers)
		clients[i] = rpcClient
		msgServices[i] = msg
		defer cleanup()
		// If there are only 2 clients then the first client is the boot peer
		if n == 2 && i == 0 {
			bootPeers = append(bootPeers, msg.MultiAddr)
		}
	}

	slog.Info("Clients created", "num-clients", n)

	slog.Info("Verify that each rpc client fetches the correct address")
	for i := 0; i < n; i++ {
		clientAddress, _ := clients[i].Address()
		if !cmp.Equal(actors[i].Address(), clientAddress) {
			t.Fatalf("expected address %s, got %s", actors[i].Address(), clientAddress)
		}
	}

	waitForPeerInfoExchange(msgServices...)
	slog.Info("Peer exchange complete")

	// create n-1 ledger channels
	ledgerChannels := make([]directfund.ObjectiveResponse, n-1)
	for i := 0; i < n-1; i++ {
		outcome := simpleOutcome(actors[i].Address(), actors[i+1].Address(), 100, 100)
		var err error
		ledgerChannels[i], err = clients[i].CreateLedgerChannel(actors[i+1].Address(), 100, outcome)
		checkError(t, err, "client.CreateLedgerChannel")

		if !directfund.IsDirectFundObjective(ledgerChannels[i].Id) {
			t.Errorf("expected direct fund objective, got %s", ledgerChannels[i].Id)
		}
	}
	// wait for the ledger channels to be ready for each client
	for i, client := range clients {
		if i != 0 { // not alice
			<-client.ObjectiveCompleteChan(ledgerChannels[i-1].Id) // left channel
		}
		if i != n-1 { // not bob
			<-client.ObjectiveCompleteChan(ledgerChannels[i].Id) // right channel
		}
	}
	slog.Info("Ledger channels created")

	// try to create duplicate ledger channel to ensure node correctly
	// handles error without panicking
	{
		outcome := simpleOutcome(actors[0].Address(), actors[1].Address(), 100, 100)
		duplicateLedgerChannelObjective, err := clients[0].CreateLedgerChannel(actors[1].Address(), 100, outcome)
		if err == nil {
			t.Error("expected error when creating duplicate ledger channel")
		}

		if directfund.IsDirectFundObjective(duplicateLedgerChannelObjective.Id) {
			t.Errorf("directfund objective should not have been created for duplicate ledger channel")
		}
	}

	// assert existence & reporting of expected ledger channels
	for i, client := range clients {
		if i != 0 {
			leftLC := ledgerChannels[i-1]
			expectedLeftLC := createLedgerInfo(leftLC.ChannelId, simpleOutcome(actors[i-1].Address(), actors[i].Address(), 100, 100), query.Open, channel.Open, actors[i].Address())
			actualLeftLC, err := client.GetLedgerChannel(leftLC.ChannelId)
			checkError(t, err, "client.GetLedgerChannel")
			checkQueryInfo(t, expectedLeftLC, actualLeftLC)
		}
		if i != n-1 {
			rightLC := ledgerChannels[i]
			expectedRightLC := createLedgerInfo(rightLC.ChannelId, simpleOutcome(actors[i].Address(), actors[i+1].Address(), 100, 100), query.Open, channel.Open, actors[i].Address())
			actualRightLC, err := client.GetLedgerChannel(rightLC.ChannelId)
			checkError(t, err, "client.GetLedgerChannel")
			checkQueryInfo(t, expectedRightLC, actualRightLC)
		}
	}

	t.Log("Ledger channels queried")

	//////////////////////////////////////////////////////////////////
	// create virtual channel, execute payment, close virtual channel
	//////////////////////////////////////////////////////////////////

	intermediaries := make([]types.Address, len(actors)-2)
	for i, actor := range actors[1 : len(actors)-1] {
		intermediaries[i] = actor.Address()
	}

	alice := actors[0]
	aliceClient := clients[0]
	bob := actors[n-1]
	bobClient := clients[n-1]
	aliceLedger := ledgerChannels[0]
	bobLedger := ledgerChannels[n-2]

	initialOutcome := simpleOutcome(actors[0].Address(), actors[n-1].Address(), 100, 0)

	vabCreateResponse, err := aliceClient.CreatePaymentChannel(
		intermediaries,
		bob.Address(),
		100,
		initialOutcome,
	)
	checkError(t, err, "client.CreatePaymentChannel")
	expectedVirtualChannel := createPaychInfo(
		vabCreateResponse.ChannelId,
		initialOutcome,
		query.Open,
	)

	_, err = aliceClient.GetPaymentChannel(types.Destination{0x000}) // Confirms server won't crash if invalid chId is provided
	if err == nil {
		t.Error("expected error for client.GetPaymentChannel(types.Destination{0x000})")
	}

	// wait for the virtual channel to be ready, and
	// assert correct reporting from query api
	for i, client := range clients {
		<-client.ObjectiveCompleteChan(vabCreateResponse.Id)
		channelInfo, err := client.GetPaymentChannel(vabCreateResponse.ChannelId)
		checkError(t, err, "client.GetPaymentChannel")
		checkQueryInfo(t, expectedVirtualChannel, channelInfo)
		if i != 0 {
			channelsByLedger, err := client.GetPaymentChannelsByLedger(ledgerChannels[i-1].ChannelId)
			checkError(t, err, "client.GetPaymentChannelsByLedger")
			checkQueryInfoCollection(t, expectedVirtualChannel, 1, channelsByLedger)
		}
		if i != n-1 {
			channelsByLedger, err := client.GetPaymentChannelsByLedger(ledgerChannels[i].ChannelId)
			checkError(t, err, "client.GetPaymentChannelsByLedger")
			checkQueryInfoCollection(t, expectedVirtualChannel, 1, channelsByLedger)
		}
	}

	t.Log("Payment channels queried")

	if !virtualfund.IsVirtualFundObjective(vabCreateResponse.Id) {
		t.Errorf("expected virtual fund objective, got %s", vabCreateResponse.Id)
	}

	if manualVoucherExchange {
		v, err := aliceClient.CreateVoucher(vabCreateResponse.ChannelId, 1)
		checkError(t, err, "aliceClient.CreateVoucher")

		rxVoucher, err := bobClient.ReceiveVoucher(v)
		checkError(t, err, "bobClient.ReceiveVoucher")

		if rxVoucher.Total.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("expected a total of 1 got %d", rxVoucher.Total)
		}
		if rxVoucher.Delta.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("expected a delta of 1 got %d", rxVoucher.Delta)
		}

		rxVoucher, err = bobClient.ReceiveVoucher(v)
		checkError(t, err, "bobClient.ReceiveVoucher")

		if rxVoucher.Delta.Cmp(big.NewInt(0)) != 0 {
			t.Errorf("adding the same voucher should result in a delta of 0, got %d", rxVoucher.Delta)
		}
	} else {
		_, err = aliceClient.Pay(vabCreateResponse.ChannelId, 1)
		checkError(t, err, "aliceClient.Pay")
	}

	t.Log("Vouchers sent/received")

	vabClosure, _ := aliceClient.ClosePaymentChannel(vabCreateResponse.ChannelId)
	for _, client := range clients {
		<-client.ObjectiveCompleteChan(vabClosure)
	}

	laiClosure, _ := aliceClient.CloseLedgerChannel(aliceLedger.ChannelId, false)
	<-aliceClient.ObjectiveCompleteChan(laiClosure)

	if n != 2 { // for n=2, alice and bob share a ledger, which should only be closed once.
		libClosure, _ := bobClient.CloseLedgerChannel(bobLedger.ChannelId, false)
		<-bobClient.ObjectiveCompleteChan(libClosure)
	}

	t.Log("Ledger/virtual channels closed")

	//////////////////////////
	// perform wrap-up checks
	//////////////////////////

	for i, client := range clients {
		if i != 0 {
			leftLC := ledgerChannels[i-1]
			paymentChannels, err := client.GetPaymentChannelsByLedger(leftLC.ChannelId)
			checkError(t, err, "client.GetPaymentChannelsByLedger")
			if len(paymentChannels) != 0 {
				t.Errorf("expected no virtual channels in ledger channel %s, got %d", leftLC.ChannelId, len(paymentChannels))
			}
		}
		if i != n-1 {
			rightLC := ledgerChannels[i]
			paymentChannels, err := client.GetPaymentChannelsByLedger(rightLC.ChannelId)
			checkError(t, err, "client.GetPaymentChannelsByLedger")
			if len(paymentChannels) != 0 {
				t.Errorf("expected no virtual channels in ledger channel %s, got %d", rightLC.ChannelId, len(paymentChannels))
			}
		}
	}

	aliceLedgerNotifs := aliceClient.LedgerChannelUpdatesChan(ledgerChannels[0].ChannelId)
	expectedAliceLedgerNotifs := createLedgerStory(
		aliceLedger.ChannelId, alice.Address(), actors[1].Address(), // actor[1] is the first intermediary - can be Bob if n=2 (0-hop)
		[]channelStatusShorthand{
			{100, 100, query.Proposed, channel.Open},
			{100, 100, query.Open, channel.Open},
			{0, 100, query.Open, channel.Open},  // alice's balance forwarded to the guarantee for the virtual channel
			{99, 101, query.Open, channel.Open}, // returns to alice & actors[1] after closure
			{99, 101, query.Closing, channel.Open},
			{99, 101, query.Complete, channel.Open}, // Since mockChain's block time stamp is always 0, the channel mode always remains open
		},
	)[alice.Address()]
	checkNotifications(t, "aliceLedger", expectedAliceLedgerNotifs, []query.LedgerChannelInfo{}, aliceLedgerNotifs, defaultTimeout)

	bobLedgerNotifs := bobClient.LedgerChannelUpdatesChan(bobLedger.ChannelId)
	expectedBobLedgerNotifs := createLedgerStory(
		bobLedger.ChannelId, actors[n-2].Address(), bob.Address(),
		[]channelStatusShorthand{
			{100, 100, query.Proposed, channel.Open},
			{100, 100, query.Open, channel.Open},
			{0, 100, query.Open, channel.Open},
			{99, 101, query.Open, channel.Open},
			{99, 101, query.Complete, channel.Open}, // Since mockChain's block time stamp is always 0, the channel mode always remains open
		},
	)[bob.Address()]
	if n != 2 { // bob does not trigger a ledger-channel close if n=2 - alice does
		expectedBobLedgerNotifs = append(expectedBobLedgerNotifs,
			createLedgerInfo(bobLedger.ChannelId, simpleOutcome(actors[n-2].Address(), bob.Address(), 99, 101), query.Closing, channel.Open, bob.Address()),
		)
	}
	checkNotifications(t, "bobLedger", expectedBobLedgerNotifs, []query.LedgerChannelInfo{}, bobLedgerNotifs, defaultTimeout)

	requiredVCNotifs := createPaychStory(
		vabCreateResponse.ChannelId, alice.Address(), bob.Address(),
		[]channelStatusShorthand{
			{100, 0, query.Proposed, channel.Open},
			{100, 0, query.Open, channel.Open},
			{99, 1, query.Complete, channel.Open}, // Since mockChain's block time stamp is always 0, the channel mode always remains open
		},
	)
	optionalVCNotifs := createPaychStory(
		vabCreateResponse.ChannelId, alice.Address(), bob.Address(),
		[]channelStatusShorthand{
			{99, 1, query.Closing, channel.Open},
			// TODO: Sometimes we see a closing notification with the original balance.
			// See https://github.com/statechannels/go-nitro/issues/1306
			{99, 1, query.Open, channel.Open},
			{100, 0, query.Closing, channel.Open},
		},
	)

	aliceVirtualNotifs := aliceClient.PaymentChannelUpdatesChan(vabCreateResponse.ChannelId)
	checkNotifications(t, "aliceVirtual", requiredVCNotifs, optionalVCNotifs, aliceVirtualNotifs, defaultTimeout)
	bobVirtualNotifs := bobClient.PaymentChannelUpdatesChan(vabCreateResponse.ChannelId)
	checkNotifications(t, "bobVirtual", requiredVCNotifs, optionalVCNotifs, bobVirtualNotifs, defaultTimeout)
}

// setupNitroNodeWithRPCClient is a helper function that spins up a Nitro Node RPC Server and returns an RPC client connected to it.
func setupNitroNodeWithRPCClient(
	t *testing.T,
	pkBytes []byte,
	msgPort int,
	wsMsgPort int,
	rpcPort int,
	chain chainservice.ChainService,
	connectionType transport.TransportType,
	bootPeers []string,
) (rpc.RpcClientApi, *p2pms.P2PMessageService, func()) {
	dataFolder, cleanupData := testhelpers.GenerateTempStoreFolder()
	ourStore, err := store.NewStore(store.StoreOpts{
		PkBytes:            pkBytes,
		UseDurableStore:    true,
		DurableStoreFolder: dataFolder,
	})
	if err != nil {
		t.Fatal(err)
	}

	slog.Info("Initializing message service on port " + fmt.Sprint(msgPort) + "...")
	messageService := p2pms.NewMessageService(p2pms.MessageOpts{
		PkBytes:   pkBytes,
		TcpPort:   msgPort,
		WsMsgPort: wsMsgPort,
		BootPeers: bootPeers,
		PublicIp:  "127.0.0.1",
		SCAddr:    *ourStore.GetAddress(),
	})

	node := node.New(
		messageService,
		chain,
		ourStore,
		&engine.PermissivePolicy{})

	var useNats bool
	switch connectionType {
	case transport.Nats:
		useNats = true
	case transport.Http:
		useNats = false
	default:
		err = fmt.Errorf("unknown connection type %v", connectionType)
		panic(err)
	}

	cert, err := tls.LoadX509KeyPair("../tls/statechannels.org.pem", "../tls/statechannels.org_key.pem")
	if err != nil {
		panic(err)
	}

	paymentsManager := paymentsmanager.PaymentsManager{}
	rpcServer, err := interRpc.InitializeNodeRpcServer(&node, paymentsManager, rpcPort, useNats, &cert)
	if err != nil {
		t.Fatal(err)
	}

	var clientConnection transport.Requester
	switch connectionType {
	case transport.Nats:

		clientConnection, err = natstrans.NewNatsTransportAsClient(rpcServer.Url())
		if err != nil {
			panic(err)
		}
	case transport.Http:

		clientConnection, err = http.NewHttpTransportAsClient(rpcServer.Url(), true, 10*time.Millisecond)
		if err != nil {
			panic(err)
		}
	default:
		err = fmt.Errorf("unknown connection type %v", connectionType)
		panic(err)
	}

	rpcClient, err := rpc.NewRpcClient(clientConnection)
	if err != nil {
		panic(err)
	}

	cleanupFn := func() {
		// Setup a logger with the address of the node so we know who is closing
		me := crypto.GetAddressFromSecretKeyBytes(pkBytes)
		logger := logging.LoggerWithAddress(slog.Default(), me)
		logger.Info("Starting rpc close")
		rpcClient.Close()
		logger.Info("Rpc client closed")
		rpcServer.Close()
		logger.Info("Rpc server closed")

		cleanupData()
	}
	return rpcClient, messageService, cleanupFn
}

type channelInfo interface {
	query.LedgerChannelInfo | query.PaymentChannelInfo
}

func checkError(t *testing.T, err error, msg string) {
	if err != nil {
		t.Error(msg + ": " + err.Error())
	}
}

func checkQueryInfo[T channelInfo](t *testing.T, expected T, fetched T) {
	if diff := cmp.Diff(expected, fetched, cmp.AllowUnexported(big.Int{})); diff != "" {
		t.Errorf("Channel query info diff mismatch (-want +got):\n%s", diff)
	}
}

func checkQueryInfoCollection[T channelInfo](t *testing.T, expected T, expectedLength int, fetched []T) {
	if len(fetched) != expectedLength {
		t.Fatalf("expected %d channel infos, got %d", expectedLength, len(fetched))
	}
	found := false
	for _, fetched := range fetched {
		if cmp.Equal(expected, fetched, cmp.AllowUnexported(big.Int{})) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("did not find info %v in channel infos: %v", expected, fetched)
	}
}

// marshalToJson marshals the given object to json and returns the string representation.
func marshalToJson[T channelInfo](t *testing.T, info T) string {
	jsonBytes, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	return string(jsonBytes)
}

// checkNotifications checks that notifications are received on the notifChan.
//
// required specifies the notifications that must be received. checkNotifications will fail
// if any of these notifications are not received.
//
// optional specifies notifications that may be received. checkNotifications will not fail
// if any of these notifications are not received.
//
// If a notification is received that is neither in required or optional, checkNotifications will fail.
func checkNotifications[T channelInfo](t *testing.T, client string, required []T, optional []T, notifChan <-chan T, timeout time.Duration) {
	// This is map containing both required and optional notifications.
	// We use the json representation of the notification as the key and a boolean as the value.
	// The boolean value is true if the notification is required and false if it is optional.
	// When a notification is received it is removed from acceptableNotifications
	acceptableNotifications := make(map[string]bool)
	unexpectedNotifications := make(map[string]bool)
	logUnexpected := func() {
		for notif := range unexpectedNotifications {
			slog.Info("Unexpected notification", "client", client, "notification", notif)
		}
	}

	for _, r := range required {
		acceptableNotifications[marshalToJson(t, r)] = true
	}
	for _, o := range optional {
		acceptableNotifications[marshalToJson(t, o)] = false
	}

	for !areRequiredComplete(acceptableNotifications) {
		select {
		case info := <-notifChan:

			notifJSON := marshalToJson(t, info)
			slog.Info("Received notification", "client", client, "notification", info)

			// Check that the notification is a required or optional one.
			_, isExpected := acceptableNotifications[notifJSON]

			if isExpected {
				// To signal we received a notification we delete it from the map
				delete(acceptableNotifications, notifJSON)
			} else {
				unexpectedNotifications[notifJSON] = true
			}

		case <-time.After(timeout):
			logUnexpected()
			// Log both to the test log file and to stdout
			failMsg := fmt.Sprintf("%s timed out waiting for notification(s): \n%v", client, incompleteRequired(acceptableNotifications))
			slog.Error(failMsg)
			t.Fatalf(failMsg)
		}
	}
	if len(unexpectedNotifications) > 0 {
		logUnexpected()
		t.FailNow()
	}
}

// incompleteRequired returns a debug string listing
// required notifications that have not been received.
func incompleteRequired(notifs map[string]bool) string {
	required := ""
	for k, isRequired := range notifs {
		if isRequired {
			required += k + "\n"
		}
	}
	return required
}

// areRequiredComplete checks if all the required notifications have been received.
// It does this by checking that there are no members of the map that are true.
func areRequiredComplete(notifs map[string]bool) bool {
	for _, isRequired := range notifs {
		if isRequired {
			return false
		}
	}
	return true
}
