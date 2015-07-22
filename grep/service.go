package grep

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/AdOnWeb/postmanq/common"
	yaml "gopkg.in/yaml.v2"
	"os"
	"regexp"
	"strings"
	"sync"
)

const (
	eol = "\n"
)

var (
	service          *Service
	mailIdRegex      = regexp.MustCompile(`mail#(\d)+`)
	consumerIdRegex  = regexp.MustCompile(`consumer#(\d)+`)
	limiterIdRegex   = regexp.MustCompile(`limiter#(\d)+`)
	connectorIdRegex = regexp.MustCompile(`connector#(\d)+`)
	mailerIdRegex    = regexp.MustCompile(`mailer#(\d)+`)
)

type Service struct {
	Output  string `yaml:"logOutput"` // название вывода логов
	logFile *os.File
}

func Inst() common.GrepService {
	if service == nil {
		service = new(Service)
	}
	return service
}

func (s *Service) OnInit(event *common.ApplicationEvent) {
	var err error
	err = yaml.Unmarshal(event.Data, s)
	if err == nil {
		if common.FilenameRegex.MatchString(s.Output) {
			s.logFile, err = os.OpenFile(s.Output, os.O_RDONLY, os.ModePerm)
			if err != nil {
				fmt.Println(err)
				common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
			}
		} else {
			fmt.Println("logOutput should be a file")
			common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
		}
	} else {
		fmt.Println("service can't unmarshal config file")
		common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
	}
}

func (s *Service) OnGrep(event *common.ApplicationEvent) {
	scanner := bufio.NewScanner(s.logFile)
	scanner.Split(bufio.ScanLines)
	lines := make([]string, 0)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	linesLen := len(lines)
	if event.GetIntArg("numberLines") > common.InvalidInputInt && event.GetIntArg("numberLines") < linesLen {
		lines = lines[linesLen-event.GetIntArg("numberLines"):]
	}

	var expr string
	hasEnvelope := len(event.GetStringArg("envelope")) > 0
	if hasEnvelope {
		expr = fmt.Sprintf("envelope - %s, recipient - %s to mailer", event.GetStringArg("envelope"), event.GetStringArg("recipient"))
	} else {
		expr = fmt.Sprintf("recipient - %s to mailer", event.GetStringArg("recipient"))
	}

	mailRegex, err := regexp.Compile(expr)
	if err == nil {
		wg := new(sync.WaitGroup)
		var mailsCount int
		for i, line := range lines {
			if mailRegex.MatchString(line) {
				mailsCount++
				go s.grep(mailIdRegex.FindString(line), lines[i:], wg)
			}
		}
		wg.Add(mailsCount)
		wg.Wait()
	} else {
		if hasEnvelope {
			fmt.Println("invalid recipient")
		} else {
			fmt.Println("invalid recipient or envelope")
		}
	}

	common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
}

func (s *Service) grep(mailId string, lines []string, wg *sync.WaitGroup) {
	var consumerId, limiterId, connectorId, mailerId string
	mailRegex, _ := regexp.Compile(mailId)
	out := new(bytes.Buffer)
	out.WriteString(eol)

	for _, line := range lines {
		if mailRegex.MatchString(line) {
			out.WriteString(line)
			out.WriteString(eol)

			if consumerIdRegex.MatchString(line) {
				consumerId = consumerIdRegex.FindString(line)
			}

			if limiterIdRegex.MatchString(line) {
				consumerId = common.InvalidInputString
				limiterId = limiterIdRegex.FindString(line)
			}

			if connectorIdRegex.MatchString(line) {
				limiterId = common.InvalidInputString
				connectorId = connectorIdRegex.FindString(line)
			}

			if mailerIdRegex.MatchString(line) {
				connectorId = common.InvalidInputString
				mailerId = mailerIdRegex.FindString(line)
			}

			if strings.Contains(line, "sending error") || strings.Contains(line, "success send") {
				mailerId = common.InvalidInputString
				break
			}
		} else {
			if len(consumerId) > 0 && strings.Contains(line, consumerId) {
				out.WriteString(line)
				out.WriteString(eol)
			}
			if len(limiterId) > 0 && strings.Contains(line, limiterId) {
				out.WriteString(line)
				out.WriteString(eol)
			}
			if len(connectorId) > 0 && strings.Contains(line, connectorId) {
				out.WriteString(line)
				out.WriteString(eol)
			}
			if len(mailerId) > 0 && strings.Contains(line, mailerId) {
				out.WriteString(line)
				out.WriteString(eol)
			}
		}
	}

	fmt.Print(out.String())
	wg.Done()
}

func (s *Service) OnFinish(event *common.ApplicationEvent) {
	s.logFile.Close()
}
