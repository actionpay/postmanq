package analyser

// KeyAggregateTableWriter автор таблиц, агрегирующий отчеты по ключу, например по коду ошибки
type KeyAggregateTableWriter struct {
	*AbstractTableWriter
}

// создает нового автора таблицы, агрегирующего отчеты по ключу
func newKeyAggregateTableWriter(fields []interface{}) TableWriter {
	return &KeyAggregateTableWriter{
		newAbstractTableWriter(fields),
	}
}

// Show записывает данные в таблицу
func (t *KeyAggregateTableWriter) Show() {
	t.Clean()
	for key, ids := range t.ids {
		t.AddRow(key, len(ids))
	}
	t.Print()
}

// AggregateTableWriter автор таблиц, агрегирующий данные
type AggregateTableWriter struct {
	*AbstractTableWriter
}

// создает нового автора таблицы, агрегирующего данные
func newAggregateTableWriter(fields []interface{}) TableWriter {
	return &AggregateTableWriter{
		newAbstractTableWriter(fields),
	}
}

// Show записывает данные в таблицу
func (a *AggregateTableWriter) Show() {
	a.Clean()
	for _, row := range a.rows {
		row.Write(a.Table, nil)
	}
	a.Print()
}
