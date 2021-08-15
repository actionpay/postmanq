package limiter

import (
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
)

var (
	// сервис ограничений
	service *Service

	// таймер, работает каждую секунду
	ticker *time.Ticker

	// канал для приема событий отправки писем
	events       = make(chan *common.SendEvent)
	eventsClosed bool
)

// Service сервис ограничений, следит за тем, чтобы почтовым сервисам не отправилось больше писем, чем нужно
type Service struct {
	// LimitersCount количество горутин проверяющих количество отправленных писем
	LimitersCount int `yaml:"workers"`

	Configs map[string]*Config `yaml:"postmans"`
}

// Inst создает сервис ограничений
func Inst() common.SendingService {
	if service == nil {
		service = new(Service)
		ticker = time.NewTicker(time.Second)
	}
	return service
}

// OnInit инициализирует сервис
func (s *Service) OnInit(event *common.ApplicationEvent) {
	logger.All().Debug("init limits...")
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		for name, config := range s.Configs {
			s.init(config, name)
		}
		if s.LimitersCount == 0 {
			s.LimitersCount = common.DefaultWorkersCount
		}
	} else {
		logger.All().FailExitErr(err)
	}
}

func (s *Service) init(conf *Config, hostname string) {
	// инициализируем ограничения
	for host, limit := range conf.Limits {
		if limit.duration == 0 {
			delete(conf.Limits, host)
			logger.By(hostname).Warn("wrong limits settings")
		}
		limit.init()
		logger.By(hostname).Debug("create limit for %s with type %v and duration %v", host, limit.bindingType, limit.duration)
	}
}

// OnRun запускает проверку ограничений и очистку значений лимитов
func (s *Service) OnRun() {
	// сразу запускаем проверку значений ограничений
	go newCleaner()
	for i := 0; i < s.LimitersCount; i++ {
		go newLimiter(i + 1)
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

// OnFinish завершает работу сервиса соединений
func (s *Service) OnFinish() {
	if !eventsClosed {
		eventsClosed = true
		close(events)
	}
}

func (s Service) getLimit(hostnameFrom, hostnameTo string) *Limit {
	if config, ok := service.Configs[hostnameFrom]; ok {
		if limit, has := config.Limits[hostnameTo]; has {
			return limit
		} else {
			return nil
		}
	} else {
		return nil
	}
}

type Config struct {
	// ограничения для почтовых сервисов, в качестве ключа используется домен
	Limits map[string]*Limit `yaml:"limits"`
}
