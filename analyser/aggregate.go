package analyser

type TypeAggregateTableWriter struct {
	*AbstractTableWriter
}

func newTypeAggregateTableWriter(fields []interface {}) TableWriter {
	return &TypeAggregateTableWriter{
		newAbstractTableWriter(fields),
	}
}

func (t *TypeAggregateTableWriter) Show() {
	for key, ids := range t.ids {
		t.AddRow(key, len(ids))
	}
	t.Print()
}

type AggregateTableWriter struct {
	*AbstractTableWriter
}

func newAggregateTableWriter(fields []interface {}) TableWriter {
	return &AggregateTableWriter{
		newAbstractTableWriter(fields),
	}
}

func (t *AggregateTableWriter) Show() {
	for key, ids := range t.ids {
		t.AddRow(key, len(ids))
	}
	t.Print()
}

