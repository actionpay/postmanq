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
	return &Iterator{items: items}
}

// Отдает первый элемент
func (i Iterator) First() interface{} {
	return i.items[0]
}

// Отдает следующий элемент
func (i *Iterator) Next() interface{} {
	var item interface{}
	i.current++
	if i.current < len(i.items) {
		item = i.items[i.current]
	}
	return item
}

// Отдает текущий элемент
func (i Iterator) Current() interface{} {
	return i.items[i.current]
}

// Сигнализирует об окончании итерации
func (i Iterator) IsDone() bool {
	return i.current >= len(i.items)
}
