package common

type Iterator struct {
	items   []interface{}
	current int
}

func NewIterator(items []interface{}) *Iterator {
	return &Iterator{items: items}
}

func (i Iterator) First() interface{} {
	return i.items[0]
}

func (i *Iterator) Next() interface{} {
	var item interface{}
	i.current++
	if i.current < len(i.items) {
		item = i.items[i.current]
	}
	return item
}

func (i Iterator) Current() interface{} {
	return i.items[i.current]
}

func (i Iterator) IsDone() bool {
	return i.current >= len(i.items)
}
