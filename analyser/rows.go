package analyser

import (
	"regexp"
	"time"

	"github.com/byorty/clitable"
)

// Report отчет об ошибке
type Report struct {
	// Id идентификатор
	Id int

	// Envelope отправитель
	Envelope string

	// Recipient получатель
	Recipient string

	// Code код ошибки
	Code int

	// Message сообщение об ошибке
	Message string

	// CreatedDates даты отправок
	CreatedDates []time.Time
}

// Write записывает отчет в таблицу
func (r Report) Write(table *clitable.Table, valueRegex *regexp.Regexp) {
	if valueRegex == nil ||
		(valueRegex != nil &&
			(valueRegex.MatchString(r.Envelope) ||
				valueRegex.MatchString(r.Recipient) ||
				valueRegex.MatchString(r.Message))) {
		table.AddRow(
			r.Envelope,
			r.Recipient,
			r.Code,
			r.Message,
			len(r.CreatedDates),
		)
	}
}

// AggregateRow агрегированная строка
type AggregateRow []int

// Write записывает строку в таблицу
func (a AggregateRow) Write(table *clitable.Table, valueRegex *regexp.Regexp) {
	table.AddRow(a[0], a[1], a[2], a[3])
}
