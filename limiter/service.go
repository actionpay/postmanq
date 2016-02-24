package limiter

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
	yaml "gopkg.in/yaml.v2"
	"time"
)

var (
	// сервис ограничений
	service *Service

	// таймер, работает каждую секунду
	ticker *time.Ticker

	// канал для приема событий отправки писем
	events = make(chan *common.SendEvent)
)

// сервис ограничений, следит за тем, чтобы почтовым сервисам не отправилось больше писем, чем нужно
type Service struct {
	Config
	// количество горутин проверяющих количество отправленных писем
	LimitersCount int `yaml:"workers"`

	Configs map[string]Config `yaml:"domains"`
}

// создает сервис ограничений
func Inst() common.SendingService {
	if service == nil {
		service = new(Service)
		service.Limits = make(map[string]*Limit)
		ticker = time.NewTicker(time.Second)
	}
	return service
}

// инициализирует сервис
func (s *Service) OnInit(event *common.ApplicationEvent) {
	logger.All().Debug("init limits...")
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		s.init(&s.Config, common.AllDomains)
		for name, config := range s.Configs {
			s.init(&config, name)
		}
		if s.LimitersCount == 0 {
			s.LimitersCount = common.DefaultWorkersCount
		}
	} else {
		logger.All().FailExitWithErr(err)
	}
}

func (s *Service) init(conf *Config, hostname string) {
	// инициализируем ограничения
	for host, limit := range conf.Limits {
		limit.init()
		logger.By(hostname).Debug("create limit for %s with type %v and duration %v", host, limit.bindingType, limit.duration)
	}
}

// запускает проверку ограничений и очистку значений лимитов
func (s *Service) OnRun() {
	// сразу запускаем проверку значений ограничений
	go newCleaner()
	for i := 0; i < s.LimitersCount; i++ {
		go newLimiter(i + 1)
	}
}

// канал для приема событий отправки писем
func (s *Service) Events() chan *common.SendEvent {
	return events
}

// завершает работу сервиса соединений
func (s *Service) OnFinish() {
	close(events)
}

func (s Service) getLimit(hostnameFrom, hostnameTo string) (Limit, bool) {
	if config, ok := service.Configs[hostnameFrom]; ok {
		return config.Limits[hostnameTo]
	} else {
		return service.Limits[hostnameTo]
	}
}

type Config struct {
	// ограничения для почтовых сервисов, в качестве ключа используется домен
	Limits map[string]*Limit `yaml:"limits"`
}
