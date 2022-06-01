package chainservice

import (
	"context"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	NitroAdjudicator "github.com/statechannels/go-nitro/client/engine/chainservice/adjudicator"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

var depositedTopic = crypto.Keccak256Hash([]byte("Deposited(bytes32,address,uint256,uint256)"))

type eventSource interface {
	SubscribeFilterLogs(ctx context.Context, query ethereum.FilterQuery, ch chan<- ethTypes.Log) (ethereum.Subscription, error)
}

type EthChainService struct {
	out chan Event
	na  *NitroAdjudicator.NitroAdjudicator
	to  *bind.TransactOpts
}

// NewEthChainService constructs a chain service that submits transactions to a NitroAdjudicator
// and listens to events from an eventSource
func NewEthChainService(na *NitroAdjudicator.NitroAdjudicator, naAddress common.Address, to *bind.TransactOpts, es eventSource) EthChainService {
	ecs := EthChainService{}
	ecs.out = make(chan Event)
	ecs.na = na
	ecs.to = to

	go ecs.listenForLogEvents(na, naAddress, es)

	return ecs
}

// SendTransaction sends the transaction and blocks until it has been submitted.
func (ecs *EthChainService) SendTransaction(tx protocols.ChainTransaction) {
	switch tx.Type {
	case protocols.DepositTransactionType:
		for address, amount := range tx.Deposit {
			// TODO clone to before modifying
			ecs.to.Value = amount
			// TODO do not assume that the channel holds 0 funds
			_, err := ecs.na.Deposit(ecs.to, address, tx.ChannelId, big.NewInt(0), amount)

			if err != nil {
				panic(err)
			}
		}
	// TODO handle other transaction types
	default:
		panic("unexpected chain transaction")
	}
}

func (cc EthChainService) SubscribeToEvents(a types.Address) <-chan Event {
	return cc.out
}

func (ecs EthChainService) listenForLogEvents(na *NitroAdjudicator.NitroAdjudicator, naAddress common.Address, es eventSource) {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{naAddress},
	}
	logs := make(chan ethTypes.Log)
	sub, err := es.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatal(err)
	}
	for {
		select {
		case err := <-sub.Err():
			log.Fatal(err)
		case chainEvent := <-logs:
			switch chainEvent.Topics[0] {
			case depositedTopic:
				nad, err := na.ParseDeposited(chainEvent)
				if err != nil {
					log.Fatal(err)
				}

				holdings := types.Funds{}
				holdings[nad.Asset] = nad.DestinationHoldings
				// TODO fill out other event fields once the event data structure is settled.
				event := DepositedEvent{
					CommonEvent: CommonEvent{
						channelID: nad.Destination,
					},
					Holdings: holdings,
				}
				ecs.out <- event
			// TODO introduce the remaining events
			default:
				panic("Unknown chain event")
			}
		}
	}
}