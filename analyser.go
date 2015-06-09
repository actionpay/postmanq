package postmanq

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/byorty/clitable"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	analyser = new(Analyser)
)

type Analyser struct {
	mutex                *sync.Mutex
	messages             chan *MailMessage
	reports              []*Report
	reportsByCode        map[string][]int
	reportsByEnvelope    map[string][]int
	reportsByRecipient   map[string][]int
	necessaryAll         bool
	necessaryCode        string
	necessaryEnvelope    string
	necessaryRecipient   string
	necessaryExport      bool
	necessaryOnly        bool
	limit                int
	offset               int
	pattern              string
	tableAggregateFields []interface{}
	tableDetailFields    []interface{}
	tableCodeFields      []interface{}
	tableAddressFields   []interface{}
	table                *clitable.Table
}

func AnalyserOnce() *Analyser {
	return analyser
}

func (a *Analyser) OnInit(event *ApplicationEvent) {
	a.messages = make(chan *MailMessage)
	a.reports = make([]*Report, 0)
	a.reportsByCode = make(map[string][]int)
	a.reportsByEnvelope = make(map[string][]int)
	a.reportsByRecipient = make(map[string][]int)
	a.mutex = new(sync.Mutex)
	a.tableAggregateFields = []interface{}{
		"Mails count",
		"Code count",
		"Envelopes count",
		"Recipients count",
	}
	a.tableDetailFields = []interface{}{
		"Envelope",
		"Recipient",
		"Code",
		"Message",
		"Sending count",
	}
	a.tableCodeFields = []interface{}{
		"Code",
		"Mails count",
	}
	a.tableAddressFields = []interface{}{
		"Address",
		"Mails count",
	}
}

func (a *Analyser) OnShowReport() {
	for i := 0; i < defaultWorkersCount; i++ {
		go a.receiveMessages()
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		a.findReports(strings.Split(scanner.Text(), " "))
	}
}

func (a *Analyser) receiveMessages() {
	for message := range a.messages {
		a.receiveMessage(message)
	}
}

func (a *Analyser) receiveMessage(message *MailMessage) {
	var report *Report
	a.mutex.Lock()
	reportsLen := len(a.reports)
	for _, existsReport := range a.reports {
		if existsReport.Envelope == message.Envelope &&
			existsReport.Recipient == message.Recipient &&
			existsReport.Code == message.Error.Code {
			report = existsReport
			break
		}
	}
	if report == nil {
		report = &Report{
			Id:        reportsLen + 1,
			Envelope:  message.Envelope,
			Recipient: message.Recipient,
			Code:      message.Error.Code,
			Message:   message.Error.Message,
		}
		report.CreatedDates = make([]time.Time, 0)
		a.reports = append(a.reports, report)
	}

	report.CreatedDates = append(report.CreatedDates, message.CreatedDate)
	isValidCode := report.Code > 0
	code := strconv.Itoa(report.Code)

	if _, ok := a.reportsByCode[code]; !ok && isValidCode {
		a.reportsByCode[code] = make([]int, 0)
	}
	if _, ok := a.reportsByEnvelope[report.Envelope]; !ok {
		a.reportsByEnvelope[report.Envelope] = make([]int, 0)
	}
	if _, ok := a.reportsByRecipient[report.Recipient]; !ok {
		a.reportsByRecipient[report.Recipient] = make([]int, 0)
	}

	if isValidCode {
		a.reportsByCode[code] = append(a.reportsByCode[code], report.Id)
	}
	a.reportsByEnvelope[report.Envelope] = append(a.reportsByEnvelope[report.Envelope], report.Id)
	a.reportsByRecipient[report.Recipient] = append(a.reportsByRecipient[report.Recipient], report.Id)
	a.mutex.Unlock()
}

func (a *Analyser) findReports(args []string) {
	flagSet := flag.NewFlagSet("analyser", flag.ContinueOnError)
	flagSet.BoolVar(&a.necessaryAll, "a", false, "show all reports")
	flagSet.StringVar(&a.necessaryCode, "c", InvalidInputString, "show reports by code")
	flagSet.StringVar(&a.necessaryEnvelope, "e", InvalidInputString, "show reports by envelope")
	flagSet.StringVar(&a.necessaryRecipient, "r", InvalidInputString, "show reports by recipient")
	flagSet.BoolVar(&a.necessaryExport, "E", false, "export addresses recipients")
	flagSet.BoolVar(&a.necessaryOnly, "O", false, "show codes or envelopes or recipients without reports")
	flagSet.StringVar(&a.pattern, "s", InvalidInputString, "search by envelope or recipient or mail body")
	flagSet.IntVar(&a.limit, "l", InvalidInputInt, "limit reports")
	flagSet.IntVar(&a.offset, "o", InvalidInputInt, "offset reports")
	err := flagSet.Parse(args)
	if err == nil {
		re := a.createRegexp()
		switch {
		case len(a.necessaryCode) > 0:
			a.showDetailTable(a.reportsByCode, a.necessaryCode, re, a.tableCodeFields)
		case len(a.necessaryEnvelope) > 0:
			a.showDetailTable(a.reportsByEnvelope, a.necessaryEnvelope, re, a.tableAddressFields)
		case len(a.necessaryRecipient) > 0:
			a.showDetailTable(a.reportsByRecipient, a.necessaryRecipient, re, a.tableAddressFields)
		case a.necessaryAll:
			a.createTable(a.tableDetailFields)
			for _, report := range a.reports {
				a.fillRowIfMatch(re, report)
			}
			a.table.Print()
		default:
			a.showAggregateTable(flagSet)
		}
	} else {
		a.showAggregateTable(flagSet)
	}
}

func (a *Analyser) showDetailTable(aggregateReportIds map[string][]int, keyPattern string, valueRegex *regexp.Regexp, fields []interface{}) {
	if a.necessaryOnly {
		a.showAggregateTableForType(aggregateReportIds, fields)
	} else {
		keyRegex, _ := regexp.Compile(keyPattern)
		a.createTable(a.tableDetailFields)
		addresses := make([]string, 0)
		rows := 0
		for key, ids := range aggregateReportIds {
			if keyPattern == "*" || (keyRegex != nil && keyRegex.MatchString(key)) {
				for _, id := range ids {
					for _, report := range a.reports {
						if report.Id == id {
							if a.offset == InvalidInputInt {
								if a.limit == InvalidInputInt || (a.limit > InvalidInputInt && rows < a.limit) {
									a.fillRowIfMatch(valueRegex, report)
									if a.necessaryExport {
										addresses = append(addresses, report.Recipient)
									}
									rows++
								}
							} else {
								a.offset--
							}
							break
						}
					}
				}
			}
		}
		a.table.Print()
		if a.necessaryExport {
			fmt.Println()
			fmt.Println("Addresses:")
			fmt.Println(strings.Join(addresses, ", "))
		}
	}
}

func (a *Analyser) showAggregateTable(flagSet *flag.FlagSet) {
	fmt.Println()
	fmt.Println("Usage: -acer *|regex [-s] [-E] [-O] [-l] [-o]")
	flagSet.VisitAll(PrintUsage)
	fmt.Println("Example:")
	fmt.Println("  -c * -O             show error codes without reports")
	fmt.Println("  -c 550 -l 100       show 100 reports with 550 error")
	fmt.Println("  -c 550 -s gmail.com show reports with 550 error and hostname gmail.com")
	fmt.Println("  -c * -l 100 -o 200  show reports with limit and offset")
	a.createTable(a.tableAggregateFields)
	a.table.AddRow(
		len(a.reports),
		len(a.reportsByCode),
		len(a.reportsByEnvelope),
		len(a.reportsByRecipient),
	)
	a.table.Print()
}

func (a *Analyser) createRegexp() *regexp.Regexp {
	if len(a.pattern) > 0 {
		re, err := regexp.Compile(a.pattern)
		if err == nil {
			return re
		} else {
			return nil
		}
	} else {
		return nil
	}
}

func (a *Analyser) createTable(fields []interface{}) {
	a.table = clitable.NewTable(fields...)
}

func (a *Analyser) fillRowIfMatch(valueRegex *regexp.Regexp, report *Report) {
	if valueRegex == nil {
		a.fillRow(report)
	} else if valueRegex.MatchString(report.Envelope) ||
		valueRegex.MatchString(report.Recipient) ||
		valueRegex.MatchString(report.Message) {
		a.fillRow(report)
	}
}

func (a *Analyser) fillRow(report *Report) {
	a.table.AddRow(
		report.Envelope,
		report.Recipient,
		report.Code,
		report.Message,
		len(report.CreatedDates),
	)
}

func (a *Analyser) showAggregateTableForType(aggregateReportIds map[string][]int, fields []interface{}) {
	table := clitable.NewTable(fields...)
	for key, ids := range aggregateReportIds {
		table.AddRow(key, len(ids))
	}
	table.Print()
}

type Report struct {
	Id           int
	Envelope     string
	Recipient    string
	Code         int
	Message      string
	CreatedDates []time.Time
}
