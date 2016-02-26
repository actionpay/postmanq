package analyser

import (
	"github.com/byorty/clitable"
	"regexp"
	"sort"
)

// автор таблиц
type TableWriter interface {

	// добавляет идентификатора по ключу
	Add(string, int)

	// экспортирует данные от одного автора другому
	Export(TableWriter)

	// возвращает идентификатору по ключу
	Ids() map[string][]int

	// устанавливает регулярное выражение для ключей
	SetKeyPattern(string)

	// устанавливает лимит
	SetLimit(int)

	// сигнализирует, нужен ли список email-ов после таблицы
	SetNecessaryExport(bool)

	// устанавливает сдвиг
	SetOffset(int)

	// устанавливает строки для вывода в таблице
	SetRows(RowWriters)

	// устанавливает регулярное выражение для значения строки таблицы
	SetValuePattern(string)

	// выводит ьаблицу
	Show()
}

// строки таблицы
type RowWriters map[int]RowWriter

// строка таблицы
type RowWriter interface {

	// записывает строку в таблицу, если строка удовлетворяет регулярному выражению
	Write(*clitable.Table, *regexp.Regexp)
}

// базовый автор таблицы
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

// создает базовый автор таблицы
func newAbstractTableWriter(fields []interface{}) *AbstractTableWriter {
	return &AbstractTableWriter{
		Table: clitable.NewTable(fields...),
		ids:   make(map[string][]int),
	}
}

// добавляет идентификатора по ключу
func (a *AbstractTableWriter) Add(key string, id int) {
	if _, ok := a.ids[key]; !ok {
		a.ids[key] = make([]int, 0)
	}
	existsIds := a.ids[key]
	for _, existsId := range existsIds {
		if existsId != id {
			existsIds = append(existsIds, id)
			break
		}
	}
}

// экспортирует данные от одного автора другому
func (a *AbstractTableWriter) Export(writer TableWriter) {
	a.ids = writer.Ids()
}

// возвращает идентификатору по ключу
func (a *AbstractTableWriter) Ids() map[string][]int {
	return a.ids
}

// устанавливает регулярное выражение для ключей
func (a *AbstractTableWriter) SetKeyPattern(pattern string) {
	a.keyPattern = pattern
}

// устанавливает лимит
func (a *AbstractTableWriter) SetLimit(limit int) {
	a.limit = limit
}

// сигнализирует, нужен ли список email-ов после таблицы
func (a *AbstractTableWriter) SetNecessaryExport(necessaryExport bool) {
	a.necessaryExport = necessaryExport
}

// устанавливает сдвиг
func (a *AbstractTableWriter) SetOffset(offset int) {
	a.offset = offset
}

// устанавливает строки для вывода в таблице
func (a *AbstractTableWriter) SetRows(rows RowWriters) {
	a.rows = rows
}

// устанавливает регулярное выражение для значения строки таблицы
func (a *AbstractTableWriter) SetValuePattern(pattern string) {
	a.valuePattern = pattern
}
