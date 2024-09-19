package chainservice

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/internal/logging"
	"github.com/statechannels/go-nitro/internal/safesync"
	NitroAdjudicator "github.com/statechannels/go-nitro/node/engine/chainservice/adjudicator"
	Token "github.com/statechannels/go-nitro/node/engine/chainservice/erc20"
	chainutils "github.com/statechannels/go-nitro/node/engine/chainservice/utils"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

type ChainOpts struct {
	ChainUrl           string
	ChainStartBlockNum uint64
	ChainAuthToken     string
	ChainPk            string
	NaAddress          common.Address
	VpaAddress         common.Address
	CaAddress          common.Address
}

var (
	naAbi, _                 = NitroAdjudicator.NitroAdjudicatorMetaData.GetAbi()
	concludedTopic           = naAbi.Events["Concluded"].ID
	allocationUpdatedTopic   = naAbi.Events["AllocationUpdated"].ID
	depositedTopic           = naAbi.Events["Deposited"].ID
	challengeRegisteredTopic = naAbi.Events["ChallengeRegistered"].ID
	challengeClearedTopic    = naAbi.Events["ChallengeCleared"].ID
	reclaimedTopic           = naAbi.Events["Reclaimed"].ID
	L2ToL1MapUpdatedTopic    = naAbi.Events["L2ToL1MapUpdated"].ID
)

var topicsToWatch = []common.Hash{
	allocationUpdatedTopic,
	concludedTopic,
	depositedTopic,
	challengeRegisteredTopic,
	challengeClearedTopic,
	reclaimedTopic,
	L2ToL1MapUpdatedTopic,
}

var topicsToEventName = map[common.Hash]string{
	concludedTopic:           "Concluded",
	allocationUpdatedTopic:   "AllocationUpdated",
	depositedTopic:           "Deposited",
	challengeRegisteredTopic: "ChallengeRegistered",
	challengeClearedTopic:    "ChallengeCleared",
	reclaimedTopic:           "Reclaimed",
	L2ToL1MapUpdatedTopic:    "L2ToL1MapUpdated",
}

const (
	MIN_BACKOFF_TIME = 1 * time.Second
	MAX_BACKOFF_TIME = 5 * time.Minute
)

type ethChain interface {
	bind.ContractBackend
	ethereum.TransactionReader
	ethereum.ChainReader
	ChainID(ctx context.Context) (*big.Int, error)
	TransactionSender(ctx context.Context, tx *ethTypes.Transaction, block common.Hash, index uint) (common.Address, error)
}

// eventTracker holds on to events in memory and dispatches an event after required number of confirmations
type EthChainService struct {
	chain                    ethChain
	na                       *NitroAdjudicator.NitroAdjudicator
	naAddress                common.Address
	consensusAppAddress      common.Address
	virtualPaymentAppAddress common.Address
	txSigner                 *bind.TransactOpts
	eventEngineOut           chan Event
	eventOut                 chan Event
	droppedEventEngineOut    chan protocols.DroppedEventInfo
	droppedEventOut          chan protocols.DroppedEventInfo
	logger                   *slog.Logger
	ctx                      context.Context
	cancel                   context.CancelFunc
	wg                       *sync.WaitGroup
	eventTracker             *eventTracker
	eventSub                 ethereum.Subscription
	newBlockSub              ethereum.Subscription
	sentTxToChannelIdMap     *safesync.Map[types.Destination]
}

// MAX_QUERY_BLOCK_RANGE is the maximum range of blocks we query for events at once.
// Most json-rpc nodes restrict the amount of blocks you can search.
// For example Wallaby supports a maximum range of 2880
// See https://github.com/Zondax/rosetta-filecoin/blob/b395b3e04401be26c6cdf6a419e14ce85e2f7331/tools/wallaby/files/config.toml#L243
const MAX_QUERY_BLOCK_RANGE = 2000

// RESUB_INTERVAL is how often we resubscribe to log events.
// We do this to avoid https://github.com/ethereum/go-ethereum/issues/23845
// We use 2.5 minutes as the default filter timeout is 5 minutes.
// See https://github.com/ethereum/go-ethereum/blob/e14164d516600e9ac66f9060892e078f5c076229/eth/filters/filter_system.go#L43
// This has been reduced to 15 seconds to support local devnets with much shorter timeouts.
const RESUB_INTERVAL = 15 * time.Second

// REQUIRED_BLOCK_CONFIRMATIONS is how many blocks must be mined before an emitted event is processed
const REQUIRED_BLOCK_CONFIRMATIONS = 3

// MAX_EPOCHS is the maximum range of old epochs we can query with a single "FilterLogs" request
// This is a restriction enforced by the rpc provider
const MAX_EPOCHS = 60480

// BLOCKS_WITHOUT_EVENT_THRESHOLD is the maximum number of blocks the node will wait for an event to confirm transaction to be mined
const BLOCKS_WITHOUT_EVENT_THRESHOLD = 16

// GAS_LIMIT_MULTIPLIER is the multiplier for updating gas limit
const GAS_LIMIT_MULTIPLIER = 1.5

// NewEthChainService is a convenient wrapper around newEthChainService, which provides a simpler API
func NewEthChainService(chainOpts ChainOpts) (ChainService, error) {
	if chainOpts.ChainPk == "" {
		return nil, fmt.Errorf("chainpk must be set")
	}
	if chainOpts.VpaAddress == chainOpts.CaAddress {
		return nil, fmt.Errorf("virtual payment app address and consensus app address cannot be the same: %s", chainOpts.VpaAddress.String())
	}

	ethClient, txSigner, err := chainutils.ConnectToChain(
		context.Background(),
		chainOpts.ChainUrl,
		chainOpts.ChainAuthToken,
		common.Hex2Bytes(chainOpts.ChainPk),
	)
	if err != nil {
		panic(err)
	}

	na, err := NitroAdjudicator.NewNitroAdjudicator(chainOpts.NaAddress, ethClient)
	if err != nil {
		panic(err)
	}

	return newEthChainService(ethClient, chainOpts.ChainStartBlockNum, na, chainOpts.NaAddress, chainOpts.CaAddress, chainOpts.VpaAddress, txSigner)
}

// newEthChainService constructs a chain service that submits transactions to a NitroAdjudicator
// and listens to events from an eventSource
func newEthChainService(chain ethChain, startBlockNum uint64, na *NitroAdjudicator.NitroAdjudicator,
	naAddress, caAddress, vpaAddress common.Address, txSigner *bind.TransactOpts,
) (*EthChainService, error) {
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
	sentTxToChannelIdMap := safesync.Map[types.Destination]{}

	// Use a buffered channel so we don't have to worry about blocking on writing to the channel.
	ecs := EthChainService{
		chain,
		na,
		naAddress,
		caAddress,
		vpaAddress,
		txSigner,
		make(chan Event, 10),
		make(chan Event, 10),
		make(chan protocols.DroppedEventInfo, 10),
		make(chan protocols.DroppedEventInfo, 10),
		logger, ctx, cancelCtx, &sync.WaitGroup{},
		tracker,
		nil,
		nil,
		&sentTxToChannelIdMap,
	}

	errChan, newBlockChan, eventChan, eventQuery, err := ecs.subscribeForLogs()
	if err != nil {
		return nil, err
	}

	// Prevent go routines from processing events before checkForMissedEvents completes
	ecs.eventTracker.mu.Lock()
	defer ecs.eventTracker.mu.Unlock()

	ecs.wg.Add(3)
	go ecs.listenForEventLogs(errChan, eventChan, eventQuery)
	go ecs.listenForNewBlocks(errChan, newBlockChan)
	go ecs.listenForErrors(errChan)

	// Search for any missed events emitted while this node was offline
	err = ecs.checkForMissedEvents(startBlock.BlockNum)
	if err != nil {
		return nil, err
	}

	return &ecs, nil
}

func (ecs *EthChainService) checkForMissedEvents(startBlock uint64) error {
	// Fetch the latest block
	latestBlock, err := ecs.chain.BlockByNumber(ecs.ctx, nil)
	if err != nil {
		return err
	}

	latestBlockNum := latestBlock.NumberU64()
	ecs.logger.Info("checking for missed chain events", "startBlock", startBlock, "currentBlock", latestBlockNum)

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
			Addresses: []common.Address{ecs.naAddress},
			Topics:    [][]common.Hash{topicsToWatch},
		}

		// Fetch logs for the current chunk
		missedEvents, err := ecs.chain.FilterLogs(ecs.ctx, query)
		if err != nil {
			ecs.logger.Error("failed to retrieve old chain logs. " + err.Error())
			errorMsg := "*** To avoid this error, consider increasing the chainstartblock value in your configuration before restarting the node."
			errorMsg += " Note that this may cause your node to miss chain events emitted prior to the chainstartblock."
			ecs.logger.Error(errorMsg)
			return err
		}
		ecs.logger.Info("finished checking for missed chain events in range", "fromBlock", currentStart, "toBlock", currentEnd, "numMissedEvents", len(missedEvents))

		for _, event := range missedEvents {
			ecs.eventTracker.Push(event)
		}

		currentStart = currentEnd + 1 // Move to the next chunk
	}

	return nil
}

// listenForErrors listens for errors on the error channel and attempts to handle them if they occur.
// TODO: Currently "handle" is panicking
func (ecs *EthChainService) listenForErrors(errChan <-chan error) {
	for {
		select {
		case <-ecs.ctx.Done():
			ecs.wg.Done()
			return
		case err := <-errChan:
			ecs.logger.Error("chain service error", "error", err)
			panic(err)
		}
	}
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

// defaultCallOpts provides options to fine-tune a contract call request
func (ecs *EthChainService) defaultCallOpts() *bind.CallOpts {
	return &bind.CallOpts{
		Pending: false,
		From:    ecs.txSigner.From,
	}
}

// SendTransaction sends the transaction and blocks until it has been submitted.
func (ecs *EthChainService) SendTransaction(tx protocols.ChainTransaction) (*ethTypes.Transaction, error) {
	switch tx := tx.(type) {
	case protocols.DepositTransaction:
		var tokenApprovalLog ethTypes.Log

		for tokenAddress, amount := range tx.Deposit {
			txOpts := ecs.defaultTxOpts()
			ethTokenAddress := common.Address{}
			if tokenAddress == ethTokenAddress {
				txOpts.Value = amount
			} else {
				// TODO: Move Approve tx to a separate switch case so that Approval event parsing can go through dispatchEvents flow
				// If custom token is used instead of ETH, we need to approve token amount to be transferred from
				approvalLog, err := ecs.handleApproveTx(tokenAddress, amount)
				if err != nil {
					return nil, err
				}

				tokenApprovalLog = approvalLog
			}

			holdings, err := ecs.na.Holdings(&bind.CallOpts{}, tokenAddress, tx.ChannelId())
			ecs.logger.Debug("existing holdings", "holdings", holdings)

			if err != nil {
				return nil, err
			}

			depositTx, err := ecs.na.Deposit(txOpts, tokenAddress, tx.ChannelId(), holdings, amount)
			if err != nil {
				// Check if `Approve` tx was confirmed when custom token is used
				if tokenAddress != ethTokenAddress {
					return nil, err
				}

				approvalBlock, err := ecs.GetBlockByNumber(big.NewInt(int64(tokenApprovalLog.BlockNumber)))
				if err != nil {
					return nil, err
				}

				if approvalBlock.Hash() != tokenApprovalLog.BlockHash {
					ecs.droppedEventEngineOut <- protocols.DroppedEventInfo{
						TxHash:    tokenApprovalLog.TxHash,
						ChannelId: tx.ChannelId(),
						EventName: "Approval",
					}

					return nil, nil
				}
				// Return nil to not panic the node and log the error instead
				return nil, err
			}

			ecs.sentTxToChannelIdMap.Store(depositTx.Hash().String(), tx.ChannelId())
		}

		// TODO: Handle multiple depositTx
		return nil, nil
	case protocols.WithdrawAllTransaction:
		signedState := tx.SignedState.State()
		signatures := tx.SignedState.Signatures()
		nitroFixedPart := NitroAdjudicator.INitroTypesFixedPart(NitroAdjudicator.ConvertFixedPart(signedState.FixedPart()))
		nitroVariablePart := NitroAdjudicator.ConvertVariablePart(signedState.VariablePart())
		nitroSignatures := []NitroAdjudicator.INitroTypesSignature{NitroAdjudicator.ConvertSignature(signatures[0]), NitroAdjudicator.ConvertSignature(signatures[1])}

		candidate := NitroAdjudicator.INitroTypesSignedVariablePart{
			VariablePart: nitroVariablePart,
			Sigs:         nitroSignatures,
		}

		withdrawAllTx, err := ecs.na.ConcludeAndTransferAllAssets(ecs.defaultTxOpts(), nitroFixedPart, candidate)
		ecs.sentTxToChannelIdMap.Store(withdrawAllTx.Hash().String(), tx.ChannelId())
		return withdrawAllTx, err
	case protocols.ChallengeTransaction:
		fp, candidate := NitroAdjudicator.ConvertSignedStateToFixedPartAndSignedVariablePart(tx.Candidate)
		proof := NitroAdjudicator.ConvertSignedStatesToProof(tx.Proof)
		challengerSig := NitroAdjudicator.ConvertSignature(tx.ChallengerSig)
		challengeTx, err := ecs.na.Challenge(ecs.defaultTxOpts(), fp, proof, candidate, challengerSig)
		return challengeTx, err
	case protocols.CheckpointTransaction:
		fp, candidate := NitroAdjudicator.ConvertSignedStateToFixedPartAndSignedVariablePart(tx.Candidate)
		proof := NitroAdjudicator.ConvertSignedStatesToProof(tx.Proof)
		checkpointTx, err := ecs.na.Checkpoint(ecs.defaultTxOpts(), fp, proof, candidate)
		return checkpointTx, err
	case protocols.TransferAllTransaction:
		transferState := tx.TransferState.State()
		channelId := transferState.ChannelId()
		stateHash, err := transferState.Hash()
		if err != nil {
			return nil, err
		}

		nitroVariablePart := NitroAdjudicator.ConvertVariablePart(transferState.VariablePart())

		transferAllTx, er := ecs.na.TransferAllAssets(ecs.defaultTxOpts(), channelId, nitroVariablePart.Outcome, stateHash)
		return transferAllTx, er
	case protocols.ReclaimTransaction:
		reclaimTx, err := ecs.na.Reclaim(ecs.defaultTxOpts(), tx.ReclaimArgs)
		return reclaimTx, err
	case protocols.MirrorTransferAllTransaction:
		transferState := tx.TransferState.State()
		channelId := transferState.ChannelId()
		stateHash, err := transferState.Hash()
		if err != nil {
			return nil, err
		}

		nitroVariablePart := NitroAdjudicator.ConvertVariablePart(transferState.VariablePart())

		mirrorTransferAllTx, err := ecs.na.MirrorTransferAllAssets(ecs.defaultTxOpts(), channelId, nitroVariablePart.Outcome, stateHash)
		return mirrorTransferAllTx, err
	case protocols.SetL2ToL1Transaction:
		setL2ToL1Tx, err := ecs.na.SetL2ToL1(ecs.defaultTxOpts(), tx.ChannelId(), tx.MirrorChannelId)
		if err != nil {
			return nil, err
		}

		ecs.sentTxToChannelIdMap.Store(setL2ToL1Tx.Hash().String(), tx.ChannelId())
		return setL2ToL1Tx, nil

	case protocols.MirrorWithdrawAllTransaction:
		signedState := tx.SignedState.State()
		signatures := tx.SignedState.Signatures()
		nitroFixedPart := NitroAdjudicator.INitroTypesFixedPart(NitroAdjudicator.ConvertFixedPart(signedState.FixedPart()))
		nitroVariablePart := NitroAdjudicator.ConvertVariablePart(signedState.VariablePart())
		nitroSignatures := []NitroAdjudicator.INitroTypesSignature{NitroAdjudicator.ConvertSignature(signatures[0]), NitroAdjudicator.ConvertSignature(signatures[1])}

		candidate := NitroAdjudicator.INitroTypesSignedVariablePart{
			VariablePart: nitroVariablePart,
			Sigs:         nitroSignatures,
		}
		MirrorWithdrawAllTx, err := ecs.na.MirrorConcludeAndTransferAllAssets(ecs.defaultTxOpts(), nitroFixedPart, candidate)
		return MirrorWithdrawAllTx, err
	default:
		return nil, fmt.Errorf("unexpected transaction type %T", tx)
	}
}

// GetL1ChannelFromL2 returns the L1 ledger channel ID from the L2 ledger channel by making a contract call to the l2ToL1 map of the Nitro Adjudicator contract
func (ecs *EthChainService) GetL1ChannelFromL2(l2Channel types.Destination) (types.Destination, error) {
	return ecs.na.L2Tol1(ecs.defaultCallOpts(), l2Channel)
}

// dispatchChainEvents takes in a collection of event logs from the chain
// and dispatches events to the out channel
func (ecs *EthChainService) dispatchChainEvents(logs []ethTypes.Log) error {
	for _, l := range logs {
		block, err := ecs.chain.BlockByHash(context.Background(), l.BlockHash)
		if err != nil {
			return fmt.Errorf("error in getting block by hash %w", err)
		}

		switch l.Topics[0] {
		case depositedTopic:
			ecs.logger.Debug("Processing Deposited event")
			nad, err := ecs.na.ParseDeposited(l)
			if err != nil {
				return fmt.Errorf("error in ParseDeposited: %w", err)
			}

			event := NewDepositedEvent(nad.Destination, Block{BlockNum: l.BlockNumber, Timestamp: block.Time()}, l.TxIndex, nad.Asset, nad.DestinationHoldings, l.TxHash)
			ecs.eventEngineOut <- event

		case allocationUpdatedTopic:
			ecs.logger.Debug("Processing AllocationUpdated event")
			au, err := ecs.na.ParseAllocationUpdated(l)
			if err != nil {
				return fmt.Errorf("error in ParseAllocationUpdated: %w", err)
			}

			_, pending, err := ecs.chain.TransactionByHash(ecs.ctx, l.TxHash)
			if pending {
				return fmt.Errorf("expected transaction to be part of the chain, but the transaction is pending")
			}
			if err != nil {
				return fmt.Errorf("error in TransactionByHash: %w", err)
			}

			event := NewAllocationUpdatedEvent(au.ChannelId, Block{BlockNum: l.BlockNumber, Timestamp: block.Time()}, l.TxIndex, au.Asset, au.FinalHoldings, l.TxHash)
			ecs.eventEngineOut <- event

		case concludedTopic:
			ecs.logger.Debug("Processing Concluded event")
			ce, err := ecs.na.ParseConcluded(l)
			if err != nil {
				return fmt.Errorf("error in ParseConcluded: %w", err)
			}

			event := ConcludedEvent{commonEvent: commonEvent{channelID: ce.ChannelId, block: Block{BlockNum: l.BlockNumber, Timestamp: block.Time()}, txIndex: l.TxIndex, txHash: l.TxHash}}
			ecs.eventEngineOut <- event

		case challengeRegisteredTopic:
			ecs.logger.Debug("Processing Challenge Registered event")

			tx, pending, err := ecs.chain.TransactionByHash(context.Background(), l.TxHash)
			if pending {
				return fmt.Errorf("expected transaction to be part of the chain, but the transaction is pending")
			}
			if err != nil {
				return fmt.Errorf("error in TransactionByHash: %w", err)
			}

			txSenderAddress, err := ecs.chain.TransactionSender(context.Background(), tx, l.BlockHash, l.TxIndex)
			if err != nil {
				return fmt.Errorf("error in TransactionSender: %w", err)
			}

			cr, err := ecs.na.ParseChallengeRegistered(l)
			if err != nil {
				return fmt.Errorf("error in ParseChallengeRegistered: %w", err)
			}
			isInitiatedByMe := txSenderAddress == ecs.txSigner.From

			event := NewChallengeRegisteredEvent(
				cr.ChannelId,
				Block{BlockNum: l.BlockNumber, Timestamp: block.Time()},
				l.TxIndex,
				state.VariablePart{
					AppData: cr.Candidate.VariablePart.AppData,
					Outcome: NitroAdjudicator.ConvertBindingsExitToExit(cr.Candidate.VariablePart.Outcome),
					TurnNum: cr.Candidate.VariablePart.TurnNum.Uint64(),
					IsFinal: cr.Candidate.VariablePart.IsFinal,
				},
				NitroAdjudicator.ConvertBindingsSignaturesToSignatures(cr.Candidate.Sigs),
				cr.FinalizesAt,
				isInitiatedByMe,
				l.TxHash,
			)
			ecs.eventEngineOut <- event
		case challengeClearedTopic:
			ecs.logger.Debug("Processing Challenge Cleared event")
			cp, err := ecs.na.ParseChallengeCleared(l)
			if err != nil {
				return fmt.Errorf("error in ParseCheckpointed: %w", err)
			}
			event := NewChallengeClearedEvent(cp.ChannelId, Block{BlockNum: l.BlockNumber, Timestamp: block.Time()}, l.TxIndex, cp.NewTurnNumRecord, l.TxHash)
			ecs.eventEngineOut <- event

		case reclaimedTopic:
			ecs.logger.Debug("Processing Reclaimed event")
			ce, err := ecs.na.ParseReclaimed(l)
			if err != nil {
				return fmt.Errorf("error in ParseReclaimed: %w", err)
			}

			event := ReclaimedEvent{commonEvent: commonEvent{channelID: ce.ChannelId, block: Block{BlockNum: l.BlockNumber, Timestamp: block.Time()}, txIndex: l.TxIndex, txHash: l.TxHash}}
			ecs.eventEngineOut <- event

		case L2ToL1MapUpdatedTopic:
			ecs.logger.Debug("Processing l2 to l1 map updated event")

			channelMapUpdatedEvent, err := ecs.na.ParseL2ToL1MapUpdated(l)
			if err != nil {
				return fmt.Errorf("error in ParseL2ToL1MapUpdated: %w", err)
			}

			event := L2ToL1MapUpdated{commonEvent: commonEvent{block: Block{BlockNum: l.BlockNumber, Timestamp: block.Time()}, txIndex: l.TxIndex, txHash: l.TxHash}, l1ChannelId: channelMapUpdatedEvent.L1ChannelId, l2ChannelId: channelMapUpdatedEvent.L2ChannelId}

			// Use non-blocking send incase no-one is listening
			select {
			case ecs.eventOut <- event:
			default:
			}

		default:
			ecs.logger.Info("Ignoring unknown chain event topic", "topic", l.Topics[0].String())

		}
	}
	return nil
}

func (ecs *EthChainService) listenForEventLogs(errorChan chan<- error, eventChan chan ethTypes.Log, eventQuery ethereum.FilterQuery) {
	for {
		select {
		case <-ecs.ctx.Done():
			ecs.eventSub.Unsubscribe()
			ecs.wg.Done()
			return

		case err := <-ecs.eventSub.Err():
			latestBlockNum := ecs.GetLastConfirmedBlockNum()

			if err != nil {
				ecs.logger.Warn("error in chain event subscription: " + err.Error())
				ecs.eventSub.Unsubscribe()
			} else {
				ecs.logger.Warn("chain event subscription closed")
			}

			resubscribed := false // Flag to indicate whether resubscription was successful

			// Use exponential backoff loop to attempt to re-establish subscription
			for backoffTime := MIN_BACKOFF_TIME; backoffTime < MAX_BACKOFF_TIME; backoffTime *= 2 {
				select {
				// Exit from resubscription loop on closing chain service (cancelling context)
				// https://github.com/golang/go/issues/39483
				case <-time.After(backoffTime):
					eventSub, err := ecs.chain.SubscribeFilterLogs(ecs.ctx, eventQuery, eventChan)
					if err != nil {
						ecs.logger.Warn("failed to resubscribe to chain events, retrying", "backoffTime", backoffTime)
						continue
					}

					ecs.eventSub = eventSub
					ecs.logger.Debug("resubscribed to chain events")

					ecs.eventTracker.mu.Lock()
					err = ecs.checkForMissedEvents(latestBlockNum)
					ecs.eventTracker.mu.Unlock()

					if err != nil {
						errorChan <- fmt.Errorf("subscribeFilterLogs failed during checkForMissedEvents: " + err.Error())
						return
					}

					resubscribed = true

				case <-ecs.ctx.Done():
					ecs.wg.Done()
					ecs.eventSub.Unsubscribe()
					return
				}

				if resubscribed {
					break
				}
			}

			if !resubscribed {
				ecs.logger.Error("subscribeFilterLogs failed to resubscribe")
				errorChan <- fmt.Errorf("subscribeFilterLogs failed to resubscribe")
				return
			}

		case <-time.After(RESUB_INTERVAL):
			// Due to https://github.com/ethereum/go-ethereum/issues/23845 we can't rely on a long running subscription.
			// We unsub here and recreate the subscription in the next iteration of the select.
			ecs.eventSub.Unsubscribe()

		case chainEvent := <-eventChan:
			ecs.logger.Debug("queueing new chainEvent", "block-num", chainEvent.BlockNumber)
			ecs.updateEventTracker(errorChan, nil, &chainEvent)
		}
	}
}

func (ecs *EthChainService) listenForNewBlocks(errorChan chan<- error, newBlockChan chan *ethTypes.Header) {
	for {
		select {
		case <-ecs.ctx.Done():
			ecs.newBlockSub.Unsubscribe()
			ecs.wg.Done()
			return

		case err := <-ecs.newBlockSub.Err():
			if err != nil {
				ecs.logger.Warn("error in chain new block subscription: " + err.Error())
				ecs.newBlockSub.Unsubscribe()
			} else {
				ecs.logger.Warn("chain new block subscription closed")
			}

			// Use exponential backoff loop to attempt to re-establish subscription
			resubscribed := false // Flag to indicate whether resubscription was successful

			for backoffTime := MIN_BACKOFF_TIME; backoffTime < MAX_BACKOFF_TIME; backoffTime *= 2 {
				select {
				// Exit from resubscription loop on closing chain service (cancelling context)
				// https://github.com/golang/go/issues/39483
				case <-time.After(backoffTime):
					newBlockSub, err := ecs.chain.SubscribeNewHead(ecs.ctx, newBlockChan)
					if err != nil {
						ecs.logger.Warn("subscribeNewHead failed to resubscribe: " + err.Error())
						continue
					}

					ecs.newBlockSub = newBlockSub
					ecs.logger.Debug("resubscribed to chain new blocks")
					resubscribed = true

				case <-ecs.ctx.Done():
					ecs.newBlockSub.Unsubscribe()
					ecs.wg.Done()
					return
				}

				if resubscribed {
					break
				}
			}

			if !resubscribed {
				errorChan <- fmt.Errorf("subscribeNewHead failed to resubscribe")
				return
			}

		case newBlock := <-newBlockChan:
			block := Block{BlockNum: newBlock.Number.Uint64(), Timestamp: newBlock.Time}
			ecs.logger.Log(ecs.ctx, logging.LevelTrace, "detected new block", "block-num", block.BlockNum)
			ecs.updateEventTracker(errorChan, &block, nil)
		}
	}
}

// updateEventTracker accepts a new block number and/or new event and dispatches a chain event if there are enough block confirmations
func (ecs *EthChainService) updateEventTracker(errorChan chan<- error, block *Block, chainEvent *ethTypes.Log) {
	// lock the mutex for the shortest amount of time. The mutex only need to be locked to update the eventTracker data structure
	ecs.eventTracker.mu.Lock()

	if block != nil && block.BlockNum > ecs.eventTracker.latestBlock.BlockNum {
		ecs.eventTracker.latestBlock = *block
	}

	if chainEvent != nil {
		ecs.eventTracker.Push(*chainEvent)
		ecs.logger.Debug("event added to queue", "updated-queue-length", ecs.eventTracker.events.Len())
	}

	eventsToDispatch := []ethTypes.Log{}
	for ecs.eventTracker.events.Len() > 0 && ecs.eventTracker.latestBlock.BlockNum >= (ecs.eventTracker.events)[0].BlockNumber+REQUIRED_BLOCK_CONFIRMATIONS {
		chainEvent := ecs.eventTracker.Pop()
		ecs.logger.Debug("event popped from queue", "updated-queue-length", ecs.eventTracker.events.Len())

		// Ensure event & associated tx is still in the chain before adding to eventsToDispatch
		oldBlock, err := ecs.chain.BlockByNumber(context.Background(), new(big.Int).SetUint64(chainEvent.BlockNumber))
		if err != nil {
			ecs.logger.Error("failed to fetch block", "err", err)
			errorChan <- fmt.Errorf("failed to fetch block: %v", err)
			return
		}

		if oldBlock.Hash() != chainEvent.BlockHash {
			ecs.logger.Warn("dropping event because its block is no longer in the chain (possible re-org)", "blockNumber", chainEvent.BlockNumber, "blockHash", chainEvent.BlockHash)

			// Send info of dropped event to engine
			channelId, exists := ecs.sentTxToChannelIdMap.Load(chainEvent.TxHash.String())
			if !exists {
				continue
			}

			ecs.droppedEventEngineOut <- protocols.DroppedEventInfo{
				TxHash:    chainEvent.TxHash,
				ChannelId: channelId,
				EventName: topicsToEventName[chainEvent.Topics[0]],
			}

			// Use non-blocking send incase no-one is listening
			select {
			case ecs.droppedEventOut <- protocols.DroppedEventInfo{
				TxHash:    chainEvent.TxHash,
				ChannelId: channelId,
				EventName: topicsToEventName[chainEvent.Topics[0]],
			}:
			default:
			}

			ecs.sentTxToChannelIdMap.Delete(chainEvent.TxHash.String())

			continue
		}

		ecs.sentTxToChannelIdMap.Delete(chainEvent.TxHash.String())
		eventsToDispatch = append(eventsToDispatch, chainEvent)
	}
	ecs.eventTracker.mu.Unlock()

	err := ecs.dispatchChainEvents(eventsToDispatch)
	if err != nil {
		errorChan <- fmt.Errorf("failed dispatchChainEvents: %w", err)
		return
	}
}

// subscribeForLogs subscribes for logs and pushes them to the out channel.
// It relies on notifications being supported by the chain node.
func (ecs *EthChainService) subscribeForLogs() (chan error, chan *ethTypes.Header, chan ethTypes.Log, ethereum.FilterQuery, error) {
	// Subscribe to Adjudicator events
	eventQuery := ethereum.FilterQuery{
		Addresses: []common.Address{ecs.naAddress},
		Topics:    [][]common.Hash{topicsToWatch},
	}
	eventChan := make(chan ethTypes.Log)
	eventSub, err := ecs.chain.SubscribeFilterLogs(ecs.ctx, eventQuery, eventChan)
	if err != nil {
		return nil, nil, nil, ethereum.FilterQuery{}, fmt.Errorf("subscribeFilterLogs failed: %w", err)
	}
	ecs.eventSub = eventSub
	errorChan := make(chan error)

	newBlockChan := make(chan *ethTypes.Header)
	newBlockSub, err := ecs.chain.SubscribeNewHead(ecs.ctx, newBlockChan)
	if err != nil {
		return nil, nil, nil, ethereum.FilterQuery{}, fmt.Errorf("subscribeNewHead failed: %w", err)
	}
	ecs.newBlockSub = newBlockSub

	return errorChan, newBlockChan, eventChan, eventQuery, nil
}

// EventEngineFeed returns the out chan, and narrows the type so that external consumers may only receive on it.
func (ecs *EthChainService) EventEngineFeed() <-chan Event {
	return ecs.eventEngineOut
}

func (ecs *EthChainService) DroppedEventEngineFeed() <-chan protocols.DroppedEventInfo {
	return ecs.droppedEventEngineOut
}

func (ecs *EthChainService) GetConsensusAppAddress() types.Address {
	return ecs.consensusAppAddress
}

func (ecs *EthChainService) GetVirtualPaymentAppAddress() types.Address {
	return ecs.virtualPaymentAppAddress
}

func (ecs *EthChainService) GetChainId() (*big.Int, error) {
	return ecs.chain.ChainID(ecs.ctx)
}

func (ecs *EthChainService) GetLastConfirmedBlockNum() uint64 {
	var confirmedBlockNum uint64

	ecs.eventTracker.mu.Lock()
	defer ecs.eventTracker.mu.Unlock()

	// Check for potential underflow
	if ecs.eventTracker.latestBlock.BlockNum >= REQUIRED_BLOCK_CONFIRMATIONS {
		confirmedBlockNum = ecs.eventTracker.latestBlock.BlockNum - REQUIRED_BLOCK_CONFIRMATIONS
	} else {
		confirmedBlockNum = 0
	}

	return confirmedBlockNum
}

func (ecs *EthChainService) GetBlockByNumber(blockNum *big.Int) (*ethTypes.Block, error) {
	block, err := ecs.chain.BlockByNumber(context.Background(), blockNum)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (ecs *EthChainService) Close() error {
	ecs.cancel()
	ecs.wg.Wait()
	return nil
}

func (ecs *EthChainService) estimateGasForApproveTx(tokenAddress common.Address, amount *big.Int) (uint64, error) {
	parsedABI, err := abi.JSON(strings.NewReader(Token.TokenABI))
	if err != nil {
		return 0, fmt.Errorf("failed to parse ABI: %w", err)
	}

	data, err := parsedABI.Pack("approve", ecs.naAddress, amount)
	if err != nil {
		return 0, fmt.Errorf("failed to encode function call: %w", err)
	}

	callMsg := ethereum.CallMsg{
		From:     ecs.txSigner.From,
		To:       &tokenAddress,
		GasPrice: nil,
		Gas:      0,
		Value:    big.NewInt(0),
		Data:     data,
	}

	estimatedGasLimit, err := ecs.chain.EstimateGas(context.Background(), callMsg)
	if err != nil {
		return 0, fmt.Errorf("failed to estimate gas: %w", err)
	}

	return estimatedGasLimit, nil
}

func (ecs *EthChainService) handleApproveTx(tokenAddress common.Address, amount *big.Int) (ethTypes.Log, error) {
	token, err := Token.NewToken(tokenAddress, ecs.chain)
	if err != nil {
		return ethTypes.Log{}, err
	}

	approvalLogsChan := make(chan *Token.TokenApproval)

	newBlockChan := make(chan *ethTypes.Header)
	_, err = ecs.chain.SubscribeNewHead(ecs.ctx, newBlockChan)
	if err != nil {
		return ethTypes.Log{}, err
	}

	approvalSubscription, err := token.WatchApproval(&bind.WatchOpts{Context: ecs.ctx}, approvalLogsChan, []common.Address{ecs.txSigner.From}, []common.Address{ecs.naAddress})
	if err != nil {
		return ethTypes.Log{}, err
	}

	approveTx, err := token.Approve(ecs.defaultTxOpts(), ecs.naAddress, amount)
	if err != nil {
		return ethTypes.Log{}, err
	}

	// Get current block
	currentBlock := <-newBlockChan

	isApproveTxRetried := false

	// Transaction hash of retried Approve transaction
	var retryApproveTxHash common.Hash

	// Wait for the Approve transaction to be mined before continuing
	for {
		select {
		case log := <-approvalLogsChan:
			if log.Owner == ecs.txSigner.From {
				approvalSubscription.Unsubscribe()
				return log.Raw, nil
			}
		case err := <-approvalSubscription.Err():
			return ethTypes.Log{}, err
		case newBlock := <-newBlockChan:
			if (newBlock.Number.Int64() - currentBlock.Number.Int64()) > BLOCKS_WITHOUT_EVENT_THRESHOLD {
				if isApproveTxRetried {
					err := fmt.Errorf("approve transaction was retried with higher gas and event Approval was not emitted till latest block, txHash: %s, latestBlock: %s", retryApproveTxHash, newBlock.Number.String())
					return ethTypes.Log{}, err
				}

				slog.Error("event Approval was not emitted", "approveTxHash", approveTx.Hash().String())

				// Estimate gas for new Approve transaction
				estimatedGasLimit, err := ecs.estimateGasForApproveTx(tokenAddress, amount)
				if err != nil {
					return ethTypes.Log{}, err
				}

				approveTxOpts := ecs.defaultTxOpts()

				// Multiply estimated gas limit with set multiplier
				approveTxOpts.GasLimit = uint64(float64(estimatedGasLimit) * GAS_LIMIT_MULTIPLIER)
				reApproveTx, err := token.Approve(approveTxOpts, ecs.naAddress, amount)
				if err != nil {
					return ethTypes.Log{}, err
				}

				isApproveTxRetried = true
				currentBlock = newBlock
				retryApproveTxHash = reApproveTx.Hash()

				slog.Info("Resubmitted transaction with higher gas limit", "gasLimit", approveTxOpts.GasLimit, "approveTxHash", reApproveTx.Hash().String())
			}
		}
	}
}

func (ecs *EthChainService) GetChain() ethChain {
	return ecs.chain
}

func (ecs *EthChainService) DroppedEventFeed() <-chan protocols.DroppedEventInfo {
	return ecs.droppedEventOut
}

func (ecs *EthChainService) EventFeed() <-chan Event {
	return ecs.eventOut
}
