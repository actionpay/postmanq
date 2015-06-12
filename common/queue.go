package common

import "sync"

type Queue struct {
	empty bool
	items []interface {}
	mutex *sync.Mutex
}

func NewQueue() *Queue {
	return *Queue{
		empty: true,
		items: make([]interface {}, 0),
		mutex : new(sync.Mutex),
	}
}

func (q *Queue) Push(item interface {}) {
	q.mutex.Lock()
	if q.empty {
		q.empty = false
	}
	*q.items = append(*q.items, item)
	q.mutex.Unlock()
}

func (q *Queue) Pop() interface {} {
	var item interface {}
	q.mutex.Lock()
	if !q.empty {
		oldItems := *q.items
		oldItemsLen := len(oldItems)
		item = oldItems[oldItemsLen-1]
		*q.items = oldItems[0:oldItemsLen-1]
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
