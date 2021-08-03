package guardian

import (
	"gopkg.in/yaml.v3"

	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
)

var (
	// сервис блокирующий отправку писем
	service *Service

	// канал для приема событий отправки писем
	events       = make(chan *common.SendEvent)
	eventsClosed bool
)

// сервис блокирующий отправку писем
type Service struct {
	// количество горутин блокирующий отправку писем к почтовым сервисам
	GuardiansCount int `yaml:"workers"`

	Configs map[string]*Config `yaml:"postmans"`
}

// создает новый сервис блокировок
func Inst() common.SendingService {
	if service == nil {
		service = new(Service)
	}
	return service
}

// инициализирует сервис блокировок
func (s *Service) OnInit(event *common.ApplicationEvent) {
	logger.All().Debug("init guardians...")
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		if s.GuardiansCount == 0 {
			s.GuardiansCount = common.DefaultWorkersCount
		}
	} else {
		logger.All().FailExitErr(err)
	}
}

// запускает горутины
func (s *Service) OnRun() {
	for i := 0; i < s.GuardiansCount; i++ {
		go newGuardian(i + 1)
	}
}

// Event send event
func (s *Service) Event(ev *common.SendEvent) bool {
	if eventsClosed {
		return false
	}

	events <- ev
	return true
}

// завершает работу сервиса соединений
func (s *Service) OnFinish() {
	if !eventsClosed {
		eventsClosed = true
		close(events)
	}
}

func (s Service) getExcludes(hostname string) []string {
	if conf, ok := s.Configs[hostname]; ok {
		return conf.Excludes
	} else {
		return common.EmptyStrSlice
	}
}

type Config struct {
	// хосты, на которую блокируется отправка писем
	Excludes []string `yaml:"exclude"`
}
