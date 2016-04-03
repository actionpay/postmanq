package grep

import (
	"bufio"
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

type Config struct {
	// путь до файла с логами
	Output string `yaml:"logOutput"`

	// файл с логами
	logFile *os.File
}

// сервис ищущий сообщения в логе об отправке письма
type Service struct {
	Configs map[string]*Config `yaml:"postmans"`
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
		for _, config := range s.Configs {
			s.init(config)
		}
	} else {
		fmt.Println("grep service can't unmarshal config file")
		common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
	}
}

func (s *Service) init(config *Config) {
	if common.FilenameRegex.MatchString(config.Output) {
		var err error
		config.logFile, err = os.OpenFile(config.Output, os.O_RDONLY, os.ModePerm)
		if err != nil {
			fmt.Println(err)
			common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
		}
	} else {
		fmt.Println("grep service can't open logOutput file")
		common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
	}
}

// ищет логи об отправке письма
func (s *Service) OnGrep(event *common.ApplicationEvent) {
	outs := make(chan string)
	group := new(sync.WaitGroup)
	group.Add(len(s.Configs))
	if event.GetStringArg("envelope") == "" {
		for _, config := range s.Configs {
			go s.grep(event, config, outs, group)
		}
	} else {
		parts := strings.Split(event.GetStringArg("envelope"), "@")
		if len(parts) == 2 {
			if config, ok := s.Configs[parts[1]]; ok {
				go s.grep(event, config, outs, group)
			} else {
				common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
			}
		} else {
			common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
		}
	}

	go func() {
		for out := range outs {
			fmt.Println(out)
		}
	}()
	group.Wait()
	common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
}

func (s *Service) grep(event *common.ApplicationEvent, config *Config, outs chan string, group *sync.WaitGroup) {
	scanner := bufio.NewScanner(config.logFile)
	scanner.Split(bufio.ScanLines)
	lines := make(chan string)

	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	var expr string
	if event.GetStringArg("envelope") == "" {
		expr = fmt.Sprintf("recipient - %s", event.GetStringArg("recipient"))
	} else {
		expr = fmt.Sprintf("envelope - %s, recipient - %s", event.GetStringArg("envelope"), event.GetStringArg("recipient"))
	}

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

	group.Done()
}

// завершает работу сервиса
func (s *Service) OnFinish(event *common.ApplicationEvent) {
	for _, config := range s.Configs {
		config.logFile.Close()
	}
}
