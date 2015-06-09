package limit

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/log"
	yaml "gopkg.in/yaml.v2"
	"time"
)

var (
	service *Service
	ticker  *time.Ticker // таймер, работает каждую секунду
	events  chan *common.SendEvent
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
		events = make(chan *SendEvent)
	}
	return service
}

// инициализирует сервис
func (s *Service) OnInit(event *ApplicationEvent) {
	log.Debug("init limits...")
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		// инициализируем ограничения
		for host, limit := range s.Limits {
			limit.init()
			log.Debug("create limit for %s with type %v and duration %v", host, limit.bindingType, limit.duration)
		}
		if s.LimitersCount == 0 {
			s.LimitersCount = common.DefaultWorkersCount
		}
	} else {
		log.FailExitWithErr(err)
	}
}

func (s *Service) OnRun() {
	// сразу запускаем проверку значений ограничений
	go new(Cleaner).clean()
	for i := 0; i < s.LimitersCount; i++ {
		go newLimiter(i + 1).run()
	}
}

func (s *Service) Events() chan *SendEvent {
	return events
}

func (s *Service) OnFinish() {
	close(events)
}
