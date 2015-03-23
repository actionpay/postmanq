package postmanq

import (
	"sync"
	"time"
	"strconv"
	"flag"
	"bufio"
	"os"
	"strings"
	"regexp"
	"github.com/byorty/clitable"
	"fmt"
)

var (
	analyser *Analyser
)

type Analyser struct {
	mutex                 *sync.Mutex
	messages              chan *MailMessage
	reports               []*Report
	reportsByCode         map[string][]int
	reportsByEnvelope     map[string][]int
	reportsByRecipient    map[string][]int
	necessaryAll          bool
	necessaryCode         string
	necessaryEnvelope     string
	necessaryRecipient    string
	necessaryExport       bool
	necessaryOnly         bool
	limit                 int
	offset                int
	pattern               string
	tableAggregateFields  []interface{}
	tableDetailFields     []interface{}
	tableCodeFields       []interface{}
	tableAddressFields    []interface{}
	table                 *clitable.Table
}

func AnalyserOnce() *Analyser {
	if analyser == nil {
		analyser = new(Analyser)
	}
	return analyser
}

func (this *Analyser) OnInit(event *ApplicationEvent) {
	this.messages = make(chan *MailMessage)
	this.reports = make([]*Report, 0)
	this.reportsByCode = make(map[string][]int)
	this.reportsByEnvelope = make(map[string][]int)
	this.reportsByRecipient = make(map[string][]int)
	this.mutex = new(sync.Mutex)
	this.tableAggregateFields = []interface{} {
		"Mails count",
		"Code count",
		"Envelopes count",
		"Recipients count",
	}
	this.tableDetailFields = []interface{} {
		"Envelope",
		"Recipient",
		"Code",
		"Message",
		"Sending count",
	}
	this.tableCodeFields = []interface{} {
		"Code",
		"Mails count",
	}
	this.tableAddressFields = []interface{} {
		"Address",
		"Mails count",
	}
}

func (this *Analyser) OnShowReport() {
	for i := 0;i < defaultWorkersCount;i++ {
		go this.receiveMessages()
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		this.findReports(strings.Split(scanner.Text(), " "))
	}
}

func (this *Analyser) receiveMessages() {
	for message := range this.messages {
		this.receiveMessage(message)
	}
}

func (this *Analyser) receiveMessage(message *MailMessage) {
	var report *Report
	this.mutex.Lock()
	reportsLen := len(this.reports)
	for _, existsReport := range this.reports {
		if existsReport.Envelope == message.Envelope &&
			existsReport.Recipient == message.Recipient &&
			existsReport.Code == message.Error.Code &&
			existsReport.Message == message.Error.Message {
			report = existsReport
			break
		}
	}
	if report == nil {
		report = &Report{
			Id: reportsLen + 1,
			Envelope: message.Envelope,
			Recipient: message.Recipient,
			Code: message.Error.Code,
			Message: message.Error.Message,
		}
		report.CreatedDates = make([]time.Time, 0)
		this.reports = append(this.reports, report)
	}

	report.CreatedDates = append(report.CreatedDates, message.CreatedDate)
	isValidCode := report.Code > 0
	code := strconv.Itoa(report.Code)

	if _, ok := this.reportsByCode[code]; !ok && isValidCode {
		this.reportsByCode[code] = make([]int, 0)
	}
	if _, ok := this.reportsByEnvelope[report.Envelope]; !ok {
		this.reportsByEnvelope[report.Envelope] = make([]int, 0)
	}
	if _, ok := this.reportsByRecipient[report.Recipient]; !ok {
		this.reportsByRecipient[report.Recipient] = make([]int, 0)
	}

	if isValidCode {
		this.reportsByCode[code] = append(this.reportsByCode[code], report.Id)
	}
	this.reportsByEnvelope[report.Envelope] = append(this.reportsByEnvelope[report.Envelope], report.Id)
	this.reportsByRecipient[report.Recipient] = append(this.reportsByRecipient[report.Recipient], report.Id)
	this.mutex.Unlock()
}

func (this *Analyser) findReports(args []string) {
	flagSet := flag.NewFlagSet("analyser", flag.ContinueOnError)
	flagSet.BoolVar(&this.necessaryAll, "a", false, "show all reports")
	flagSet.StringVar(&this.necessaryCode, "c", INVALID_INPUT_STRING, "show reports by code")
	flagSet.StringVar(&this.necessaryEnvelope, "e", INVALID_INPUT_STRING, "show reports by envelope")
	flagSet.StringVar(&this.necessaryRecipient, "r", INVALID_INPUT_STRING, "show reports by recipient")
	flagSet.BoolVar(&this.necessaryExport, "E", false, "export addresses recipients")
	flagSet.BoolVar(&this.necessaryOnly, "O", false, "show codes or envelopes or recipients without reports")
	flagSet.StringVar(&this.pattern, "s", INVALID_INPUT_STRING, "search by envelope or recipient or mail body")
	flagSet.IntVar(&this.limit, "l", INVALID_INPUT_INT, "limit reports")
	flagSet.IntVar(&this.offset, "o", INVALID_INPUT_INT, "offset reports")
	err := flagSet.Parse(args)
	if err == nil {
		re := this.createRegexp()
		switch {
		case len(this.necessaryCode) > 0: this.showDetailTable(this.reportsByCode, this.necessaryCode, re, this.tableCodeFields)
		case len(this.necessaryEnvelope) > 0: this.showDetailTable(this.reportsByEnvelope, this.necessaryEnvelope, re, this.tableAddressFields)
		case len(this.necessaryRecipient) > 0: this.showDetailTable(this.reportsByRecipient, this.necessaryRecipient, re, this.tableAddressFields)
		case this.necessaryAll:
			this.createTable(this.tableDetailFields)
			for _, report := range this.reports {
				this.fillRowIfMatch(re, report)
			}
			this.table.Print()
		default: this.showAggregateTable(flagSet)
		}
	} else {
		this.showAggregateTable(flagSet)
	}
}

func (this *Analyser) showDetailTable(aggregateReportIds map[string][]int, keyPattern string, valueRegex *regexp.Regexp, fields []interface{}) {
	if this.necessaryOnly {
		this.showAggregateTableForType(aggregateReportIds, fields)
	} else {
		keyRegex, _ := regexp.Compile(keyPattern)
		this.createTable(this.tableDetailFields)
		addresses := make([]string, 0)
		rows := 0
		for key, ids := range aggregateReportIds {
			if keyPattern == "*" || (keyRegex != nil && keyRegex.MatchString(key))  {
				for _, id := range ids {
					for _, report := range this.reports {
						if report.Id == id {
							if this.offset == INVALID_INPUT_INT {
								if this.limit == INVALID_INPUT_INT || (this.limit > INVALID_INPUT_INT && rows < this.limit) {
									this.fillRowIfMatch(valueRegex, report)
									if this.necessaryExport {
										addresses = append(addresses, report.Recipient)
									}
									rows++
								}
							} else {
								this.offset--
							}
							break
						}
					}
				}
			}
		}
		this.table.Print()
		if this.necessaryExport {
			fmt.Println()
			fmt.Println("Addresses:")
			fmt.Println(strings.Join(addresses, ", "))
		}
	}
}

func (this *Analyser) showAggregateTable(flagSet *flag.FlagSet) {
	fmt.Println()
	fmt.Println("Usage: -acer *|regex [-s] [-E] [-O] [-l] [-o]")
	flagSet.VisitAll(PrintUsage)
	fmt.Println("Example:")
	fmt.Println("  -c * -O             show error codes without reports")
	fmt.Println("  -c 550 -l 100       show 100 reports with 550 error")
	fmt.Println("  -c 550 -s gmail.com show reports with 550 error and hostname gmail.com")
	fmt.Println("  -c * -l 100 -o 200  show reports with limit and offset")
	this.createTable(this.tableAggregateFields)
	this.table.AddRow(
		len(this.reports),
		len(this.reportsByCode),
		len(this.reportsByEnvelope),
		len(this.reportsByRecipient),
	)
	this.table.Print()
}

func (this *Analyser) createRegexp() *regexp.Regexp {
	if len(this.pattern) > 0 {
		re, err := regexp.Compile(this.pattern)
		if err == nil {
			return re
		} else {
			return nil
		}
	} else {
		return nil
	}
}

func (this *Analyser) createTable(fields []interface{}) {
	this.table = clitable.NewTable(fields...)
}

func (this *Analyser) fillRowIfMatch(valueRegex *regexp.Regexp, report *Report) {
	if valueRegex == nil {
		this.fillRow(report)
	} else if valueRegex.MatchString(report.Envelope) ||
			  valueRegex.MatchString(report.Recipient) ||
			  valueRegex.MatchString(report.Message) {
		this.fillRow(report)
	}
}

func (this *Analyser) fillRow(report *Report) {
	this.table.AddRow(
		report.Envelope,
		report.Recipient,
		report.Code,
		report.Message,
		len(report.CreatedDates),
	)
}

func (this *Analyser) showAggregateTableForType(aggregateReportIds map[string][]int, fields []interface{}) {
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
