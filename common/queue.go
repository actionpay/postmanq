package common

import "sync"

// Queue потоко-безопасная очередь
type Queue struct {
	// флаг, сигнализирующий, что очередь пуста
	empty bool

	// элементы очереди
	items []interface{}

	// семафор
	mutex *sync.Mutex
}

// NewQueue создает новую очередь
func NewQueue() *Queue {
	return &Queue{
		empty: true,
		items: make([]interface{}, 0),
		mutex: new(sync.Mutex),
	}
}

// Push добавляет элемент в конец очереди
func (q *Queue) Push(item interface{}) {
	q.mutex.Lock()
	if q.empty {
		q.empty = false
	}
	q.items = append(q.items, item)
	q.mutex.Unlock()
}

// Pop достает первый элемент из очереди
func (q *Queue) Pop() interface{} {
	var item interface{}
	q.mutex.Lock()
	if !q.empty {
		oldItems := q.items
		oldItemsLen := len(oldItems)
		if oldItemsLen > 0 {
			item = oldItems[oldItemsLen-1]
			q.items = oldItems[0 : oldItemsLen-1]
		} else {
			q.empty = true
		}
	}
	q.mutex.Unlock()
	return item
}

// Empty сигнализирует, что очередь пуста
func (q *Queue) Empty() bool {
	var empty bool
	q.mutex.Lock()
	empty = q.empty
	q.mutex.Unlock()
	return empty
}

// Len возвращает длину очереди
func (q *Queue) Len() int {
	var itemsLen int
	q.mutex.Lock()
	itemsLen = len(q.items)
	q.mutex.Unlock()
	return itemsLen
}

// статус очереди
type queueStatus int

const (
	// лимитированная очередь
	limitedQueueStatus queueStatus = iota

	// безлимитная очередь
	unlimitedQueueStatus
)

// LimitedQueue лимитированная очередь, в ней будут храниться клиенты к почтовым сервисам
type LimitedQueue struct {
	*Queue

	// статус, говорящий заблокирована очередь или нет
	status queueStatus

	// максимальное количество элементов, которое было в очереди
	maxLen int
}

// NewLimitQueue создает новую лимитированную очередь
func NewLimitQueue() *LimitedQueue {
	return &LimitedQueue{
		Queue:  NewQueue(),
		status: unlimitedQueueStatus,
	}
}

// HasLimit сигнализирует, что очередь имеет лимит
func (l *LimitedQueue) HasLimit() bool {
	l.mutex.Lock()
	hasLimit := l.status == limitedQueueStatus
	l.mutex.Unlock()
	return hasLimit
}

// HasLimitOn устанавливает лимит очереди
func (l *LimitedQueue) HasLimitOn() {
	if l.MaxLen() > 0 && !l.HasLimit() {
		l.setStatus(limitedQueueStatus)
	}
}

// HasLimitOff снимает лимит очереди
func (l *LimitedQueue) HasLimitOff() {
	l.setStatus(unlimitedQueueStatus)
}

// устанавливает статус очереди
func (l *LimitedQueue) setStatus(status queueStatus) {
	l.mutex.Lock()
	l.status = status
	l.mutex.Unlock()
}

// MaxLen максимальная длина очереди до того момента, как был установлен лимит
func (l *LimitedQueue) MaxLen() int {
	l.mutex.Lock()
	maxLen := l.maxLen
	l.mutex.Unlock()
	return maxLen
}

// AddMaxLen увеличивает максимальную длину очереди
func (l *LimitedQueue) AddMaxLen() {
	l.mutex.Lock()
	l.maxLen++
	l.mutex.Unlock()
}
