package common

import (
	"sync"
	"sync/atomic"
)

type Queue struct {
	empty bool
	items []interface{}
	mutex *sync.Mutex
}

func NewQueue() *Queue {
	return &Queue{
		empty: true,
		items: make([]interface{}, 0),
		mutex: new(sync.Mutex),
	}
}

func (q *Queue) Push(item interface{}) {
	q.mutex.Lock()
	if q.empty {
		q.empty = false
	}
	q.items = append(q.items, item)
	q.mutex.Unlock()
}

func (q *Queue) Pop() interface{} {
	var item interface{}
	q.mutex.Lock()
	if !q.empty {
		oldItems := q.items
		oldItemsLen := len(oldItems)
		item = oldItems[oldItemsLen-1]
		q.items = oldItems[0 : oldItemsLen-1]
		if oldItemsLen == 0 {
			q.empty = true
		}
	}
	q.mutex.Unlock()
	return item
}

func (q *Queue) Empty() bool {
	var empty bool
	q.mutex.Lock()
	empty = q.empty
	q.mutex.Unlock()
	return empty
}

func (q *Queue) Len() int {
	var itemsLen int
	q.mutex.Lock()
	itemsLen = len(q.items)
	q.mutex.Unlock()
	return itemsLen
}

const (
	limitedQueueStatus int32 = iota
	unlimitedQueueStatus
)

type LimitedQueue struct {
	*Queue
	status     int32
	maxLen     int32
	limitMutex *sync.Mutex
}

func NewLimitQueue() *LimitedQueue {
	return &LimitedQueue{
		Queue: NewQueue(),
	}
}

func (l *LimitedQueue) HasLimit() bool {
	return atomic.LoadInt32(&(l.status)) == limitedQueueStatus
}

func (l *LimitedQueue) HasLimitOn() {
	if l.MaxLen() > 0 && !l.HasLimit() {
		l.setStatus(limitedQueueStatus)
	}
}

func (l *LimitedQueue) HasLimitOff() {
	l.setStatus(unlimitedQueueStatus)
}

func (l *LimitedQueue) setStatus(status int32) {
	atomic.StoreInt32(&(l.status), status)
}

func (l *LimitedQueue) MaxLen() int32 {
	return atomic.LoadInt32(&(l.maxLen))
}

func (l *LimitedQueue) AddMaxLen() {
	atomic.AddInt32(&(l.maxLen), 1)
}
