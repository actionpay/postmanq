package analyser

import (
	"github.com/byorty/clitable"
	"regexp"
	"time"
)

type Report struct {
	Id           int
	Envelope     string
	Recipient    string
	Code         int
	Message      string
	CreatedDates []time.Time
}

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

type AggregateRow []int

func (a AggregateRow) Write(table *clitable.Table, valueRegex *regexp.Regexp) {
	table.AddRow(a[0], a[1], a[2], a[3])
}
