package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/channel/state"
	nodeutils "github.com/statechannels/go-nitro/internal/node"
	"github.com/statechannels/go-nitro/internal/safesync"
	"github.com/statechannels/go-nitro/node"
	p2pms "github.com/statechannels/go-nitro/node/engine/messageservice/p2p-message-service"
	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/protocols/bridgeddefund"
	"github.com/statechannels/go-nitro/protocols/bridgedfund"
	"github.com/statechannels/go-nitro/protocols/directfund"
	"github.com/tidwall/buntdb"

	"github.com/statechannels/go-nitro/node/engine/chainservice"
	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

const (
	L1_DURABLE_STORE_SUB_DIR = "l1-node"
	L2_DURABLE_STORE_SUB_DIR = "l2-node"
)

const RETRY_TX_LIMIT = 1

type SentTx struct {
	Tx                  protocols.ChainTransaction `json:"tx"`
	NumOfRetries        uint                       `json:"num_of_retries"`
	IsRetryLimitReached bool                       `json:"is_retry_limit_reached"`
	IsL2                bool                       `json:"is_l2"`
}

type PendingTx struct {
	SentTx
	TxHash string `json:"tx_hash"`
}

type Bridge struct {
	bridgeStore *DurableStore

	nodeL1         *node.Node
	storeL1        store.Store
	chainServiceL1 chainservice.ChainService

	nodeL2         *node.Node
	storeL2        store.Store
	chainServiceL2 chainservice.ChainService

	cancel                context.CancelFunc
	mirrorChannelMap      map[types.Destination]MirrorChannelDetails
	createdMirrorChannels chan types.Destination
	sentTxs               safesync.Map[SentTx]
}

type BridgeConfig struct {
	L1ChainUrl         string
	L1ChainStartBlock  uint64
	ChainPK            string
	StateChannelPK     string
	NaAddress          string
	VpaAddress         string
	CaAddress          string
	BridgeAddress      string
	DurableStoreDir    string
	BridgePublicIp     string
	NodeL1ExtMultiAddr string
	NodeL2ExtMultiAddr string
	NodeL1MsgPort      int
	NodeL2MsgPort      int
}

func New() *Bridge {
	bridge := Bridge{
		mirrorChannelMap:      make(map[types.Destination]MirrorChannelDetails),
		createdMirrorChannels: make(chan types.Destination),
	}

	return &bridge
}

func (b *Bridge) Start(configOpts BridgeConfig) (nodeL1 *node.Node, nodeL2 *node.Node, nodeL1MultiAddress string, nodeL2MultiAddress string, err error) {
	chainOptsL2 := chainservice.LaconicdChainOpts{
		VpaAddress: common.HexToAddress(configOpts.VpaAddress),
		CaAddress:  common.HexToAddress(configOpts.CaAddress),
	}

	chainOptsL1 := chainservice.ChainOpts{
		ChainUrl:           configOpts.L1ChainUrl,
		ChainStartBlockNum: configOpts.L1ChainStartBlock,
		ChainPk:            configOpts.ChainPK,
		NaAddress:          common.HexToAddress(configOpts.NaAddress),
		VpaAddress:         common.HexToAddress(configOpts.VpaAddress),
		CaAddress:          common.HexToAddress(configOpts.CaAddress),
	}

	storeOptsL1 := store.StoreOpts{
		PkBytes:            common.Hex2Bytes(configOpts.StateChannelPK),
		UseDurableStore:    true,
		DurableStoreFolder: filepath.Join(configOpts.DurableStoreDir, L1_DURABLE_STORE_SUB_DIR),
	}

	storeOptsL2 := store.StoreOpts{
		PkBytes:            common.Hex2Bytes(configOpts.StateChannelPK),
		UseDurableStore:    true,
		DurableStoreFolder: filepath.Join(configOpts.DurableStoreDir, L2_DURABLE_STORE_SUB_DIR),
	}

	messageOptsL1 := p2pms.MessageOpts{
		PkBytes:      common.Hex2Bytes(configOpts.StateChannelPK),
		TcpPort:      configOpts.NodeL1MsgPort,
		BootPeers:    nil,
		PublicIp:     configOpts.BridgePublicIp,
		ExtMultiAddr: configOpts.NodeL1ExtMultiAddr,
	}

	messageOptsL2 := p2pms.MessageOpts{
		PkBytes:      common.Hex2Bytes(configOpts.StateChannelPK),
		TcpPort:      configOpts.NodeL2MsgPort,
		BootPeers:    nil,
		PublicIp:     configOpts.BridgePublicIp,
		ExtMultiAddr: configOpts.NodeL2ExtMultiAddr,
	}

	// Initialize nodes
	nodeL1, storeL1, msgServiceL1, chainServiceL1, err := nodeutils.InitializeNode(chainOptsL1, storeOptsL1, messageOptsL1, &NodeL1PermissivePolicy{})
	if err != nil {
		return nil, nil, nodeL1MultiAddress, nodeL2MultiAddress, err
	}

	nodeL2, storeL2, msgServiceL2, chainServiceL2, err := nodeutils.InitializeL2Node(chainOptsL2, storeOptsL2, messageOptsL2)
	if err != nil {
		return nil, nil, nodeL1MultiAddress, nodeL2MultiAddress, err
	}

	b.nodeL1 = nodeL1
	b.storeL1 = *storeL1
	b.chainServiceL1 = chainServiceL1
	b.nodeL2 = nodeL2
	b.storeL2 = *storeL2
	b.chainServiceL2 = chainServiceL2

	ctx, cancelFunc := context.WithCancel(context.Background())
	b.cancel = cancelFunc

	ds, err := NewDurableStore(configOpts.DurableStoreDir, buntdb.Config{})
	if err != nil {
		return nil, nil, nodeL1MultiAddress, nodeL2MultiAddress, err
	}

	b.bridgeStore = ds

	go b.listenForDroppedEvents(ctx)
	go b.run(ctx)

	return nodeL1, nodeL2, msgServiceL1.MultiAddr, msgServiceL2.MultiAddr, nil
}

func (b *Bridge) run(ctx context.Context) {
	completedObjectivesInNodeL1 := b.nodeL1.CompletedObjectives()
	completedObjectivesInNodeL2 := b.nodeL2.CompletedObjectives()

	for {
		var err error
		select {
		case objId, ok := <-completedObjectivesInNodeL1:
			if ok {
				err = b.processCompletedObjectivesFromL1(objId)
				b.checkError(err)
			}

		case objId, ok := <-completedObjectivesInNodeL2:
			if ok {
				err = b.processCompletedObjectivesFromL2(objId)
				b.checkError(err)
			}

		case <-ctx.Done():
			return
		}
	}
}

func (b *Bridge) processCompletedObjectivesFromL1(objId protocols.ObjectiveId) error {
	obj, err := b.storeL1.GetObjectiveById(objId)
	if err != nil {
		return fmt.Errorf("error in getting objective %w", err)
	}

	// If objectiveId corresponds to direct fund objective
	// Create new outcome for mirrored ledger channel based on L1 ledger channel
	// Create mirrored ledger channel on L2 based on created outcome
	ddFo, isDdfo := obj.(*directfund.Objective)
	if !isDdfo {
		return nil
	}

	channelId := ddFo.OwnsChannel()
	slog.Debug("Creating mirror outcome for L2", "channelId", channelId)
	l1LedgerChannel, err := b.storeL1.GetConsensusChannelById(channelId)
	if err != nil {
		return err
	}

	l1ledgerChannelState := l1LedgerChannel.SupportedSignedState()
	l1ledgerChannelStateClone := l1ledgerChannelState.Clone()
	// Put NodeBPrime's allocation at index 0 as it creates mirrored ledger channel
	// Swap the allocations to be set in mirrored ledger channel
	for _, outcome := range l1ledgerChannelStateClone.State().Outcome {
		// Swap the allocations in place
		// Allocations have at least two elements before performing the swap
		if len(outcome.Allocations) >= 2 {
			outcome.Allocations[0], outcome.Allocations[1] = outcome.Allocations[1], outcome.Allocations[0]
		}
	}

	// Create extended state outcome based on l1ChannelState
	l1ChannelCloneOutcome := l1ledgerChannelStateClone.State().Outcome

	// Create mirrored ledger channel between node BPrime and APrime
	// TODO: Support mirrored ledger channel creation with multiple assets
	l2LedgerChannelResponse, err := b.nodeL2.CreateBridgeChannel(l1ledgerChannelStateClone.State().Participants[0], l1ledgerChannelStateClone.State().ChallengeDuration, l1ChannelCloneOutcome)
	if err != nil {
		return err
	}

	err = b.bridgeStore.SetMirrorChannelDetails(l2LedgerChannelResponse.ChannelId, MirrorChannelDetails{L1ChannelId: l1LedgerChannel.Id})
	if err != nil {
		return err
	}

	slog.Debug("Started creating mirror ledger channel in L2", "channelId", l2LedgerChannelResponse.ChannelId)
	return nil
}

func (b *Bridge) processCompletedObjectivesFromL2(objId protocols.ObjectiveId) error {
	obj, err := b.storeL2.GetObjectiveById(objId)
	if err != nil {
		return fmt.Errorf("error in getting objective %w", err)
	}

	switch objective := obj.(type) {

	case *bridgedfund.Objective:
		l2channelId := objective.OwnsChannel()

		mirrorChannelDetails, err := b.bridgeStore.GetMirrorChannelDetails(l2channelId)
		if err != nil {
			return err
		}

		err = b.bridgeStore.SetMirrorChannelDetails(l2channelId, MirrorChannelDetails{L1ChannelId: mirrorChannelDetails.L1ChannelId, IsCreated: true})
		if err != nil {
			return err
		}

		// Node B calls contract method to store L2ChannelId => L1ChannelId
		setL2ToL1TxToSubmit := protocols.NewSetL2ToL1Transaction(mirrorChannelDetails.L1ChannelId, l2channelId)
		setL2ToL1Tx, err := b.chainServiceL1.SendTransaction(setL2ToL1TxToSubmit)
		if err != nil {
			return fmt.Errorf("error in send transaction %w", err)
		}

		b.sentTxs.Store(setL2ToL1Tx.Hash().String(), SentTx{setL2ToL1TxToSubmit, 0, false, false})

		// use a nonblocking send in case no one is listening
		select {
		case b.createdMirrorChannels <- l2channelId:
		default:
		}

	case *bridgeddefund.Objective:
		// Get latest supported signed state of L2
		signedState, err := objective.C.LatestSupportedSignedState()
		if err != nil {
			return fmt.Errorf("error in latest supported signed state: %w", err)
		}

		// Get L1 ledger channel Id
		mirrorInfo, err := b.bridgeStore.GetMirrorChannelDetails(obj.OwnsChannel())
		if err != nil {
			return fmt.Errorf("error in getting mirror channel details: %w", err)
		}

		// Initiate mirror bridged defund on L1 using L2 signed state
		_, err = b.nodeL1.MirrorBridgedDefund(mirrorInfo.L1ChannelId, signedState, false)
		if err != nil {
			return fmt.Errorf("error in initiating mirror bridged defund: %w", err)
		}
	}

	return nil
}

// Since bridge node addresses are same
func (b *Bridge) GetBridgeAddress() common.Address {
	return *b.nodeL1.Address
}

func (b *Bridge) GetL2SupportedSignedState(id types.Destination) (state.SignedState, error) {
	return b.nodeL2.GetSignedState(id)
}

func (b *Bridge) MirrorBridgedDefund(l1ChannelId types.Destination, l2SignedState state.SignedState, isChallenge bool) (protocols.ObjectiveId, error) {
	return b.nodeL1.MirrorBridgedDefund(l1ChannelId, l2SignedState, isChallenge)
}

func (b *Bridge) CounterChallenge(id types.Destination, action types.CounterChallengeAction, payload state.SignedState) {
	b.nodeL1.CounterChallenge(id, action, payload)
}

func (b *Bridge) GetL2ChannelIdByL1ChannelId(l1ChannelId types.Destination) (l2ChannelId types.Destination, isCreated bool) {
	var err error
	l2ChannelId, isCreated, err = b.bridgeStore.GetMirrorChannelDetailsByL1Channel(l1ChannelId)
	if err != nil {
		return l2ChannelId, isCreated
	}

	return l2ChannelId, isCreated
}

func (b *Bridge) GetL2ObjectiveByL1ObjectiveId(l1ObjectiveId protocols.ObjectiveId) (protocols.Objective, error) {
	l1Objective, err := b.storeL1.GetObjectiveById(l1ObjectiveId)
	if err != nil {
		return nil, err
	}

	l1ChannelId := l1Objective.OwnsChannel()
	l2ChannelId, _ := b.GetL2ChannelIdByL1ChannelId(l1ChannelId)

	if l2ChannelId.IsZero() {
		return nil, fmt.Errorf("could not find L2 channel for given L1 objective ID")
	}

	l2Objective, ok := b.storeL2.GetObjectiveByChannelId(l2ChannelId)
	if !ok {
		return nil, fmt.Errorf("corresponding L2 objective is either complete or does not exist")
	}

	return l2Objective, nil
}

func (b *Bridge) GetObjectiveById(objectiveId protocols.ObjectiveId, l2 bool) (protocols.Objective, error) {
	if l2 {
		return b.nodeL2.GetObjectiveById(objectiveId)
	}
	return b.nodeL1.GetObjectiveById(objectiveId)
}

func (b *Bridge) GetAllL2Channels() ([]query.LedgerChannelInfo, error) {
	return b.nodeL2.GetAllLedgerChannels()
}

func (b *Bridge) CreatedMirrorChannels() <-chan types.Destination {
	return b.createdMirrorChannels
}

func (b *Bridge) RetryObjectiveTx(objectiveId protocols.ObjectiveId) error {
	if bridgedfund.IsBridgedFundObjective(objectiveId) {
		b.nodeL2.RetryObjectiveTx(objectiveId)
		return nil
	}
	if directfund.IsDirectFundObjective(objectiveId) {
		b.nodeL1.RetryObjectiveTx(objectiveId)
		return nil
	}
	return fmt.Errorf("objective with given Id is not supported for retrying")
}

func (b *Bridge) RetryTx(txHash common.Hash) error {
	var chainService chainservice.ChainService

	txToRetry, ok := b.sentTxs.Load(txHash.String())
	if !ok {
		return fmt.Errorf("tx with given hash %s was either complete or cannot be found", txHash)
	}

	if !txToRetry.IsRetryLimitReached {
		return fmt.Errorf("tx with given hash %s is pending confirmation and connot be retried", txHash)
	}

	if txToRetry.IsL2 {
		chainService = b.chainServiceL2
	} else {
		chainService = b.chainServiceL1
	}

	_, err := chainService.SendTransaction(txToRetry.Tx)
	if err != nil {
		return err
	}

	return nil
}

func (b *Bridge) Close() error {
	b.cancel()
	err := b.nodeL1.Close()
	if err != nil {
		return err
	}

	err = b.nodeL2.Close()
	if err != nil {
		return err
	}

	return b.bridgeStore.Close()
}

func (b *Bridge) checkError(err error) {
	if err != nil {
		slog.Error("error in run loop", "error", err)
	}
}

func (b *Bridge) listenForDroppedEvents(ctx context.Context) {
	var err error
	for {
		select {
		case l1DroppedEvent := <-b.chainServiceL1.DroppedEventFeed():
			err = b.checkAndRetryDroppedTxs(l1DroppedEvent, b.chainServiceL1, false)

		case l2DroppedEvent := <-b.chainServiceL2.DroppedEventFeed():
			err = b.checkAndRetryDroppedTxs(l2DroppedEvent, b.chainServiceL2, true)

		case l1ConfirmedEvent := <-b.chainServiceL1.EventFeed():
			b.sentTxs.Delete(l1ConfirmedEvent.TxHash().String())

		case l2ConfirmedEvent := <-b.chainServiceL2.EventFeed():
			b.sentTxs.Delete(l2ConfirmedEvent.TxHash().String())

		case <-ctx.Done():
			return
		}

		b.checkError(err)
	}
}

func (b *Bridge) checkAndRetryDroppedTxs(droppedEvent protocols.DroppedEventInfo, chainservice chainservice.ChainService, isL2 bool) error {
	txToRetry, ok := b.sentTxs.Load(droppedEvent.TxHash.String())
	if !ok {
		return nil
	}

	if txToRetry.NumOfRetries >= RETRY_TX_LIMIT {
		txToRetry.IsRetryLimitReached = true
		b.sentTxs.Store(droppedEvent.TxHash.String(), txToRetry)
		return nil
	}

	retriedTx, err := chainservice.SendTransaction(txToRetry.Tx)
	if err != nil {
		return err
	}

	b.sentTxs.Delete(droppedEvent.TxHash.String())
	b.sentTxs.Store(retriedTx.Hash().String(), SentTx{txToRetry.Tx, txToRetry.NumOfRetries + 1, false, isL2})
	return nil
}

func (b *Bridge) GetPendingBridgeTxs(channelId types.Destination) []PendingTx {
	var foundPendingTx []PendingTx
	b.sentTxs.Range(func(txHash string, sentTxInfo SentTx) bool {
		if sentTxInfo.Tx.ChannelId() == channelId {
			foundPendingTx = append(foundPendingTx, PendingTx{sentTxInfo, txHash})
		}
		return true
	})

	return foundPendingTx
}

func (b *Bridge) GetNodeInfo() types.NodeInfo {
	// State channel address and message service peer ID for both the L1 and L2 nodes are same because the same private key is being used for both nodes
	return b.nodeL1.GetNodeInfo()
}
