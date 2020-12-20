package eventlogger

import (
	"fmt"
	"github.com/Halfi/postmanq/common"
	yaml "gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"time"
)

// уровень логирования
type Level int

// уровни логирования
const (
	DebugLevel Level = iota
	InfoLevel
	WarningLevel
	ErrorLevel
)

// название уровней логирования
const (
	DebugLevelName   = "debug"
	InfoLevelName    = "info"
	WarningLevelName = "warning"
	ErrorLevelName   = "error"
)

var (
	// названия уровней логирования, используется непосредственно в момент создания записи в лог
	logLevelById = map[Level]string{
		DebugLevel:   DebugLevelName,
		InfoLevel:    InfoLevelName,
		WarningLevel: WarningLevelName,
		ErrorLevel:   ErrorLevelName,
	}
	// уровни логирования по названию, используется для удобной инициализации сервиса логирования
	logLevelByName = map[string]Level{
		DebugLevelName:   DebugLevel,
		InfoLevelName:    InfoLevel,
		WarningLevelName: WarningLevel,
		ErrorLevelName:   ErrorLevel,
	}
	// канал логирования
	messages         = make(chan *Message)
	messagesChanPool = make(map[string]chan *Message)
	service          *Service
)

// сервис логирования
type Service struct {
	Config

	Configs map[string]*Config `yaml:"postmans"`
}

// создает новый сервис логирования
func Inst() common.SendingService {
	if service == nil {
		service = new(Service)
		for i := 0; i < common.DefaultWorkersCount; i++ {
			go service.listenCommonMessags()
		}
		service.Configs = map[string]*Config{
			"default": &Config{
				LevelName: "debug",
				Output:    "stdout",
			},
		}
		service.init()
	}
	return service
}

func (s *Service) listenCommonMessags() {
	for message := range messages {
		if message.Hostname == common.AllDomains {
			for _, messagesChan := range messagesChanPool {
				messagesChan <- message
			}
		} else {
			if messagesChan, ok := messagesChanPool[message.Hostname]; ok {
				messagesChan <- message
			}
		}
	}
}

func (s *Service) init() {
	for name, config := range s.Configs {
		messagesChan := make(chan *Message)
		var level Level
		if existsLevel, ok := logLevelByName[config.LevelName]; ok {
			level = existsLevel
		} else {
			level = DebugLevel
		}
		for i := 0; i < common.DefaultWorkersCount; i++ {
			var writer Writer
			if common.FilenameRegex.MatchString(config.Output) { // проверяем получили ли из настроек имя файла
				// получаем директорию, в которой лежит файл
				dir := filepath.Dir(config.Output)
				// смотрим, что она реально существует
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					All().FailExit("directory %s is not exists", dir)
				} else {
					writer = &FileWriter{
						filename: config.Output,
						level:    level,
					}
				}
			} else if len(config.Output) == 0 || config.Output == "stdout" {
				writer = &StdoutWriter{
					level: level,
				}
			}
			if writer != nil {
				go s.listenMessages(messagesChan, writer)
			}
		}
		messagesChanPool[name] = messagesChan
	}
}

// подписывает авторов на получение сообщений для логирования
func (s *Service) listenMessages(messagesChan chan *Message, writer Writer) {
	for message := range messagesChan {
		if writer.getLevel() <= message.Level {
			s.writeMessage(writer, message)
		}
	}
}

// пишет сообщение в лог
func (s *Service) writeMessage(writer Writer, message *Message) {
	writer.writeString(
		fmt.Sprintf(
			"PostmanQ | %v | %s: %s\n",
			time.Now().Format("2006-01-02 15:04:05"),
			logLevelById[message.Level],
			fmt.Sprintf(message.Message, message.Args...),
		),
	)
}

// инициализирует сервис логирования
func (s *Service) OnInit(event *common.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		s.OnFinish()
		// заново инициализируем вывод для логов
		delete(service.Configs, "default")
		messages = make(chan *Message)
		messagesChanPool = make(map[string]chan *Message)
		for i := 0; i < common.DefaultWorkersCount; i++ {
			go s.listenCommonMessags()
		}
		s.init()
	} else {
		All().FailExitWithErr(err)
	}
}

// ничего не делает, авторы логов уже пишут
func (s *Service) OnRun() {}

// не учавствеут в отправке писем
func (s *Service) Events() chan *common.SendEvent {
	return nil
}

// закрывает канал логирования
func (s *Service) OnFinish() {
	if messagesChanPool == nil {
		return
	}

	for name, messagesChan := range messagesChanPool {
		close(messagesChan)
		delete(messagesChanPool, name)
	}
	messagesChanPool = nil
	go func() {
		<- time.After(time.Second)
		close(messages)
	}()
}

type Config struct {
	// название уровня логирования, устанавливается в конфиге
	LevelName string `yaml:"logLevel"`

	// название вывода логов
	Output string `yaml:"logOutput"`
}
