package consumer

import (
	"fmt"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
	"github.com/streadway/amqp"
	yaml "gopkg.in/yaml.v2"
	"net/url"
	"sync"
)

var (
	service common.SendingService
	events  = make(chan *common.SendEvent)
)

type Service struct {
	// настройка получателей сообщений
	Configs []*Config `yaml:"consumers"`
	// подключения к очередям
	connections map[string]*amqp.Connection
	// получатели сообщений из очереди
	consumers map[string][]*Consumer
}

func Inst() common.SendingService {
	if service == nil {
		service := new(Service)
		service.connections = make(map[string]*amqp.Connection)
		service.consumers = make(map[string][]*Consumer)
		return service
	}
	return service
}

// инициализирует сервис
func (s *Service) OnInit(event *common.ApplicationEvent) {
	logger.Debug("init consumer service")
	// получаем настройки
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		appsCount := 0
		for _, config := range s.Configs {
			connect, err := amqp.Dial(config.URI)
			if err == nil {
				channel, err := connect.Channel()
				if err == nil {
					apps := make([]*Consumer, len(config.Bindings))
					for i, binding := range config.Bindings {
						binding.init()
						// объявляем очередь
						binding.declare(channel)

						binding.delayedBindings = make(map[common.DelayedBindingType]*Binding)
						// объявляем отложенные очереди
						for delayedBindingType, delayedBinding := range delayedBindings {
							delayedBinding.declareDelayed(binding, channel)
							binding.delayedBindings[delayedBindingType] = delayedBinding
						}

						binding.failureBindings = make(map[FailureBindingType]*Binding)
						for failureBindingType, tplName := range failureBindingTypeTplNames {
							failureBinding := new(Binding)
							failureBinding.Exchange = fmt.Sprintf(tplName, binding.Exchange)
							failureBinding.Queue = fmt.Sprintf(tplName, binding.Queue)
							failureBinding.Type = binding.Type
							failureBinding.declare(channel)
							binding.failureBindings[failureBindingType] = failureBinding
						}

						appsCount++
						app := NewConsumer(appsCount, connect, binding)
						apps[i] = app
					}
					s.connections[config.URI] = connect
					s.consumers[config.URI] = apps
					// слушаем закрытие соединения
					s.reconnect(connect, config)
				} else {
					logger.FailExit("consumer service can't get channel to %s, error - %v", config.URI, err)
				}
			} else {
				logger.FailExit("consumer service can't connect to %s, error - %v", config.URI, err)
			}
		}
	} else {
		logger.FailExit("consumer service can't unmarshal config, error - %v", err)
	}
}

// объявляет слушателя закрытия соединения
func (s *Service) reconnect(connect *amqp.Connection, config *Config) {
	closeErrors := connect.NotifyClose(make(chan *amqp.Error))
	go s.notifyCloseError(config, closeErrors)
}

// слушает закрытие соединения
func (s *Service) notifyCloseError(config *Config, closeErrors chan *amqp.Error) {
	for closeError := range closeErrors {
		logger.Warn("consumer service close connection %s with error - %v, restart...", config.URI, closeError)
		connect, err := amqp.Dial(config.URI)
		if err == nil {
			s.connections[config.URI] = connect
			closeErrors = nil
			if apps, ok := s.consumers[config.URI]; ok {
				for _, app := range apps {
					app.connect = connect
				}
				s.reconnect(connect, config)
			}
			logger.Debug("consumer service reconnect to amqp server %s", config.URI)
		} else {
			logger.Warn("consumer service can't reconnect to amqp server %s with error - %v", config.URI, err)
		}
	}
}

// запускает получателей
func (s *Service) OnRun() {
	logger.Debug("run consumers...")
	for _, apps := range s.consumers {
		s.runConsumers(apps)
	}
}

func (s *Service) runConsumers(apps []*Consumer) {
	for _, app := range apps {
		go app.run()
	}
}

// останавливает получателей
func (s *Service) OnFinish() {
	logger.Debug("stop consumers...")
	for _, connect := range s.connections {
		if connect != nil {
			err := connect.Close()
			if err != nil {
				logger.WarnWithErr(err)
			}
		}
	}
	close(events)
}

func (s *Service) Events() chan *common.SendEvent {
	return events
}

func (s *Service) OnShowReport() {
	waiter := newWaiter()
	group := new(sync.WaitGroup)

	var delta int
	for _, apps := range s.consumers {
		for _, app := range apps {
			delta += app.binding.Handlers
		}
	}
	group.Add(delta)
	for _, apps := range s.consumers {
		go func() {
			for _, app := range apps {
				for i := 0; i < app.binding.Handlers; i++ {
					go app.consumeFailureMessages(group)
				}
			}
		}()
	}
	group.Wait()
	waiter.Stop()

	sendEvent := common.NewSendEvent(nil)
	sendEvent.DefaultPrevented = true
	sendEvent.Iterator.Next().(common.ReportService).Events() <- sendEvent
}

func (s *Service) OnPublish(event *common.ApplicationEvent) {
	group := new(sync.WaitGroup)
	delta := 0
	for uri, apps := range s.consumers {
		var necessaryPublish bool
		if len(event.GetStringArg("host")) > 0 {
			parsedUri, err := url.Parse(uri)
			if err == nil && parsedUri.Host == event.GetStringArg("host") {
				necessaryPublish = true
			} else {
				necessaryPublish = false
			}
		} else {
			necessaryPublish = true
		}
		if necessaryPublish {
			for _, app := range apps {
				delta += app.binding.Handlers
				for i := 0; i < app.binding.Handlers; i++ {
					go app.consumeAndPublishMessages(event, group)
				}
			}
		}
	}
	group.Add(delta)
	group.Wait()
	fmt.Println("done")
	common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
}

// получатель сообщений из очереди
type Config struct {
	URI      string     `yaml:"uri"`
	Bindings []*Binding `yaml:"bindings"`
}
