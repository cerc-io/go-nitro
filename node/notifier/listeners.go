package notifier

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/statechannels/go-nitro/node/query"
	"github.com/statechannels/go-nitro/protocols"
)

// swapListeners is a struct that holds a list of listeners for swap channel info.
type swapListeners struct {
	// listeners is a list of listeners for swap info that we need to notify.
	listeners []chan query.SwapInfo
	// prev is the previous swap channel info that was sent to the listeners.
	prev query.SwapInfo
	// listenersLock is used to protect against concurrent access to to sibling struct members.
	listenersLock *sync.Mutex
}

// newSwapListeners constructs a new swap listeners struct.
func newSwapListeners() *swapListeners {
	return &swapListeners{listeners: []chan query.SwapInfo{}, listenersLock: &sync.Mutex{}}
}

// Notify notifies all listeners of a swap update.
// It only notifies listeners if the new info is different from the previous info.
func (li *swapListeners) Notify(info query.SwapInfo) {
	li.listenersLock.Lock()
	defer li.listenersLock.Unlock()
	if li.prev.Id == info.Id && li.prev.ChannelId == info.ChannelId {
		return
	}

	for _, list := range li.listeners {
		list <- info
	}
	li.prev = info
}

// createNewListener creates a new listener and adds it to the list of listeners.
func (li *swapListeners) createNewListener() <-chan query.SwapInfo {
	li.listenersLock.Lock()
	defer li.listenersLock.Unlock()
	// Use a buffered channel to avoid blocking the notifier.
	listener := make(chan query.SwapInfo, 1000)
	li.listeners = append(li.listeners, listener)
	return listener
}

// getOrCreateListener returns the first listener, creating one if none exist.
func (li *swapListeners) getOrCreateListener() <-chan query.SwapInfo {
	li.listenersLock.Lock()
	if len(li.listeners) != 0 {
		l := li.listeners[0]
		li.listenersLock.Unlock()
		return l
	}
	li.listenersLock.Unlock()
	return li.createNewListener()
}

// Close closes any active listeners.
func (li *swapListeners) Close() error {
	li.listenersLock.Lock()
	defer li.listenersLock.Unlock()
	for _, c := range li.listeners {
		close(c)
	}

	return nil
}

// paymentChannelListeners is a struct that holds a list of listeners for payment channel info.
type paymentChannelListeners struct {
	// listeners is a list of listeners for payment channel info that we need to notify.
	listeners []chan query.PaymentChannelInfo
	// prev is the previous payment channel info that was sent to the listeners.
	prev query.PaymentChannelInfo
	// listenersLock is used to protect against concurrent access to to sibling struct members.
	listenersLock *sync.Mutex
}

// newPaymentChannelListeners constructs a new payment channel listeners struct.
func newPaymentChannelListeners() *paymentChannelListeners {
	return &paymentChannelListeners{listeners: []chan query.PaymentChannelInfo{}, listenersLock: &sync.Mutex{}}
}

// Notify notifies all listeners of a payment channel update.
// It only notifies listeners if the new info is different from the previous info.
func (li *paymentChannelListeners) Notify(info query.PaymentChannelInfo) {
	li.listenersLock.Lock()
	defer li.listenersLock.Unlock()
	if li.prev.Equal(info) {
		return
	}
	for i, list := range li.listeners {
		list <- info
		marshalledInfo, err := json.Marshal(info)

		if err != nil {
			slog.Debug("DEBUG: listeners.go-Notify for paymentChannelListeners error marshalling paymentChannelInfo", "listenerNum", i, "error", err)
		} else {
			slog.Debug("DEBUG: listeners.go-Notify for paymentChannelListeners", "listenerNum", i, "paymentChannelInfo", string(marshalledInfo))
		}

	}
	li.prev = info
}

// createNewListener creates a new listener and adds it to the list of listeners.
func (li *paymentChannelListeners) createNewListener() <-chan query.PaymentChannelInfo {
	li.listenersLock.Lock()
	defer li.listenersLock.Unlock()
	// Use a buffered channel to avoid blocking the notifier.
	listener := make(chan query.PaymentChannelInfo, 1000)
	li.listeners = append(li.listeners, listener)
	return listener
}

// getOrCreateListener returns the first listener, creating one if none exist.
func (li *paymentChannelListeners) getOrCreateListener() <-chan query.PaymentChannelInfo {
	li.listenersLock.Lock()
	if len(li.listeners) != 0 {
		l := li.listeners[0]
		li.listenersLock.Unlock()
		return l
	}
	li.listenersLock.Unlock()
	return li.createNewListener()
}

// Close closes any active listeners.
func (li *paymentChannelListeners) Close() error {
	li.listenersLock.Lock()
	defer li.listenersLock.Unlock()
	for _, c := range li.listeners {
		close(c)
	}

	return nil
}

// ledgerChannelListeners is a struct that holds a list of listeners for ledger channel info.
type ledgerChannelListeners struct {
	// listeners is a list of listeners for ledger channel info that we need to notify.
	listeners []chan query.LedgerChannelInfo
	// prev is the previous ledger channel info that was sent to the listeners.
	prev query.LedgerChannelInfo
	// listenersLock is used to protect against concurrent access to sibling struct members.
	listenersLock sync.Mutex
}

// newLedgerChannelListeners constructs a new ledger channel listeners struct.
func newLedgerChannelListeners() *ledgerChannelListeners {
	return &ledgerChannelListeners{listeners: []chan query.LedgerChannelInfo{}, listenersLock: sync.Mutex{}}
}

// Notify notifies all listeners of a ledger channel update.
// It only notifies listeners if the new info is different from the previous info.
func (li *ledgerChannelListeners) Notify(info query.LedgerChannelInfo) {
	li.listenersLock.Lock()
	defer li.listenersLock.Unlock()
	if li.prev.Equal(info) {
		return
	}

	for _, list := range li.listeners {
		list <- info
	}
	li.prev = info
}

// createNewListener creates a new listener and adds it to the list of listeners.
func (li *ledgerChannelListeners) createNewListener() <-chan query.LedgerChannelInfo {
	li.listenersLock.Lock()
	defer li.listenersLock.Unlock()
	// Use a buffered channel to avoid blocking the notifier.
	listener := make(chan query.LedgerChannelInfo, 1000)
	li.listeners = append(li.listeners, listener)
	return listener
}

// getOrCreateListener returns the first listener, creating one if none exist.
func (li *ledgerChannelListeners) getOrCreateListener() <-chan query.LedgerChannelInfo {
	li.listenersLock.Lock()
	if len(li.listeners) != 0 {
		l := li.listeners[0]
		li.listenersLock.Unlock()
		return l
	}
	li.listenersLock.Unlock()
	return li.createNewListener()
}

// Close closes all listeners.
func (li *ledgerChannelListeners) Close() error {
	li.listenersLock.Lock()
	defer li.listenersLock.Unlock()
	for _, c := range li.listeners {
		close(c)
	}

	return nil
}

type completedObjectivesListeners struct {
	// listeners is a list of listeners for completed objectives that we need to notify
	listeners []chan protocols.ObjectiveId
	// listenersLock is used to protect against concurrent access to sibling struct members
	listenersLock sync.Mutex
}

func newCompletedObjectivesListeners() *completedObjectivesListeners {
	return &completedObjectivesListeners{listeners: []chan protocols.ObjectiveId{}, listenersLock: sync.Mutex{}}
}

// createNewListener creates a new listener and adds it to the list of listeners
func (li *completedObjectivesListeners) createNewListener() <-chan protocols.ObjectiveId {
	li.listenersLock.Lock()
	defer li.listenersLock.Unlock()
	// Use a buffered channel to avoid blocking the notifier.
	listener := make(chan protocols.ObjectiveId, 1000)
	li.listeners = append(li.listeners, listener)
	return listener
}

// broadcastCompletedObjective broadcasts the completed objectives to all the listeners
func (li *completedObjectivesListeners) broadcastCompletedObjective(objectiveId protocols.ObjectiveId) {
	li.listenersLock.Lock()
	defer li.listenersLock.Unlock()

	for _, listener := range li.listeners {
		select {
		case listener <- objectiveId:
		default:
		}
	}
}

// Close closes all listeners
func (li *completedObjectivesListeners) Close() error {
	li.listenersLock.Lock()
	defer li.listenersLock.Unlock()
	for _, c := range li.listeners {
		close(c)
	}

	return nil
}
