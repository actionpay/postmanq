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
	t.Clean()
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

func (a *AggregateTableWriter) Show() {
	a.Clean()
	for _, row := range a.rows {
		row.Write(a.Table, nil)
	}
	a.Print()
}

