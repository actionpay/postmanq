package grep

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/actionpay/postmanq/common"
	yaml "gopkg.in/yaml.v2"
	"os"
	"regexp"
	"strings"
	"sync"
)

var (
	// сервис ищущий сообщения в логе об отправке письма
	service *Service

	// регулярное выражение, по которому находим начало отправки
	mailIdRegex = regexp.MustCompile(`mail#((\d)+)+`)
)

// сервис ищущий сообщения в логе об отправке письма
type Service struct {
	// путь до файла с логами
	Output string `yaml:"logOutput"`

	// файл с логами
	logFile *os.File
}

// создает новый сервис поиска по логам
func Inst() common.GrepService {
	if service == nil {
		service = new(Service)
	}
	return service
}

// инициализирует сервис
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

// ищет логи об отправке письма
func (s *Service) OnGrep(event *common.ApplicationEvent) {
	scanner := bufio.NewScanner(s.logFile)
	scanner.Split(bufio.ScanLines)
	lines := make(chan string)
	outs := make(chan string)

	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	var expr string
	hasEnvelope := event.GetStringArg("envelope") == ""
	if hasEnvelope {
		expr = fmt.Sprintf("envelope - %s, recipient - %s to mailer", event.GetStringArg("envelope"), event.GetStringArg("recipient"))
	} else {
		expr = fmt.Sprintf("recipient - %s to mailer", event.GetStringArg("recipient"))
	}

	go func() {
		var successExpr, failExpr, failPubExpr, delayExpr, limitExpr string
		var mailId string
		for line := range lines {
			if mailId == "" {
				if strings.Contains(line, expr) {
					results := mailIdRegex.FindStringSubmatch(line)
					if len(results) == 3 {
						mailId = results[1]

						successExpr = fmt.Sprintf("%s success send", mailId)
						failExpr = fmt.Sprintf("%s publish failure mail to queue", mailId)
						failPubExpr = fmt.Sprintf("%s can't publish failure mail to queue", mailId)
						delayExpr = fmt.Sprintf("%s detect old dlx queue", mailId)
						limitExpr = fmt.Sprintf("%s detect overlimit", mailId)

						outs <- line
					}
				}
			} else {
				if strings.Contains(line, mailId) {
					outs <- line
				}
				if strings.Contains(line, successExpr) ||
					strings.Contains(line, failExpr) ||
					strings.Contains(line, failPubExpr) ||
					strings.Contains(line, delayExpr) ||
					strings.Contains(line, limitExpr) {
					mailId = ""
				}
			}
		}
		close(lines)
	}()

	for out := range outs {
		fmt.Println(out)
	}
	close(outs)
	common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
}

// выводит логи в терминал
func (s *Service) print(mailId string, lines []string, wg *sync.WaitGroup) {
	out := new(bytes.Buffer)

	for _, line := range lines {
		if strings.Contains(line, mailId) {
			out.WriteString(line)
			out.WriteString("\n")
		}
	}

	fmt.Println(out.String())
	wg.Done()
}

// завершает работу сервиса
func (s *Service) OnFinish(event *common.ApplicationEvent) {
	s.logFile.Close()
}
