package consumer

import (
	"fmt"
	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
	"github.com/streadway/amqp"
	yaml "gopkg.in/yaml.v2"
	"net/url"
	"sync"
)

var (
	// сервис получения сообщений
	service common.SendingService

	// канал для получения событий
	events = make(chan *common.SendEvent)
)

// сервис получения сообщений
type Service struct {
	// настройка получателей сообщений
	Configs []*Config `yaml:"consumers"`

	// подключения к очередям
	connections map[string]*amqp.Connection

	// получатели сообщений из очереди
	consumers map[string][]*Consumer

	assistants map[string][]*Assistant
}

// создает новый сервис получения сообщений
func Inst() common.SendingService {
	if service == nil {
		service := new(Service)
		service.connections = make(map[string]*amqp.Connection)
		service.consumers = make(map[string][]*Consumer)
		service.assistants = make(map[string][]*Assistant)
		return service
	}
	return service
}

// инициализирует сервис
func (s *Service) OnInit(event *common.ApplicationEvent) {
	logger.All().Debug("init consumer service")
	// получаем настройки
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		consumersCount := 0
		assistantsCount := 0
		for _, config := range s.Configs {
			connect, err := amqp.Dial(config.URI)
			if err == nil {
				channel, err := connect.Channel()
				if err == nil {
					consumers := make([]*Consumer, len(config.Bindings))
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

						consumersCount++
						consumers[i] = NewConsumer(consumersCount, connect, binding)
					}
					assistants := make([]*Assistant, len(config.Assistants))
					for i, assistantBinding := range config.Assistants {
						assistantBinding.init()
						// объявляем очередь
						assistantBinding.declare(channel)

						destBindings := make(map[string]*Binding)
						for domain, exchange := range assistantBinding.Dest {
							for _, consumer := range consumers {
								if consumer.binding.Exchange == exchange {
									destBindings[domain] = consumer.binding
									break
								}
							}
						}

						assistantsCount++
						assistants[i] = &Assistant{
							id:           assistantsCount,
							connect:      connect,
							srcBinding:   assistantBinding,
							destBindings: destBindings,
						}
					}

					s.connections[config.URI] = connect
					s.consumers[config.URI] = consumers
					s.assistants[config.URI] = assistants
					// слушаем закрытие соединения
					s.reconnect(connect, config)
				} else {
					logger.All().FailExit("consumer service can't get channel to %s, error - %v", config.URI, err)
				}
			} else {
				logger.All().FailExit("consumer service can't connect to %s, error - %v", config.URI, err)
			}
		}
	} else {
		logger.All().FailExit("consumer service can't unmarshal config, error - %v", err)
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
		logger.All().Warn("consumer service close connection %s with error - %v, restart...", config.URI, closeError)
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
			logger.All().Debug("consumer service reconnect to amqp server %s", config.URI)
		} else {
			logger.All().Warn("consumer service can't reconnect to amqp server %s with error - %v", config.URI, err)
		}
	}
}

// запускает сервис
func (s *Service) OnRun() {
	logger.All().Debug("run consumers...")
	for _, consumers := range s.consumers {
		s.runConsumers(consumers)
	}
	for _, assistants := range s.assistants {
		s.runAssistants(assistants)
	}
}

// запускает получателей
func (s *Service) runConsumers(consumers []*Consumer) {
	for _, consumer := range consumers {
		go consumer.run()
	}
}

func (s *Service) runAssistants(assistants []*Assistant) {
	for _, assistant := range assistants {
		go assistant.run()
	}
}

// останавливает получателей
func (s *Service) OnFinish() {
	logger.All().Debug("stop consumers...")
	for _, connect := range s.connections {
		if connect != nil {
			err := connect.Close()
			if err != nil {
				logger.All().WarnWithErr(err)
			}
		}
	}
	close(events)
}

// канал для приема событий отправки писем
func (s *Service) Events() chan *common.SendEvent {
	return events
}

// запускает получение сообщений с ошибками и пересылает их другому сервису
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
	sendEvent.Iterator.Next().(common.ReportService).Events() <- sendEvent
}

// перекладывает сообщения из очереди в очередь
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
	URI        string              `yaml:"uri"`
	Assistants []*AssistantBinding `yaml:"assistants"`
	Bindings   []*Binding          `yaml:"bindings"`
}
