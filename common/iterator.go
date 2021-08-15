package common

// Iterator итератор, используется для слабой связи между сервисами приложения
type Iterator struct {
	// элементы
	items []interface{}

	// указатель на текущий элемент
	current int
}

// NewIterator создает итератор
func NewIterator(items []interface{}) *Iterator {
	return &Iterator{items: items, current: -1}
}

// First отдает первый элемент
func (i Iterator) First() interface{} {
	return i.items[0]
}

// Next отдает следующий элемент
func (i *Iterator) Next() interface{} {
	var item interface{}
	i.current++
	if i.isValidCurrent() {
		item = i.items[i.current]
	}
	return item
}

// проверяет, что указатель на элемент не превысил количества элементов
func (i *Iterator) isValidCurrent() bool {
	return i.current < len(i.items)
}

// Current отдает текущий элемент
func (i Iterator) Current() interface{} {
	var item interface{}
	if i.isValidCurrent() {
		item = i.items[i.current]
	}
	return item
}

// IsDone сигнализирует об окончании итерации
func (i Iterator) IsDone() bool {
	return i.current >= len(i.items)
}
