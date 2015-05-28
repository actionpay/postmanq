package log

import (
	yaml "gopkg.in/yaml.v2"
	"github.com/AdOnWeb/postmanq/types"
	"regexp"
)

// уровень логирования
type Level int

// уровни логирования
const(
	DebugLevel   Level = iota
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
	filenameRegex = regexp.MustCompile(`[^\\/]+\.[^\\/]+`)
	// названия уровней логирования, используется непосредственно в момент создания записи в лог
	logLevelById = map[Level]string{
		DebugLevel  : DebugLevelName,
		InfoLevel   : InfoLevelName,
		WarningLevel: WarningLevelName,
		ErrorLevel  : ErrorLevelName,
	}
	// уровни логирования по названию, используется для удобной инициализации сервиса логирования
	logLevelByName = map[string]Level{
		DebugLevelName  : DebugLevel,
		InfoLevelName   : InfoLevel,
		WarningLevelName: WarningLevel,
		ErrorLevelName  : ErrorLevel,
	}
	messages = make(chan *Message)
	writers = make(Writers, types.DefaultWorkersCount)
	level = WarningLevel
	service *Service
)

// запись логирования
type Message struct {
	Message string        // сообщение для лога, может содержать параметры
	Level   Level         // уровень логирования записи, необходим для отсечения лишних записей
	Args    []interface{} // аргументы для параметров сообщения
}

// созадние новой записи логирования
func NewMessage(level Level, message string, args ...interface{}) *Message {
	logMessage := new(Message)
	logMessage.Level = level
	logMessage.Message = message
	logMessage.Args = args
	return logMessage
}

// сервис логирования
type Service struct {
	LevelName string        `yaml:"logLevel"`  // название уровня логирования, устанавливается в конфиге
	Output    string        `yaml:"logOutput"` // название вывода логов
	level     Level                            // уровень логов, ниже этого уровня логи писаться не будут
	writer    Writer                           // куда пишем логи stdout или файл
	messages  chan *Message                    // канал логирования
}

// создает новый сервис логирования
func Once() *Service {
	if service == nil {
		service = new(Service)
		// запускаем запись логов в отдельном потоке
		writers.init(service)
		writers.write()
	}
	return service
}

// инициализирует сервис логирования
func (s *Service) OnInit(event *types.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		// устанавливаем уровень логирования
		if existsLevel, ok := logLevelByName[s.LevelName]; ok {
			level = existsLevel
		}
		// заново инициализируем вывод для логов
		writers.init(service)
		writers.write()
	} else {
		FailExitWithErr(err)
	}
}

func (s *Service) OnRun() {}

// закрывает канал логирования
func (s *Service) OnFinish() {
	close(messages)
}
