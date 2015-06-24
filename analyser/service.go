package analyser

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/AdOnWeb/postmanq/common"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	service = new(Service)

	codeFields = []interface{}{
		"Code",
		"Mails count",
	}
	addressFields = []interface{}{
		"Address",
		"Mails count",
	}
	detailFields = []interface{}{
		"Envelope",
		"Recipient",
		"Code",
		"Message",
		"Sending count",
	}
	codesWriter      = newDetailTableWriter(detailFields)
	recipientsWriter = newDetailTableWriter(detailFields)
	envelopesWriter  = newDetailTableWriter(detailFields)
	allWriter        = newDetailTableWriter(detailFields)
	aggrWriter       = newAggregateTableWriter([]interface {}{
		"Mails count",
		"Code count",
		"Envelopes count",
		"Recipients count",
	})
)

type Service struct {
	mutex                *sync.Mutex
	events               chan *common.SendEvent
	reports              RowWriters
	tableAggregateFields []interface{}
}

func Inst() *Service {
	return service
}

func (s *Service) OnInit(event *common.ApplicationEvent) {
	s.events = make(chan *common.SendEvent)
	s.reports = make(RowWriters)
	s.mutex = new(sync.Mutex)
}

func (s *Service) OnShowReport() {
	for i := 0; i < common.DefaultWorkersCount; i++ {
		go s.receiveMessages()
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		s.findReports(strings.Split(scanner.Text(), " "))
	}
}

func (s *Service) receiveMessages() {
	for event := range s.events {
		s.receiveMessage(event)
	}
}

func (s *Service) receiveMessage(event *common.SendEvent) {
	if event.DefaultPrevented {
		close(s.events)
		s.findReports([]string{})
	} else {
		var message = event.Message
		var report *Report
		s.mutex.Lock()
		reportsLen := len(s.reports)
		for _, rawExistsReport := range s.reports {
			existsReport := rawExistsReport.(*Report)
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
			s.reports[report.Id] = report
		}

		report.CreatedDates = append(report.CreatedDates, message.CreatedDate)
		isValidCode := report.Code > 0
		code := strconv.Itoa(report.Code)

		if isValidCode {
			codesWriter.Add(code, report.Id)
		}
		envelopesWriter.Add(report.Envelope, report.Id)
		recipientsWriter.Add(report.Recipient, report.Id)
		s.mutex.Unlock()
	}
}

func (s *Service) findReports(args []string) {
	var writer TableWriter
	var necessaryAll bool
	var necessaryCode string
	var necessaryEnvelope string
	var necessaryRecipient string
	var necessaryExport bool
	var necessaryOnly bool
	var pattern string
	var limit int
	var offset int

	flagSet := flag.NewFlagSet("service", flag.ContinueOnError)
	flagSet.BoolVar(&necessaryAll, "s", false, "show all reports")
	flagSet.StringVar(&necessaryCode, "c", common.InvalidInputString, "show reports by code")
	flagSet.StringVar(&necessaryEnvelope, "e", common.InvalidInputString, "show reports by envelope")
	flagSet.StringVar(&necessaryRecipient, "r", common.InvalidInputString, "show reports by recipient")
	flagSet.BoolVar(&necessaryExport, "E", false, "export addresses recipients")
	flagSet.BoolVar(&necessaryOnly, "O", false, "show codes or envelopes or recipients without reports")
	flagSet.StringVar(&pattern, "s", common.InvalidInputString, "search by envelope or recipient or mail body")
	flagSet.IntVar(&limit, "l", common.InvalidInputInt, "limit reports")
	flagSet.IntVar(&offset, "o", common.InvalidInputInt, "offset reports")
	err := flagSet.Parse(args)

	if err == nil {
		switch {
		case len(necessaryCode) > 0:
			if necessaryOnly {
				writer = newTypeAggregateTableWriter(codeFields)
				writer.Export(codesWriter)
			} else {
				writer = codesWriter
				writer.SetKeyPattern(necessaryCode)
			}
		case len(necessaryEnvelope) > 0:
			if necessaryOnly {
				writer = newTypeAggregateTableWriter(addressFields)
				writer.Export(envelopesWriter)
			} else {
				writer = envelopesWriter
				writer.SetKeyPattern(necessaryEnvelope)
			}
		case len(necessaryRecipient) > 0:
			if necessaryOnly {
				writer = newTypeAggregateTableWriter(addressFields)
				writer.Export(recipientsWriter)
			} else {
				writer = recipientsWriter
				writer.SetKeyPattern(necessaryRecipient)
			}
		case necessaryAll:
			writer = allWriter
		default:
			s.printUsage(flagSet)
			writer = aggrWriter
		}
	} else {
		s.printUsage(flagSet)
		writer = aggrWriter
	}

	if _, ok := writer.(*AggregateTableWriter); ok {
		writer.SetRows(RowWriters{
			1: AggregateRow{
				len(s.reports),
				len(codesWriter.Ids()),
				len(envelopesWriter.Ids()),
				len(recipientsWriter.Ids()),
			},
		})
	} else {
		writer.SetLimit(limit)
		writer.SetNecessaryExport(necessaryExport)
		writer.SetOffset(offset)
		writer.SetRows(s.reports)
		writer.SetValuePattern(pattern)
	}
	writer.Show()
}

func (s *Service) printUsage(flagSet *flag.FlagSet) {
	fmt.Println()
	fmt.Println("Usage: -acer *|regex [-s] [-E] [-O] [-l] [-o]")
	flagSet.VisitAll(common.PrintUsage)
	fmt.Println("Example:")
	fmt.Println("  -c * -O             show error codes without reports")
	fmt.Println("  -c 550 -l 100       show 100 reports with 550 error")
	fmt.Println("  -c 550 -s gmail.com show reports with 550 error and hostname gmail.com")
	fmt.Println("  -c * -l 100 -o 200  show reports with limit and offset")
}

func (s *Service) Events() chan *common.SendEvent {
	return s.events
}
