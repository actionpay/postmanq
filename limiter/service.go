package limiter

import (
	"github.com/actionpay/postmanq/common"
	"github.com/actionpay/postmanq/logger"
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
	// количество горутин проверяющих количество отправленных писем
	LimitersCount int `yaml:"workers"`

	// ограничения для почтовых сервисов, в качестве ключа используется домен
	Limits map[string]*Limit `yaml:"limits"`
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
	logger.Debug("init limits...")
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		// инициализируем ограничения
		for host, limit := range s.Limits {
			limit.init()
			logger.Debug("create limit for %s with type %v and duration %v", host, limit.bindingType, limit.duration)
		}
		if s.LimitersCount == 0 {
			s.LimitersCount = common.DefaultWorkersCount
		}
	} else {
		logger.FailExitWithErr(err)
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
