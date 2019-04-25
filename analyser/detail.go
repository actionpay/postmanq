package analyser

import (
	"fmt"
	"github.com/Halfi/postmanq/common"
	"regexp"
	"strings"
)

// автор таблиц, выводящий детализированные отчеты об ошибке
type DetailTableWriter struct {
	*AbstractTableWriter
}

// создает нового автора таблицы, выводящего детализированные отчеты
func newDetailTableWriter(fields []interface{}) TableWriter {
	return &DetailTableWriter{
		newAbstractTableWriter(fields),
	}
}

// записывает данные в таблицу
func (d *DetailTableWriter) Show() {
	d.Clean()
	keyRegex, _ := regexp.Compile(d.keyPattern)
	valueRegex := regexp.MustCompile(d.valuePattern)
	addresses := make([]string, 0)
	rows := 0
	for key, ids := range d.ids {
		if d.keyPattern == "*" || (keyRegex != nil && keyRegex.MatchString(key)) {
			for _, id := range ids {
				if d.offset == common.InvalidInputInt {
					if d.limit == common.InvalidInputInt || (d.limit > common.InvalidInputInt && rows < d.limit) {
						row := d.rows[id]
						row.Write(d.Table, valueRegex)
						if d.necessaryExport {
							addresses = append(addresses, row.(Report).Recipient)
						}
						rows++
					}
				} else {
					d.offset--
				}
			}
		}
	}
	d.Print()
	if d.necessaryExport {
		fmt.Println()
		fmt.Println("Addresses:")
		fmt.Println(strings.Join(addresses, ", "))
	}
}
