package chainservice

import (
	"container/heap"
	"sync"

	"github.com/ethereum/go-ethereum/core/types"
)

type eventTracker struct {
	latestBlock LatestBlock
	events      eventQueue
	mu          sync.Mutex
}

type LatestBlock struct {
	BlockNum  uint64
	Timestamp uint64
}

func NewEventTracker(startBlock LatestBlock) *eventTracker {
	eventQueue := eventQueue{}
	heap.Init(&eventQueue)
	return &eventTracker{latestBlock: startBlock, events: eventQueue}
}

func (eT *eventTracker) Push(l types.Log) {
	heap.Push(&eT.events, (l))
}

func (eT *eventTracker) Pop() types.Log {
	return heap.Pop(&eT.events).(types.Log)
}

type eventQueue []types.Log

func (q eventQueue) Len() int { return len(q) }
func (q eventQueue) Less(i, j int) bool {
	if q[i].BlockNumber == q[j].BlockNumber {
		return i < j
	}
	return q[i].BlockNumber < q[j].BlockNumber
}

func (q eventQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }

func (q *eventQueue) Push(x interface{}) {
	*q = append(*q, x.(types.Log))
}

func (q *eventQueue) Pop() interface{} {
	old := *q
	n := len(old)
	x := old[n-1]
	*q = old[0 : n-1]
	return x
}
