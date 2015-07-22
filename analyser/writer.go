package analyser

import (
	"github.com/byorty/clitable"
	"regexp"
	"sort"
)

type TableWriter interface {
	Add(string, int)
	Export(TableWriter)
	Ids() map[string][]int
	SetKeyPattern(string)
	SetLimit(int)
	SetNecessaryExport(bool)
	SetOffset(int)
	SetRows(RowWriters)
	SetValuePattern(string)
	Show()
}

type RowWriters map[int]RowWriter

type RowWriter interface {
	Write(*clitable.Table, *regexp.Regexp)
}

type AbstractTableWriter struct {
	*clitable.Table
	ids             map[string][]int
	keyPattern      string
	limit           int
	necessaryExport bool
	offset          int
	rows            RowWriters
	valuePattern    string
}

func newAbstractTableWriter(fields []interface{}) *AbstractTableWriter {
	return &AbstractTableWriter{
		Table: clitable.NewTable(fields...),
		ids:   make(map[string][]int),
	}
}

func (a *AbstractTableWriter) Add(key string, id int) {
	if _, ok := a.ids[key]; !ok {
		a.ids[key] = make([]int, 0)
	}
	idsLen := len(a.ids[key])
	if sort.Search(idsLen, func(i int) bool { return a.ids[key][i] == id }) == idsLen {
		a.ids[key] = append(a.ids[key], id)
	}
}

func (a *AbstractTableWriter) Export(writer TableWriter) {
	a.ids = writer.Ids()
}

func (a *AbstractTableWriter) Ids() map[string][]int {
	return a.ids
}

func (a *AbstractTableWriter) SetKeyPattern(pattern string) {
	a.keyPattern = pattern
}

func (a *AbstractTableWriter) SetLimit(limit int) {
	a.limit = limit
}

func (a *AbstractTableWriter) SetNecessaryExport(necessaryExport bool) {
	a.necessaryExport = necessaryExport
}

func (a *AbstractTableWriter) SetOffset(offset int) {
	a.offset = offset
}

func (a *AbstractTableWriter) SetRows(rows RowWriters) {
	a.rows = rows
}

func (a *AbstractTableWriter) SetValuePattern(pattern string) {
	a.valuePattern = pattern
}
