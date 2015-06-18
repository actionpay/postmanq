package limiter

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
	yaml "gopkg.in/yaml.v2"
	"time"
)

var (
	service *Service
	ticker  *time.Ticker // таймер, работает каждую секунду
	events  = make(chan *common.SendEvent)
)

// сервис ограничений, следит за тем, чтобы почтовым сервисам не отправилось больше писем, чем нужно
type Service struct {
	LimitersCount int               `yaml:"workers"`
	Limits        map[string]*Limit `json:"limits"` // ограничения для почтовых сервисов, в качестве ключа используется домен
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

func (s *Service) OnRun() {
	// сразу запускаем проверку значений ограничений
	go newCleaner()
	for i := 0; i < s.LimitersCount; i++ {
		go newLimiter(i + 1)
	}
}

func (s *Service) Events() chan *common.SendEvent {
	return events
}

func (s *Service) OnFinish() {
	close(events)
}
