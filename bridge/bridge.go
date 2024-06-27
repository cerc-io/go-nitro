package bridge

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	nodehelper "github.com/statechannels/go-nitro/internal/node"
	"github.com/statechannels/go-nitro/node"

	"github.com/statechannels/go-nitro/node/engine/chainservice"
	p2pms "github.com/statechannels/go-nitro/node/engine/messageservice/p2p-message-service"
	"github.com/statechannels/go-nitro/node/engine/store"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

type Bridge struct {
	config                      BridgeConfig
	nodeL1                      *node.Node
	storeL1                     store.Store
	completedObjectivesInNodeL1 <-chan protocols.ObjectiveId

	nodeL2  *node.Node
	storeL2 store.Store

	cancel           context.CancelFunc
	mirrorChannelMap map[types.Destination]types.Destination
}

type BridgeConfig struct {
	ChainOptsL1   chainservice.ChainOpts
	StoreOptsL1   store.StoreOpts
	MessageOptsL1 p2pms.MessageOpts

	ChainOptsL2   chainservice.L2ChainOpts
	StoreOptsL2   store.StoreOpts
	MessageOptsL2 p2pms.MessageOpts
}

func New(configOpts BridgeConfig) *Bridge {
	bridge := Bridge{
		config:           configOpts,
		mirrorChannelMap: make(map[types.Destination]types.Destination),
	}

	return &bridge
}

func (b *Bridge) Start() error {
	// Initialize nodes
	nodeL1, storeL1, _, _, err := nodehelper.InitializeNode(b.config.ChainOptsL1, b.config.StoreOptsL1, b.config.MessageOptsL1)
	if err != nil {
		return err
	}

	nodeL2, storeL2, _, _, err := nodehelper.InitializeL2Node(b.config.ChainOptsL2, b.config.StoreOptsL2, b.config.MessageOptsL2)
	if err != nil {
		return err
	}

	b.nodeL1 = nodeL1
	b.storeL1 = *storeL1
	b.nodeL2 = nodeL2
	b.storeL2 = *storeL2
	b.completedObjectivesInNodeL1 = nodeL1.CompletedObjectives()

	ctx, cancelFunc := context.WithCancel(context.Background())
	b.cancel = cancelFunc

	go b.run(ctx)

	return nil
}

func (b *Bridge) run(ctx context.Context) {
	for {
		var err error
		select {
		case objId := <-b.completedObjectivesInNodeL1:
			err = b.processObjectivesFromL1(objId)
			b.checkError(err)
		case <-ctx.Done():
			return
		}
	}
}

func (b *Bridge) processObjectivesFromL1(objId protocols.ObjectiveId) error {
	objIdArr := strings.Split(string(objId), "-")
	objectiveType := objIdArr[0]
	channelId := objIdArr[1]

	// If objectiveId corresponds to direct fund objective
	// Create new outcome for mirrored ledger channel based on L1 ledger channel
	// Create mirrored ledger channel on L2 based on created outcome
	if objectiveType == "DirectFunding" {
		fmt.Println("Creating mirror outcome for L2", channelId)
		l1LedgerChannel, err := b.storeL1.GetConsensusChannelById(types.Destination(common.HexToHash(channelId)))
		if err != nil {
			return err
		}

		l1ledgerChannelState := l1LedgerChannel.SupportedSignedState()
		l1ledgerChannelStateClone := l1ledgerChannelState.Clone()

		// Swap the destination
		tempDestination := l1ledgerChannelStateClone.State().Outcome[0].Allocations[0].Destination
		l1ledgerChannelStateClone.State().Outcome[0].Allocations[0].Destination = l1ledgerChannelStateClone.State().Outcome[0].Allocations[1].Destination
		l1ledgerChannelStateClone.State().Outcome[0].Allocations[1].Destination = tempDestination

		// Create extended state outcome based on l1ChannelState
		l2ChannelOutcome := l1ledgerChannelStateClone.State().Outcome

		// Create mirrored ledger channel between node BPrime and APrime
		l2LedgerChannelResponse, err := b.nodeL2.CreateBridgeChannel(l1ledgerChannelState.State().Participants[0], uint32(10), l2ChannelOutcome)
		if err != nil {
			return err
		}

		b.mirrorChannelMap[l1LedgerChannel.Id] = l2LedgerChannelResponse.ChannelId
		fmt.Println("Started creating mirror ledger channel in L2", l2LedgerChannelResponse.ChannelId)
	}

	return nil
}

func (b Bridge) GetMirrorChannel(l1ChannelId types.Destination) (l2ChannelId types.Destination, ok bool) {
	l2ChannelId, ok = b.mirrorChannelMap[l1ChannelId]
	return
}

func (b *Bridge) Close() {
	b.cancel()
	// TODO: Close bridge nodes
	// TODO: Fix issue preventing node terminal from closing
}

func (b *Bridge) checkError(err error) {
	if err != nil {
		fmt.Println("error in run loop", err)
		panic(err)
	}
}
