package chainservice

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/statechannels/go-nitro/internal/logging"
	Bridge "github.com/statechannels/go-nitro/node/engine/chainservice/bridge"
	chainutils "github.com/statechannels/go-nitro/node/engine/chainservice/utils"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

type L2ChainOpts struct {
	ChainUrl           string
	ChainStartBlockNum uint64
	ChainAuthToken     string
	ChainPk            string
	bridgeAddress  		 common.Address
}

type L2ChainService struct {
	chain                    ethChain
	bridge                   *Bridge.Bridge
	bridgeAddress            common.Address
	txSigner                 *bind.TransactOpts
	out                      chan Event
	logger                   *slog.Logger
	ctx                      context.Context
	cancel                   context.CancelFunc
	wg                       *sync.WaitGroup
	eventTracker             *eventTracker
	eventSub                 ethereum.Subscription
	newBlockSub              ethereum.Subscription
}

// NewEthChainService is a convenient wrapper around newEthChainService, which provides a simpler API
func NewL2ChainService(l2ChainOpts L2ChainOpts) (*L2ChainService, error) {
	if l2ChainOpts.ChainPk == "" {
		return nil, fmt.Errorf("chainpk must be set")
	}

	ethClient, txSigner, err := chainutils.ConnectToChain(
		context.Background(),
		l2ChainOpts.ChainUrl,
		l2ChainOpts.ChainAuthToken,
		common.Hex2Bytes(l2ChainOpts.ChainPk),
	)
	if err != nil {
		panic(err)
	}

	na, err := Bridge.NewBridge(l2ChainOpts.bridgeAddress, ethClient)
	if err != nil {
		panic(err)
	}

	return newL2ChainService(ethClient, l2ChainOpts.ChainStartBlockNum, na, l2ChainOpts.bridgeAddress, txSigner)
}

// newEthChainService constructs a chain service that submits transactions to a NitroAdjudicator
// and listens to events from an eventSource
func newL2ChainService(chain ethChain, startBlockNum uint64, bridge *Bridge.Bridge,
	bridgeAddress common.Address, txSigner *bind.TransactOpts,
) (*L2ChainService, error) {
	ctx, cancelCtx := context.WithCancel(context.Background())

	logger := logging.LoggerWithAddress(slog.Default(), txSigner.From)

	block, err := chain.BlockByNumber(ctx, new(big.Int).SetUint64(startBlockNum))
	if err != nil {
		cancelCtx()
		return nil, err
	}
	startBlock := Block{
		BlockNum:  block.NumberU64(),
		Timestamp: block.Time(),
	}
	tracker := NewEventTracker(startBlock)

	// Use a buffered channel so we don't have to worry about blocking on writing to the channel.
	l2cs := L2ChainService{chain, bridge, bridgeAddress, txSigner, make(chan Event, 10), logger, ctx, cancelCtx, &sync.WaitGroup{}, tracker, nil, nil}
	errChan, newBlockChan, eventChan, eventQuery, err := l2cs.subscribeForLogs()
	if err != nil {
		return nil, err
	}

	// Prevent go routines from processing events before checkForMissedEvents completes
	l2cs.eventTracker.mu.Lock()
	defer l2cs.eventTracker.mu.Unlock()

	l2cs.wg.Add(3)
	go l2cs.listenForEventLogs(errChan, eventChan, eventQuery)
	go l2cs.listenForNewBlocks(errChan, newBlockChan)
	go l2cs.listenForErrors(errChan)

	// Search for any missed events emitted while this node was offline
	err = l2cs.checkForMissedEvents(startBlock.BlockNum)
	if err != nil {
		return nil, err
	}

	return &l2cs, nil
}

func (l2cs *L2ChainService) checkForMissedEvents(startBlock uint64) error {
	// Fetch the latest block
	latestBlock, err := l2cs.chain.BlockByNumber(l2cs.ctx, nil)
	if err != nil {
		return err
	}

	latestBlockNum := latestBlock.NumberU64()
	l2cs.logger.Info("checking for missed chain events", "startBlock", startBlock, "currentBlock", latestBlockNum)

	// Loop through in chunks of MAX_EPOCHS
	for currentStart := startBlock; currentStart <= latestBlockNum; {
		currentEnd := currentStart + MAX_EPOCHS
		if currentEnd > latestBlockNum {
			currentEnd = latestBlockNum
		}

		// Create a query for the current chunk
		query := ethereum.FilterQuery{
			FromBlock: big.NewInt(int64(currentStart)),
			ToBlock:   big.NewInt(int64(currentEnd)),
			Addresses: []common.Address{l2cs.bridgeAddress},
			Topics:    [][]common.Hash{topicsToWatch},
		}

		// Fetch logs for the current chunk
		missedEvents, err := l2cs.chain.FilterLogs(l2cs.ctx, query)
		if err != nil {
			l2cs.logger.Error("failed to retrieve old chain logs. " + err.Error())
			errorMsg := "*** To avoid this error, consider increasing the chainstartblock value in your configuration before restarting the node."
			errorMsg += " Note that this may cause your node to miss chain events emitted prior to the chainstartblock."
			l2cs.logger.Error(errorMsg)
			return err
		}
		l2cs.logger.Info("finished checking for missed chain events in range", "fromBlock", currentStart, "toBlock", currentEnd, "numMissedEvents", len(missedEvents))

		for _, event := range missedEvents {
			l2cs.eventTracker.Push(event)
		}

		currentStart = currentEnd + 1 // Move to the next chunk
	}

	return nil
}

// listenForErrors listens for errors on the error channel and attempts to handle them if they occur.
// TODO: Currently "handle" is panicking
func (l2cs *L2ChainService) listenForErrors(errChan <-chan error) {
	for {
		select {
		case <-l2cs.ctx.Done():
			l2cs.wg.Done()
			return
		case err := <-errChan:
			l2cs.logger.Error("chain service error", "error", err)
			panic(err)
		}
	}
}

// defaultTxOpts returns transaction options suitable for most transaction submissions
func (l2cs *L2ChainService) defaultTxOpts() *bind.TransactOpts {
	return &bind.TransactOpts{
		From:      l2cs.txSigner.From,
		Nonce:     l2cs.txSigner.Nonce,
		Signer:    l2cs.txSigner.Signer,
		GasFeeCap: l2cs.txSigner.GasFeeCap,
		GasTipCap: l2cs.txSigner.GasTipCap,
		GasLimit:  l2cs.txSigner.GasLimit,
		GasPrice:  l2cs.txSigner.GasPrice,
	}
}

// SendTransaction sends the transaction and blocks until it has been submitted.
func (l2cs *L2ChainService) SendTransaction(tx protocols.ChainTransaction) error {
	switch tx := tx.(type) {
	case protocols.UpdateMirroredChannelStatusTransaction:
		_, err := l2cs.bridge.UpdateMirroredChannelStatus(l2cs.defaultTxOpts(), tx.ChannelId(), tx.StateHash, tx.OutcomeHash)
		return err
	case protocols.GetMirroredChannelStatusTransaction:
		_, err := l2cs.bridge.GetMirroredChannelStatus(&bind.CallOpts{}, tx.ChannelId())
		return err
	default:
		return fmt.Errorf("unexpected transaction type %T", tx)
	}
}

// dispatchChainEvents takes in a collection of event logs from the chain
// and dispatches events to the out channel
func (l2cs *L2ChainService) dispatchChainEvents(logs []ethTypes.Log) error {
	for _, l := range logs {
		_, err := l2cs.chain.BlockByHash(context.Background(), l.BlockHash)
		if err != nil {
			return fmt.Errorf("error in getting block by hash %w", err)
		}

		switch l.Topics[0] {

		default:
			l2cs.logger.Info("Ignoring unknown chain event topic", "topic", l.Topics[0].String())

		}
	}
	return nil
}

func (l2cs *L2ChainService) listenForEventLogs(errorChan chan<- error, eventChan chan ethTypes.Log, eventQuery ethereum.FilterQuery) {
	for {
		select {
		case <-l2cs.ctx.Done():
			l2cs.eventSub.Unsubscribe()
			l2cs.wg.Done()
			return

		case err := <-l2cs.eventSub.Err():
			// Use helper function block to ensure "defer" statement is called for all exit paths
			func() {
				latestBlockNum := l2cs.GetLastConfirmedBlockNum()

				l2cs.eventTracker.mu.Lock()
				defer l2cs.eventTracker.mu.Unlock()

				if err != nil {
					l2cs.logger.Warn("error in chain event subscription: " + err.Error())
					l2cs.eventSub.Unsubscribe()
				} else {
					l2cs.logger.Warn("chain event subscription closed")
				}

				resubscribed := false // Flag to indicate whether resubscription was successful

				// Use exponential backoff loop to attempt to re-establish subscription
				for backoffTime := MIN_BACKOFF_TIME; backoffTime < MAX_BACKOFF_TIME; backoffTime *= 2 {
					eventSub, err := l2cs.chain.SubscribeFilterLogs(l2cs.ctx, eventQuery, eventChan)
					if err != nil {
						l2cs.logger.Warn("failed to resubscribe to chain events, retrying", "backoffTime", backoffTime)
						time.Sleep(backoffTime)
						continue
					}

					l2cs.eventSub = eventSub
					l2cs.logger.Debug("resubscribed to chain events")
					err = l2cs.checkForMissedEvents(latestBlockNum)
					if err != nil {
						errorChan <- fmt.Errorf("subscribeFilterLogs failed during checkForMissedEvents: " + err.Error())
						return
					}

					resubscribed = true
					break
				}

				if !resubscribed {
					l2cs.logger.Error("subscribeFilterLogs failed to resubscribe")
					errorChan <- fmt.Errorf("subscribeFilterLogs failed to resubscribe")
					return
				}
			}()

		case <-time.After(RESUB_INTERVAL):
			// Due to https://github.com/ethereum/go-ethereum/issues/23845 we can't rely on a long running subscription.
			// We unsub here and recreate the subscription in the next iteration of the select.
			l2cs.eventSub.Unsubscribe()

		case chainEvent := <-eventChan:
			l2cs.logger.Debug("queueing new chainEvent", "block-num", chainEvent.BlockNumber)
			l2cs.updateEventTracker(errorChan, nil, &chainEvent)
		}
	}
}

func (l2cs *L2ChainService) listenForNewBlocks(errorChan chan<- error, newBlockChan chan *ethTypes.Header) {
	for {
		select {
		case <-l2cs.ctx.Done():
			l2cs.newBlockSub.Unsubscribe()
			l2cs.wg.Done()
			return

		case err := <-l2cs.newBlockSub.Err():
			if err != nil {
				l2cs.logger.Warn("error in chain new block subscription: " + err.Error())
				l2cs.newBlockSub.Unsubscribe()
			} else {
				l2cs.logger.Warn("chain new block subscription closed")
			}

			// Use exponential backoff loop to attempt to re-establish subscription
			retryFailed := true
			for backoffTime := MIN_BACKOFF_TIME; backoffTime < MAX_BACKOFF_TIME; backoffTime *= 2 {
				newBlockSub, err := l2cs.chain.SubscribeNewHead(l2cs.ctx, newBlockChan)
				if err != nil {
					l2cs.logger.Warn("subscribeNewHead failed to resubscribe: " + err.Error())
					time.Sleep(backoffTime)
					continue
				}

				l2cs.newBlockSub = newBlockSub
				l2cs.logger.Debug("resubscribed to chain new blocks")
				retryFailed = false
				break
			}

			if retryFailed {
				errorChan <- fmt.Errorf("subscribeNewHead failed to resubscribe")
				return
			}

		case newBlock := <-newBlockChan:
			block := Block{BlockNum: newBlock.Number.Uint64(), Timestamp: newBlock.Time}
			l2cs.logger.Log(l2cs.ctx, logging.LevelTrace, "detected new block", "block-num", block.BlockNum)
			l2cs.updateEventTracker(errorChan, &block, nil)
		}
	}
}

// updateEventTracker accepts a new block number and/or new event and dispatches a chain event if there are enough block confirmations
func (l2cs *L2ChainService) updateEventTracker(errorChan chan<- error, block *Block, chainEvent *ethTypes.Log) {
	// lock the mutex for the shortest amount of time. The mutex only need to be locked to update the eventTracker data structure
	l2cs.eventTracker.mu.Lock()

	if block != nil && block.BlockNum > l2cs.eventTracker.latestBlock.BlockNum {
		l2cs.eventTracker.latestBlock = *block
	}

	if chainEvent != nil {
		l2cs.eventTracker.Push(*chainEvent)
		l2cs.logger.Debug("event added to queue", "updated-queue-length", l2cs.eventTracker.events.Len())
	}

	eventsToDispatch := []ethTypes.Log{}
	for l2cs.eventTracker.events.Len() > 0 && l2cs.eventTracker.latestBlock.BlockNum >= (l2cs.eventTracker.events)[0].BlockNumber+REQUIRED_BLOCK_CONFIRMATIONS {
		chainEvent := l2cs.eventTracker.Pop()
		l2cs.logger.Debug("event popped from queue", "updated-queue-length", l2cs.eventTracker.events.Len())

		// Ensure event & associated tx is still in the chain before adding to eventsToDispatch
		oldBlock, err := l2cs.chain.BlockByNumber(context.Background(), new(big.Int).SetUint64(chainEvent.BlockNumber))
		if err != nil {
			l2cs.logger.Error("failed to fetch block: %v", err)
			errorChan <- fmt.Errorf("failed to fetch block: %v", err)
			return
		}

		if oldBlock.Hash() != chainEvent.BlockHash {
			l2cs.logger.Warn("dropping event because its block is no longer in the chain (possible re-org)", "blockNumber", chainEvent.BlockNumber, "blockHash", chainEvent.BlockHash)
			continue
		}

		eventsToDispatch = append(eventsToDispatch, chainEvent)
	}
	l2cs.eventTracker.mu.Unlock()

	err := l2cs.dispatchChainEvents(eventsToDispatch)
	if err != nil {
		errorChan <- fmt.Errorf("failed dispatchChainEvents: %w", err)
		return
	}
}

// subscribeForLogs subscribes for logs and pushes them to the out channel.
// It relies on notifications being supported by the chain node.
func (l2cs *L2ChainService) subscribeForLogs() (chan error, chan *ethTypes.Header, chan ethTypes.Log, ethereum.FilterQuery, error) {
	// Subscribe to Adjudicator events
	eventQuery := ethereum.FilterQuery{
		Addresses: []common.Address{l2cs.bridgeAddress},
		Topics:    [][]common.Hash{topicsToWatch},
	}
	eventChan := make(chan ethTypes.Log)
	eventSub, err := l2cs.chain.SubscribeFilterLogs(l2cs.ctx, eventQuery, eventChan)
	if err != nil {
		return nil, nil, nil, ethereum.FilterQuery{}, fmt.Errorf("subscribeFilterLogs failed: %w", err)
	}
	l2cs.eventSub = eventSub
	errorChan := make(chan error)

	newBlockChan := make(chan *ethTypes.Header)
	newBlockSub, err := l2cs.chain.SubscribeNewHead(l2cs.ctx, newBlockChan)
	if err != nil {
		return nil, nil, nil, ethereum.FilterQuery{}, fmt.Errorf("subscribeNewHead failed: %w", err)
	}
	l2cs.newBlockSub = newBlockSub

	return errorChan, newBlockChan, eventChan, eventQuery, nil
}

// EventFeed returns the out chan, and narrows the type so that external consumers may only receive on it.
func (l2cs *L2ChainService) EventFeed() <-chan Event {
	return l2cs.out
}

func (l2cs *L2ChainService) GetChainId() (*big.Int, error) {
	return l2cs.chain.ChainID(l2cs.ctx)
}

func (l2cs *L2ChainService) GetConsensusAppAddress() types.Address {
	return types.Address{}
}

func (l2cs *L2ChainService) GetVirtualPaymentAppAddress() types.Address {
	return types.Address{}
}

func (l2cs *L2ChainService) GetLastConfirmedBlockNum() uint64 {
	var confirmedBlockNum uint64

	l2cs.eventTracker.mu.Lock()
	defer l2cs.eventTracker.mu.Unlock()

	// Check for potential underflow
	if l2cs.eventTracker.latestBlock.BlockNum >= REQUIRED_BLOCK_CONFIRMATIONS {
		confirmedBlockNum = l2cs.eventTracker.latestBlock.BlockNum - REQUIRED_BLOCK_CONFIRMATIONS
	} else {
		confirmedBlockNum = 0
	}

	return confirmedBlockNum
}

func (l2cs *L2ChainService) GetLatestBlock() Block {
	l2cs.eventTracker.mu.Lock()
	defer l2cs.eventTracker.mu.Unlock()
	return l2cs.eventTracker.latestBlock
}

func (l2cs *L2ChainService) Close() error {
	l2cs.cancel()
	l2cs.wg.Wait()
	return nil
}
