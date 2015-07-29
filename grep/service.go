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

	var mailsCount int
	for _, line := range lines {
		if strings.Contains(line, expr) {
			mailsCount++
		}
	}
	wg := new(sync.WaitGroup)
	wg.Add(mailsCount)
	for i, line := range lines {
		if strings.Contains(line, expr) {
			results := mailIdRegex.FindStringSubmatch(line)
			if len(results) == 3 {
				go s.print(results[1], lines[i:], wg)
			}
		}
	}
	wg.Wait()

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
