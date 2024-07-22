package rpc

import (
	"crypto/tls"
	"fmt"
	"log/slog"

	"github.com/statechannels/go-nitro/bridge"
	"github.com/statechannels/go-nitro/node"
	"github.com/statechannels/go-nitro/rpc"
	"github.com/statechannels/go-nitro/rpc/transport"
	httpTransport "github.com/statechannels/go-nitro/rpc/transport/http"
	"github.com/statechannels/go-nitro/rpc/transport/nats"
)

func InitializeNodeRpcServer(node *node.Node, rpcPort int, useNats bool, cert *tls.Certificate) (*rpc.NodeRpcServer, error) {
	transport, err := initializeTransport(rpcPort, useNats, cert)
	if err != nil {
		return nil, err
	}

	rpcServer, err := rpc.NewNodeRpcServer(node, transport)
	if err != nil {
		return nil, err
	}

	slog.Info("Completed RPC server initialization", "url", rpcServer.Url())
	return rpcServer, nil
}

func InitializeBridgeRpcServer(bridge *bridge.Bridge, rpcPort int, useNats bool, cert *tls.Certificate) (*rpc.BridgeRpcServer, error) {
	transport, err := initializeTransport(rpcPort, useNats, cert)
	if err != nil {
		return nil, err
	}

	rpcServer, err := rpc.NewBridgeRpcServer(bridge, transport)
	if err != nil {
		return nil, err
	}

	slog.Info("Completed RPC server initialization", "url", rpcServer.Url())
	return rpcServer, nil
}

func initializeTransport(rpcPort int, useNats bool, cert *tls.Certificate) (transport.Responder, error) {
	var transport transport.Responder
	var err error

	if useNats {
		slog.Info("Initializing NATS RPC transport...")
		transport, err = nats.NewNatsTransportAsServer(rpcPort)
	} else {
		slog.Info("Initializing Http RPC transport...")
		transport, err = httpTransport.NewHttpTransportAsServer(fmt.Sprint(rpcPort), cert)
	}
	if err != nil {
		return nil, err
	}

	return transport, nil
}
