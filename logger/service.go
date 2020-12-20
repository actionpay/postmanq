package logger

import (
	"strconv"
	"strings"

	"github.com/Halfi/postmanq/common"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

var (
	service *Service
	logger  zerolog.Logger
)

type Config struct {
	// название уровня логирования, устанавливается в конфиге
	LevelName string `yaml:"logLevel"`

	// название вывода логов
	Output string `yaml:"logOutput"`
}

type Service struct {
	Config

	Configs map[string]*Config `yaml:"postmans"`
}

func Inst() common.SendingService {
	if service == nil {
		service = new(Service)
		service.Config = Config{
			LevelName: "debug",
			Output:    "stdout",
		}
	}
	return service
}

// инициализирует сервис логирования
func (s *Service) OnInit(event *common.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		level, err := zerolog.ParseLevel(s.LevelName)
		if err != nil {
			All().FailExitWithErr(err)
			return
		}

		zerolog.SetGlobalLevel(level)
		zerolog.TimestampFieldName = "t"
		zerolog.LevelFieldName = "l"
		zerolog.MessageFieldName = "m"
		zerolog.CallerFieldName = "c"

		// default CallerMarshalFunc adds full path
		// callerMarshalFunc adds only last 2 parts
		zerolog.CallerMarshalFunc = callerMarshalFunc
		log.Logger = log.With().Caller().Logger()

		logger = log.With().Logger()

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
func (s *Service) OnFinish() {}

func callerMarshalFunc(file string, line int) string {
	parts := strings.Split(file, "/")
	if len(parts) > 1 {
		return strings.Join(parts[len(parts)-2:], "/") + ":" + strconv.Itoa(line)
	}
	return file + ":" + strconv.Itoa(line)
}
