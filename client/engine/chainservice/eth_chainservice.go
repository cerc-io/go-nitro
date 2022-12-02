package chainservice

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	NitroAdjudicator "github.com/statechannels/go-nitro/client/engine/chainservice/adjudicator"
	Token "github.com/statechannels/go-nitro/client/engine/chainservice/erc20"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"

	"github.com/ethereum/go-ethereum/rpc"
)

const FEVM_PARSE_ERROR = "json: cannot unmarshal hex string \"0x\" into Go struct field txJSON.v of type *hexutil.Big"

var allocationUpdatedTopic = crypto.Keccak256Hash([]byte("AllocationUpdated(bytes32,uint256,uint256)"))
var concludedTopic = crypto.Keccak256Hash([]byte("Concluded(bytes32,uint48)"))
var depositedTopic = crypto.Keccak256Hash([]byte("Deposited(bytes32,address,uint256,uint256)"))

type ethChain interface {
	bind.ContractBackend
	ethereum.TransactionReader
}

type EthChainService struct {
	chain                    ethChain
	na                       *NitroAdjudicator.NitroAdjudicator
	naAddress                common.Address
	consensusAppAddress      common.Address
	virtualPaymentAppAddress common.Address
	txSigner                 *bind.TransactOpts
	out                      chan Event
	logger                   *log.Logger
}

// Since we only fetch events if there's a new block number
// it's safe to poll relatively frequently.
const EVENT_POLL_INTERVAL = 500 * time.Millisecond

// RESUB_INTERVAL is how often we resubscribe to log events.
// We do this to avoid https://github.com/ethereum/go-ethereum/issues/23845
// We use 2.5 minutes as the default filter timeout is 5 minutes.
// See https://github.com/ethereum/go-ethereum/blob/e14164d516600e9ac66f9060892e078f5c076229/eth/filters/filter_system.go#L43
const RESUB_INTERVAL = 2*time.Minute + 30*time.Second

// NewEthChainService constructs a chain service that submits transactions to a NitroAdjudicator
// and listens to events from an eventSource
func NewEthChainService(chain ethChain, na *NitroAdjudicator.NitroAdjudicator,
	naAddress, caAddress, vpaAddress common.Address, txSigner *bind.TransactOpts, logDestination io.Writer) (*EthChainService, error) {
	logPrefix := "chainservice " + txSigner.From.String() + ": "
	logger := log.New(logDestination, logPrefix, log.Lmicroseconds|log.Lshortfile)
	// Use a buffered channel so we don't have to worry about blocking on writing to the channel.
	ecs := EthChainService{chain, na, naAddress, caAddress, vpaAddress, txSigner, make(chan Event, 10), logger}

	if ecs.subscriptionsSupported() {
		logger.Printf("Notifications are supported by the chain. Using notifications to listen for events.")
		go ecs.subscribeForLogs(context.Background())
	} else {
		logger.Printf("Notifications are NOT supported by the chain. Using polling to listen for events.")
		go ecs.pollForLogs(context.Background())
	}
	return &ecs, nil
}

// subscriptionsSupported returns true if the node supports subscriptions for events.
// Otherwise returns false
func (ecs *EthChainService) subscriptionsSupported() bool {
	// This is slightly painful but seems like the only way to find out if notifications are supported
	// We attempt to subscribe (with a query that should never return a result) and check the error
	query := ethereum.FilterQuery{
		Addresses: []common.Address{ecs.naAddress},
		FromBlock: big.NewInt(0),
		ToBlock:   big.NewInt(0),
	}

	logs := make(chan ethTypes.Log, 1)
	sub, err := ecs.chain.SubscribeFilterLogs(context.Background(), query, logs)
	if err == rpc.ErrNotificationsUnsupported {
		return false
	}

	sub.Unsubscribe()
	return true
}

// defaultTxOpts returns transaction options suitable for most transaction submissions
func (ecs *EthChainService) defaultTxOpts() *bind.TransactOpts {
	return &bind.TransactOpts{
		From:      ecs.txSigner.From,
		Nonce:     ecs.txSigner.Nonce,
		Signer:    ecs.txSigner.Signer,
		GasFeeCap: ecs.txSigner.GasFeeCap,
		GasTipCap: ecs.txSigner.GasTipCap,
		GasLimit:  ecs.txSigner.GasLimit,
		GasPrice:  ecs.txSigner.GasPrice,
	}
}

// SendTransaction sends the transaction and blocks until it has been submitted.
func (ecs *EthChainService) SendTransaction(tx protocols.ChainTransaction) error {
	switch tx := tx.(type) {
	case protocols.DepositTransaction:
		for tokenAddress, amount := range tx.Deposit {
			txOpts := ecs.defaultTxOpts()
			ethTokenAddress := common.Address{}
			if tokenAddress == ethTokenAddress {
				txOpts.Value = amount
			} else {
				tokenTransactor, err := Token.NewTokenTransactor(tokenAddress, ecs.chain)
				if err != nil {
					return err
				}
				_, err = tokenTransactor.Approve(ecs.defaultTxOpts(), ecs.naAddress, amount)
				if err != nil {
					return err
				}
			}
			holdings, err := ecs.na.Holdings(&bind.CallOpts{}, tokenAddress, tx.ChannelId())
			if err != nil {
				return err
			}

			_, err = ecs.na.Deposit(txOpts, tokenAddress, tx.ChannelId(), holdings, amount)
			if err != nil {
				return err
			}
		}
		return nil
	case protocols.WithdrawAllTransaction:
		state := tx.SignedState.State()
		signatures := tx.SignedState.Signatures()
		nitroFixedPart := NitroAdjudicator.INitroTypesFixedPart(NitroAdjudicator.ConvertFixedPart(state.FixedPart()))
		nitroVariablePart := NitroAdjudicator.ConvertVariablePart(state.VariablePart())
		nitroSignatures := []NitroAdjudicator.INitroTypesSignature{NitroAdjudicator.ConvertSignature(signatures[0]), NitroAdjudicator.ConvertSignature(signatures[1])}
		proof := make([]NitroAdjudicator.INitroTypesSignedVariablePart, 0)
		candidate := NitroAdjudicator.INitroTypesSignedVariablePart{
			VariablePart: nitroVariablePart,
			Sigs:         nitroSignatures,
		}
		_, err := ecs.na.ConcludeAndTransferAllAssets(ecs.defaultTxOpts(), nitroFixedPart, proof, candidate)
		return err

	default:
		return fmt.Errorf("unexpected transaction type %T", tx)
	}
}

// fatalError is called when a goroutine encounters an unrecoverable error.
// If prints out the error to STDOUT, the logger and then exits the program.
func (ecs *EthChainService) fatalError(format string, v ...any) {

	// Print to STDOUT in case we're using a noop logger
	fmt.Println(fmt.Errorf(format, v...))
	// FatalF prints to the logger then calls exit(1)
	ecs.logger.Fatalf(format, v...)

	// Manually panic in case we're using a logger that doesn't call exit(1)
	panic(fmt.Errorf(format, v...))

}

// dispatchChainEvents takes in a collection of event logs from the chain
// and dispatches events to the out channel
func (ecs *EthChainService) dispatchChainEvents(logs []ethTypes.Log) {
	for _, l := range logs {
		switch l.Topics[0] {
		case depositedTopic:
			nad, err := ecs.na.ParseDeposited(l)
			if err != nil {
				ecs.fatalError("error in ParseDeposited: %v", err)
			}

			event := NewDepositedEvent(nad.Destination, l.BlockNumber, nad.Asset, nad.AmountDeposited, nad.DestinationHoldings)
			ecs.out <- event
		case allocationUpdatedTopic:
			au, err := ecs.na.ParseAllocationUpdated(l)
			if err != nil {
				ecs.fatalError("error in ParseAllocationUpdated: %v", err)

			}

			tx, pending, err := ecs.chain.TransactionByHash(context.Background(), l.TxHash)
			if pending {
				ecs.fatalError("Expected transaction to be part of the chain, but the transaction is pending")

			}
			var assetAddress types.Address
			var amount *big.Int
			switch {

			// TODO: Workaround for https://github.com/filecoin-project/ref-fvm/issues/1158
			case err != nil && strings.Contains(err.Error(), FEVM_PARSE_ERROR):
				ecs.logger.Printf("WARNING: Cannot parse transaction, assuming asset address of 0x00...")
				fmt.Printf("WARNING: Cannot parse transaction, assuming asset address of 0x00...")
				assetAddress := types.Address{}
				amount, err = getAssetHoldings(ecs.na, assetAddress, new(big.Int).SetUint64(au.Raw.BlockNumber), au.ChannelId)
				if err != nil {
					ecs.fatalError("error in getAssetHoldings: %v", err)
				}

			case err != nil:
				ecs.fatalError("error in TransactionByHash: %v", err)

			default:
				assetAddress, amount, err = getChainHolding(ecs.na, tx, au)
				if err != nil {
					ecs.fatalError("error in getChainHoldings: %v", err)
				}
			}

			event := NewAllocationUpdatedEvent(au.ChannelId, l.BlockNumber, assetAddress, amount)
			ecs.out <- event
		case concludedTopic:
			ce, err := ecs.na.ParseConcluded(l)
			if err != nil {
				ecs.fatalError("error in ParseConcluded: %v", err)

			}

			event := ConcludedEvent{commonEvent: commonEvent{channelID: ce.ChannelId, BlockNum: l.BlockNumber}}
			ecs.out <- event

		default:
			ecs.fatalError("Unknown chain event")
		}
	}

}

// getCurrentBlockNumber returns the current block number.
func (ecs *EthChainService) getCurrentBlockNum() *big.Int {
	h, err := ecs.chain.HeaderByNumber(context.Background(), nil)
	if err != nil {
		panic(err)
	}

	return h.Number
}

// subscribeForLogs subscribes for logs and pushes them to the out channel.
// It relies on notifications being supported by the chain node.
func (ecs *EthChainService) subscribeForLogs(ctx context.Context) {
	// Subscribe to Adjudicator events
	query := ethereum.FilterQuery{
		Addresses: []common.Address{ecs.naAddress},
	}
	logs := make(chan ethTypes.Log)
	sub, err := ecs.chain.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		panic(err)
	}
	for {
		select {
		case err := <-sub.Err():
			if err != nil {
				panic(err)
			}

			// If the error is nil then the subscription was closed and we need to re-subscribe.
			// This is a workaround for https://github.com/ethereum/go-ethereum/issues/23845
			var sErr error
			sub, sErr = ecs.chain.SubscribeFilterLogs(ctx, query, logs)
			if sErr != nil {
				panic(err)
			}
			ecs.logger.Println("resubscribed to filtered logs")

		case <-time.After(RESUB_INTERVAL):
			// Due to https://github.com/ethereum/go-ethereum/issues/23845 we can't rely on a long running subscription.
			// We unsub here and recreate the subscription in the next iteration of the select.
			sub.Unsubscribe()
		case chainEvent := <-logs:
			ecs.dispatchChainEvents([]ethTypes.Log{chainEvent})

		}
	}

}

// pollForLogs periodically polls the chain client to check if there new events.
// It can function over a chain node that does not support notifications.
func (ecs *EthChainService) pollForLogs(ctx context.Context) {

	// The initial query we want to get all events since the contract was deployed to the current block.
	query := ethereum.FilterQuery{
		Addresses: []common.Address{ecs.naAddress},
		FromBlock: nil,
		ToBlock:   ecs.getCurrentBlockNum(),
	}

	fetchedLogs, err := ecs.chain.FilterLogs(context.Background(), query)

	if err != nil {
		panic(err)
	}

	ecs.dispatchChainEvents(fetchedLogs)
	for {
		select {
		case <-time.After(EVENT_POLL_INTERVAL):
			currentBlock := ecs.getCurrentBlockNum()

			if moreRecentBlockAvailable := currentBlock.Cmp(query.ToBlock) > 0; moreRecentBlockAvailable {
				// The query includes the from and to blocks so we need to increment the from block to avoid duplicating events
				query.FromBlock = big.NewInt(0).Add(query.ToBlock, big.NewInt(1))
				query.ToBlock = big.NewInt(0).Set(currentBlock)

				fetchedLogs, err := ecs.chain.FilterLogs(context.Background(), query)

				if err != nil {
					panic(err)
				}

				ecs.dispatchChainEvents(fetchedLogs)
			}
		case <-ctx.Done():
			return
		}
	}

}

// EventFeed returns the out chan, and narrows the type so that external consumers may only receive on it.
func (ecs *EthChainService) EventFeed() <-chan Event {
	return ecs.out
}

func (ecs *EthChainService) GetConsensusAppAddress() types.Address {
	return ecs.consensusAppAddress
}

func (ecs *EthChainService) GetVirtualPaymentAppAddress() types.Address {
	return ecs.virtualPaymentAppAddress
}
