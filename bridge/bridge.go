package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/channel/state/outcome"
	nodeutils "github.com/statechannels/go-nitro/internal/node"
	"github.com/statechannels/go-nitro/node"
	p2pms "github.com/statechannels/go-nitro/node/engine/messageservice/p2p-message-service"
	"github.com/statechannels/go-nitro/node/query"
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

type Asset struct {
	L1AssetAddress string `toml:"l1AssetAddress"`
	L2AssetAddress string `toml:"l2AssetAddress"`
}

type L1ToL2AssetConfig struct {
	Assets []Asset `toml:"assets"`
}

type Bridge struct {
	bridgeStore *DurableStore

	nodeL1         *node.Node
	storeL1        store.Store
	chainServiceL1 chainservice.ChainService

	nodeL2  *node.Node
	storeL2 store.Store

	cancel                  context.CancelFunc
	L1ToL2AssetAddressMap   map[common.Address]common.Address
	mirrorChannelMap        map[types.Destination]MirrorChannelDetails
	completedMirrorChannels chan types.Destination
}

type BridgeConfig struct {
	L1ChainUrl         string
	L2ChainUrl         string
	L1ChainStartBlock  uint64
	L2ChainStartBlock  uint64
	ChainPK            string
	StateChannelPK     string
	NaAddress          string
	VpaAddress         string
	CaAddress          string
	BridgeAddress      string
	Assets             []Asset
	DurableStoreDir    string
	BridgePublicIp     string
	NodeL1ExtMultiAddr string
	NodeL2ExtMultiAddr string
	NodeL1MsgPort      int
	NodeL2MsgPort      int
}

func New() *Bridge {
	bridge := Bridge{
		mirrorChannelMap:        make(map[types.Destination]MirrorChannelDetails),
		L1ToL2AssetAddressMap:   make(map[common.Address]common.Address),
		completedMirrorChannels: make(chan types.Destination),
	}

	return &bridge
}

func (b *Bridge) Start(configOpts BridgeConfig) (nodeL1MultiAddress string, nodeL2MultiAddress string, err error) {
	chainOptsL1 := chainservice.ChainOpts{
		ChainUrl:           configOpts.L1ChainUrl,
		ChainStartBlockNum: configOpts.L1ChainStartBlock,
		ChainPk:            configOpts.ChainPK,
		NaAddress:          common.HexToAddress(configOpts.NaAddress),
		VpaAddress:         common.HexToAddress(configOpts.VpaAddress),
		CaAddress:          common.HexToAddress(configOpts.CaAddress),
	}

	chainOptsL2 := chainservice.L2ChainOpts{
		ChainUrl:           configOpts.L2ChainUrl,
		ChainStartBlockNum: configOpts.L2ChainStartBlock,
		ChainPk:            configOpts.ChainPK,
		BridgeAddress:      common.HexToAddress(configOpts.BridgeAddress),
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
		Port:         configOpts.NodeL1MsgPort,
		BootPeers:    nil,
		PublicIp:     configOpts.BridgePublicIp,
		ExtMultiAddr: configOpts.NodeL1ExtMultiAddr,
	}

	messageOptsL2 := p2pms.MessageOpts{
		PkBytes:      common.Hex2Bytes(configOpts.StateChannelPK),
		Port:         configOpts.NodeL2MsgPort,
		BootPeers:    nil,
		PublicIp:     configOpts.BridgePublicIp,
		ExtMultiAddr: configOpts.NodeL2ExtMultiAddr,
	}

	// Initialize nodes
	nodeL1, storeL1, msgServiceL1, chainServiceL1, err := nodeutils.InitializeNode(chainOptsL1, storeOptsL1, messageOptsL1)
	if err != nil {
		return nodeL1MultiAddress, nodeL2MultiAddress, err
	}

	nodeL2, storeL2, msgServiceL2, _, err := nodeutils.InitializeL2Node(chainOptsL2, storeOptsL2, messageOptsL2)
	if err != nil {
		return nodeL1MultiAddress, nodeL2MultiAddress, err
	}

	// Process Assets array to convert it to map of L1 asset address to L2 asset address
	for _, asset := range configOpts.Assets {
		b.L1ToL2AssetAddressMap[common.HexToAddress(asset.L1AssetAddress)] = common.HexToAddress(asset.L2AssetAddress)
	}

	b.nodeL1 = nodeL1
	b.storeL1 = *storeL1
	b.chainServiceL1 = chainServiceL1
	b.nodeL2 = nodeL2
	b.storeL2 = *storeL2

	ctx, cancelFunc := context.WithCancel(context.Background())
	b.cancel = cancelFunc

	ds, err := NewDurableStore(configOpts.DurableStoreDir, buntdb.Config{})
	if err != nil {
		return nodeL1MultiAddress, nodeL2MultiAddress, err
	}

	b.bridgeStore = ds

	go b.run(ctx)

	return msgServiceL1.MultiAddr, msgServiceL2.MultiAddr, nil
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
		l1ChannelCloneOutcome := l1ledgerChannelStateClone.State().Outcome
		var l2ChannelOutcome outcome.Exit

		for _, l1Outcome := range l1ChannelCloneOutcome {
			if (l1Outcome.Asset == common.Address{}) {
				l2ChannelOutcome = append(l2ChannelOutcome, l1Outcome)
			} else {
				value, ok := b.L1ToL2AssetAddressMap[l1Outcome.Asset]

				if !ok {
					return fmt.Errorf("could not find corresponding L2 asset address for L1 asset address %s", l1Outcome.Asset.String())
				}

				l1Outcome.Asset = value
				l2ChannelOutcome = append(l2ChannelOutcome, l1Outcome)
			}
		}

		// Create mirrored ledger channel between node BPrime and APrime
		l2LedgerChannelResponse, err := b.nodeL2.CreateBridgeChannel(l1ledgerChannelStateClone.State().Participants[0], uint32(10), l2ChannelOutcome)
		if err != nil {
			return err
		}

		err = b.bridgeStore.SetMirrorChannelDetails(l2LedgerChannelResponse.ChannelId, MirrorChannelDetails{L1ChannelId: l1LedgerChannel.Id})
		if err != nil {
			return err
		}

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
		mirrorChannelDetails, err := b.bridgeStore.GetMirrorChannelDetails(l2channelId)
		if err != nil {
			return err
		}

		err = b.bridgeStore.SetMirrorChannelDetails(l2channelId, MirrorChannelDetails{L1ChannelId: mirrorChannelDetails.L1ChannelId, IsCreated: true})
		if err != nil {
			return err
		}

		// Node B calls contract method to store L2ChannelId => L1ChannelId
		setL2ToL1Tx := protocols.NewSetL2ToL1Transaction(mirrorChannelDetails.L1ChannelId, l2channelId)
		err = b.chainServiceL1.SendTransaction(setL2ToL1Tx)
		if err != nil {
			return fmt.Errorf("error in send transaction %w", err)
		}

		// use a nonblocking send in case no one is listening
		select {
		case b.completedMirrorChannels <- l2channelId:
		default:
		}
	}

	return nil
}

// Since bridge node addresses are same
func (b Bridge) GetBridgeAddress() common.Address {
	return *b.nodeL1.Address
}

func (b Bridge) GetL2ChannelIdByL1ChannelId(l1ChannelId types.Destination) (l2ChannelId types.Destination, isCreated bool) {
	var err error
	l2ChannelId, isCreated, err = b.bridgeStore.GetMirrorChannelDetailsByL1Channel(l1ChannelId)
	if err != nil {
		return l2ChannelId, isCreated
	}

	return l2ChannelId, isCreated
}

func (b Bridge) GetAllL2Channels() ([]query.LedgerChannelInfo, error) {
	return b.nodeL2.GetAllLedgerChannels()
}

func (b *Bridge) CompletedMirrorChannels() <-chan types.Destination {
	return b.completedMirrorChannels
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
