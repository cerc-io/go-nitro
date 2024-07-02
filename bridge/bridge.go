package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	nodeutils "github.com/statechannels/go-nitro/internal/node"
	"github.com/statechannels/go-nitro/node"
	p2pms "github.com/statechannels/go-nitro/node/engine/messageservice/p2p-message-service"
	"github.com/statechannels/go-nitro/protocols/bridgedfund"
	"github.com/statechannels/go-nitro/protocols/directfund"

	"github.com/statechannels/go-nitro/node/engine/chainservice"
	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

const (
	L1_DURABLE_STORE_SUB_DIR = "l1-nitro-store"
	L2_DURABLE_STORE_SUB_DIR = "l2-nitro-store"
)

type MirrorChannelDetails struct {
	l1ChannelId types.Destination
	isCreated   bool
}

type Bridge struct {
	nodeL1  *node.Node
	storeL1 store.Store

	nodeL2  *node.Node
	storeL2 store.Store

	config           BridgeConfig
	cancel           context.CancelFunc
	mirrorChannelMap map[types.Destination]MirrorChannelDetails
}

type BridgeConfig struct {
	L1ChainUrl        string
	L2ChainUrl        string
	L1ChainStartBlock uint64
	L2ChainStartBlock uint64
	ChainPK           string
	StateChannelPK    string
	NaAddress         string
	VpaAddress        string
	CaAddress         string
	BridgeAddress     string
	DurableStoreDir   string
	BridgePublicIp    string
	NodeL1MsgPort     int
	NodeL2MsgPort     int
}

func New(configOpts BridgeConfig) *Bridge {
	bridge := Bridge{
		config:           configOpts,
		mirrorChannelMap: make(map[types.Destination]MirrorChannelDetails),
	}

	return &bridge
}

func (b *Bridge) Start() error {
	chainOptsL1 := chainservice.ChainOpts{
		ChainUrl:           b.config.L1ChainUrl,
		ChainStartBlockNum: b.config.L1ChainStartBlock,
		ChainPk:            b.config.ChainPK,
		NaAddress:          common.HexToAddress(b.config.NaAddress),
		VpaAddress:         common.HexToAddress(b.config.VpaAddress),
		CaAddress:          common.HexToAddress(b.config.CaAddress),
	}

	chainOptsL2 := chainservice.L2ChainOpts{
		ChainUrl:           b.config.L2ChainUrl,
		ChainStartBlockNum: b.config.L2ChainStartBlock,
		ChainPk:            b.config.ChainPK,
		BridgeAddress:      common.HexToAddress(b.config.BridgeAddress),
		VpaAddress:         common.HexToAddress(b.config.VpaAddress),
		CaAddress:          common.HexToAddress(b.config.CaAddress),
	}

	storeOptsL1 := store.StoreOpts{
		PkBytes:            common.Hex2Bytes(b.config.StateChannelPK),
		UseDurableStore:    true,
		DurableStoreFolder: filepath.Join(b.config.DurableStoreDir, L1_DURABLE_STORE_SUB_DIR),
	}

	storeOptsL2 := store.StoreOpts{
		PkBytes:            common.Hex2Bytes(b.config.StateChannelPK),
		UseDurableStore:    true,
		DurableStoreFolder: filepath.Join(b.config.DurableStoreDir, L2_DURABLE_STORE_SUB_DIR),
	}

	messageOptsL1 := p2pms.MessageOpts{
		PkBytes:   common.Hex2Bytes(b.config.StateChannelPK),
		Port:      b.config.NodeL1MsgPort,
		BootPeers: nil,
		PublicIp:  b.config.BridgePublicIp,
	}

	messageOptsL2 := p2pms.MessageOpts{
		PkBytes:   common.Hex2Bytes(b.config.StateChannelPK),
		Port:      b.config.NodeL2MsgPort,
		BootPeers: nil,
		PublicIp:  b.config.BridgePublicIp,
	}

	// Initialize nodes
	nodeL1, storeL1, _, _, err := nodeutils.InitializeNode(chainOptsL1, storeOptsL1, messageOptsL1)
	if err != nil {
		return err
	}

	nodeL2, storeL2, _, _, err := nodeutils.InitializeL2Node(chainOptsL2, storeOptsL2, messageOptsL2)
	if err != nil {
		return err
	}

	b.nodeL1 = nodeL1
	b.storeL1 = *storeL1
	b.nodeL2 = nodeL2
	b.storeL2 = *storeL2

	ctx, cancelFunc := context.WithCancel(context.Background())
	b.cancel = cancelFunc

	go b.run(ctx)

	return nil
}

func (b *Bridge) run(ctx context.Context) {
	completedObjectivesInNodeL1 := b.nodeL1.CompletedObjectives()
	completedObjectivesInNodeL2 := b.nodeL2.CompletedObjectives()

	for {
		var err error
		select {
		case objId := <-completedObjectivesInNodeL1:
			err = b.processCompletedObjectivesFromL1(objId)
			b.checkError(err)
		case objId := <-completedObjectivesInNodeL2:
			err = b.processCompletedObjectivesFromL2(objId)
			b.checkError(err)
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
	if isDdfo {
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
		tempAllocation := l1ledgerChannelStateClone.State().Outcome[0].Allocations[0]
		l1ledgerChannelStateClone.State().Outcome[0].Allocations[0] = l1ledgerChannelStateClone.State().Outcome[0].Allocations[1]
		l1ledgerChannelStateClone.State().Outcome[0].Allocations[1] = tempAllocation

		// Create extended state outcome based on l1ChannelState
		l2ChannelOutcome := l1ledgerChannelStateClone.State().Outcome

		// Create mirrored ledger channel between node BPrime and APrime
		l2LedgerChannelResponse, err := b.nodeL2.CreateBridgeChannel(l1ledgerChannelStateClone.State().Participants[0], uint32(10), l2ChannelOutcome)
		if err != nil {
			return err
		}

		b.mirrorChannelMap[l2LedgerChannelResponse.ChannelId] = MirrorChannelDetails{l1ChannelId: l1LedgerChannel.Id}
		slog.Debug("Started creating mirror ledger channel in L2", "channelId", l2LedgerChannelResponse.ChannelId)
	}

	return nil
}

func (b *Bridge) processCompletedObjectivesFromL2(objId protocols.ObjectiveId) error {
	obj, err := b.storeL2.GetObjectiveById(objId)
	if err != nil {
		return fmt.Errorf("error in getting objective %w", err)
	}

	bFo, isBfo := obj.(*bridgedfund.Objective)
	if isBfo {
		l2channelId := bFo.OwnsChannel()
		l2Info := b.mirrorChannelMap[l2channelId]
		l2Info.isCreated = true
		b.mirrorChannelMap[l2channelId] = l2Info
	}

	return nil
}

func (b Bridge) GetMirrorChannel(l1ChannelId types.Destination) (l2ChannelId types.Destination, ok bool) {
	for key, value := range b.mirrorChannelMap {
		if value.l1ChannelId == l1ChannelId {
			return key, true
		}
	}
	return types.Destination{}, false
}

func (b *Bridge) Close() {
	b.cancel()
	b.nodeL1.Close()
	b.nodeL2.Close()
}

func (b *Bridge) checkError(err error) {
	if err != nil {
		slog.Error("error in run loop", err)
	}
}
