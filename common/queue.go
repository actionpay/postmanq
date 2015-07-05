package common

import "sync"

// Потоко-безопасная очередь
type Queue struct {
	// Флаг, сигнализирующий, что очередь пуста
	empty bool

	// Элементы очереди
	items []interface{}

	// Семафор
	mutex *sync.Mutex
}

// Создает новую очередь
func NewQueue() *Queue {
	return &Queue{
		empty: true,
		items: make([]interface{}, 0),
		mutex: new(sync.Mutex),
	}
}

// Добавляет элемент в конец очереди
func (q *Queue) Push(item interface{}) {
	q.mutex.Lock()
	if q.empty {
		q.empty = false
	}
	q.items = append(q.items, item)
	q.mutex.Unlock()
}

// Достает первый элемент из очереди
func (q *Queue) Pop() interface{} {
	var item interface{}
	q.mutex.Lock()
	if !q.empty {
		oldItems := q.items
		oldItemsLen := len(oldItems)
		if oldItemsLen > 0 {
			item = oldItems[oldItemsLen-1]
			q.items = oldItems[0:oldItemsLen-1]
		} else {
			q.empty = true
		}
	}
	q.mutex.Unlock()
	return item
}

// Сигнализирует, что очередь пуста
func (q *Queue) Empty() bool {
	var empty bool
	q.mutex.Lock()
	empty = q.empty
	q.mutex.Unlock()
	return empty
}

// Возвращает длину очереди
func (q *Queue) Len() int {
	var itemsLen int
	q.mutex.Lock()
	itemsLen = len(q.items)
	q.mutex.Unlock()
	return itemsLen
}

// Статус очереди
type queueStatus int

const (
	// Лимитированная очередь
	limitedQueueStatus queueStatus = iota

	// Безлимитная очередь
	unlimitedQueueStatus
)

// Лимитированная очередь, в ней будут храниться клиенты к почтовым сервисам
type LimitedQueue struct {
	*Queue

	// Статус, говорящий заблокирована очередь или нет
	status queueStatus

	// Максимальное количество элементов, которое было в очереди
	maxLen int
}

// Создает новую лимитированную очередь
func NewLimitQueue() *LimitedQueue {
	return &LimitedQueue{
		Queue:  NewQueue(),
		status: unlimitedQueueStatus,
	}
}

// Сигнализирует, что очередь имеет лимит
func (l *LimitedQueue) HasLimit() bool {
	l.mutex.Lock()
	hasLimit := l.status == limitedQueueStatus
	l.mutex.Unlock()
	return hasLimit
}

// Устанавливает лимит очереди
func (l *LimitedQueue) HasLimitOn() {
	if l.MaxLen() > 0 && !l.HasLimit() {
		l.setStatus(limitedQueueStatus)
	}
}

// Снимает лимит очереди
func (l *LimitedQueue) HasLimitOff() {
	l.setStatus(unlimitedQueueStatus)
}

// Устанавливает статус очереди
func (l *LimitedQueue) setStatus(status queueStatus) {
	l.mutex.Lock()
	l.status = status
	l.mutex.Unlock()
}

// Максимальная длина очереди до того момента, как был установлен лимит
func (l *LimitedQueue) MaxLen() int {
	l.mutex.Lock()
	maxLen := l.maxLen
	l.mutex.Unlock()
	return maxLen
}

// Увеличивает максимальную длину очереди
func (l *LimitedQueue) AddMaxLen() {
	l.mutex.Lock()
	l.maxLen++
	l.mutex.Unlock()
}
