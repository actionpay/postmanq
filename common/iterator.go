package common

// Итератор, используется для слабой связи между сервисами приложения
type Iterator struct {
	// Элементы
	items []interface{}

	// Указатель на текущий элемент
	current int
}

// Создает итератор
func NewIterator(items []interface{}) *Iterator {
	return &Iterator{items: items, current: -1}
}

// Отдает первый элемент
func (i Iterator) First() interface{} {
	return i.items[0]
}

// Отдает следующий элемент
func (i *Iterator) Next() interface{} {
	var item interface{}
	i.current++
	if i.isValidCurrent() {
		item = i.items[i.current]
	}
	return item
}

func (i *Iterator) isValidCurrent() bool {
	return i.current < len(i.items)
}

// Отдает текущий элемент
func (i Iterator) Current() interface{} {
	var item interface{}
	if i.isValidCurrent() {
		item = i.items[i.current]
	}
	return item
}

// Сигнализирует об окончании итерации
func (i Iterator) IsDone() bool {
	return i.current >= len(i.items)
}
