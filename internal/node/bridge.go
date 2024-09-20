package node

import (
	"fmt"
	"log/slog"

	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/node/engine"
	"github.com/statechannels/go-nitro/node/engine/chainservice"
	p2pms "github.com/statechannels/go-nitro/node/engine/messageservice/p2p-message-service"
	"github.com/statechannels/go-nitro/node/engine/store"
)

func InitializeL2Node(chainOpts chainservice.LaconicdChainOpts, storeOpts store.StoreOpts, messageOpts p2pms.MessageOpts) (*node.Node, *store.Store, *p2pms.P2PMessageService, chainservice.ChainService, error) {
	ourStore, err := store.NewStore(storeOpts)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	slog.Info("Initializing message service on port " + fmt.Sprint(messageOpts.TcpPort) + "...")
	messageOpts.SCAddr = *ourStore.GetAddress()
	messageService := p2pms.NewMessageService(messageOpts)

	// TODO: Implement laconicd chain service
	ourChain, err := chainservice.NewLaconicdChainService(chainOpts)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	node := node.New(
		messageService,
		ourChain,
		ourStore,
		&engine.PermissivePolicy{},
	)

	return &node, &ourStore, messageService, ourChain, nil
}
