package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"github.com/streadway/amqp"
	"encoding/json"
	"fmt"
	"time"
	"sync"
)

const (
	FAIL_BINDING = "%s.fail"
)

// тип отложенной очереди
type DelayedBindingType int

const (
	DELAYED_BINDING_UNKNOWN     DelayedBindingType = iota
	DELAYED_BINDING_SECOND
	DELAYED_BINDING_THIRTY_SECOND
	DELAYED_BINDING_MINUTE
	DELAYED_BINDING_FIVE_MINUTES
	DELAYED_BINDING_TEN_MINUTES
	DELAYED_BINDING_TWENTY_MINUTES
	DELAYED_BINDING_THIRTY_MINUTES
	DELAYED_BINDING_FORTY_MINUTES
	DELAYED_BINDING_FYFTY_MINUTES
	DELAYED_BINDING_HOUR
	DELAYED_BINDING_SIX_HOURS
	DELAYED_BINDING_DAY
)

var (
	// отложенные очереди вообще
	// письмо отправляется повторно при возниковении ошибки во время отправки
	delayedBindings = map[DelayedBindingType]*Binding {
		DELAYED_BINDING_SECOND         : &Binding{Name: "%s.dlx.second",         QueueArgs: amqp.Table{"x-message-ttl": int64(time.Second.Seconds()) * 1000}},
		DELAYED_BINDING_THIRTY_SECOND  : &Binding{Name: "%s.dlx.thirty.second",  QueueArgs: amqp.Table{"x-message-ttl": int64(time.Second.Seconds() * 30) * 1000}},
		DELAYED_BINDING_MINUTE         : &Binding{Name: "%s.dlx.minute",         QueueArgs: amqp.Table{"x-message-ttl": int64(time.Minute.Seconds()) * 1000}},
		DELAYED_BINDING_FIVE_MINUTES   : &Binding{Name: "%s.dlx.five.minutes",   QueueArgs: amqp.Table{"x-message-ttl": int64((time.Minute * 5).Seconds()) * 1000}},
		DELAYED_BINDING_TEN_MINUTES    : &Binding{Name: "%s.dlx.ten.minutes",    QueueArgs: amqp.Table{"x-message-ttl": int64((time.Minute * 10).Seconds()) * 1000}},
		DELAYED_BINDING_TWENTY_MINUTES : &Binding{Name: "%s.dlx.twenty.minutes", QueueArgs: amqp.Table{"x-message-ttl": int64((time.Minute * 20).Seconds()) * 1000}},
		DELAYED_BINDING_THIRTY_MINUTES : &Binding{Name: "%s.dlx.thirty.minutes", QueueArgs: amqp.Table{"x-message-ttl": int64((time.Minute * 30).Seconds()) * 1000}},
		DELAYED_BINDING_FORTY_MINUTES  : &Binding{Name: "%s.dlx.forty.minutes",  QueueArgs: amqp.Table{"x-message-ttl": int64((time.Minute * 40).Seconds()) * 1000}},
		DELAYED_BINDING_FYFTY_MINUTES  : &Binding{Name: "%s.dlx.fyfty.minutes",  QueueArgs: amqp.Table{"x-message-ttl": int64((time.Minute * 50).Seconds()) * 1000}},
		DELAYED_BINDING_HOUR           : &Binding{Name: "%s.dlx.hour",           QueueArgs: amqp.Table{"x-message-ttl": int64(time.Hour.Seconds()) * 1000}},
		DELAYED_BINDING_SIX_HOURS      : &Binding{Name: "%s.dlx.six.hours",      QueueArgs: amqp.Table{"x-message-ttl": int64((time.Hour * 6).Seconds()) * 1000}},
		DELAYED_BINDING_DAY            : &Binding{Name: "%s.dlx.day",            QueueArgs: amqp.Table{"x-message-ttl": int64((time.Hour * 24).Seconds()) * 1000}},
	}

	// отложенные очереди для лимитов
	limitBindings = []DelayedBindingType {
		DELAYED_BINDING_SECOND,
		DELAYED_BINDING_MINUTE,
		DELAYED_BINDING_HOUR,
		DELAYED_BINDING_DAY,
	}

	limitBindingsLen = len(limitBindings)

	// цепочка очередей, используемых для повторной отправки писем
	// в качестве ключа используется текущий тип очереди, а в качестве значения следующий
	bindingsChain = map[DelayedBindingType]DelayedBindingType {
		DELAYED_BINDING_UNKNOWN       : DELAYED_BINDING_SECOND,
		DELAYED_BINDING_SECOND        : DELAYED_BINDING_THIRTY_SECOND,
		DELAYED_BINDING_THIRTY_SECOND : DELAYED_BINDING_MINUTE,
		DELAYED_BINDING_MINUTE        : DELAYED_BINDING_FIVE_MINUTES,
		DELAYED_BINDING_FIVE_MINUTES  : DELAYED_BINDING_TEN_MINUTES,
		DELAYED_BINDING_TEN_MINUTES   : DELAYED_BINDING_TWENTY_MINUTES,
		DELAYED_BINDING_TWENTY_MINUTES: DELAYED_BINDING_THIRTY_MINUTES,
		DELAYED_BINDING_THIRTY_MINUTES: DELAYED_BINDING_FORTY_MINUTES,
		DELAYED_BINDING_FORTY_MINUTES : DELAYED_BINDING_FYFTY_MINUTES,
		DELAYED_BINDING_FYFTY_MINUTES : DELAYED_BINDING_HOUR,
		DELAYED_BINDING_HOUR          : DELAYED_BINDING_SIX_HOURS,
		DELAYED_BINDING_SIX_HOURS     : DELAYED_BINDING_UNKNOWN,
	}
)

// сервис, отвечающий за объявление очередей и получение писем из очереди
type Consumer struct {
	AppsConfigs []*ConsumerApplicationConfig      `yaml:"consumers"` // настройка получателей сообщений
	connections map[string]*amqp.Connection                          // подключения к очередям
	appsByURI   map[string][]*ConsumerApplication                    // получатели сообщений из очереди
}

// создает новый сервис
func ConsumerOnce() *Consumer {
	consumer := new(Consumer)
	consumer.connections = make(map[string]*amqp.Connection)
	consumer.appsByURI = make(map[string][]*ConsumerApplication)
	return consumer
}

// инициализирует сервис
func (this *Consumer) OnInit(event *InitEvent) {
	Debug("init consumers apps...")
	// получаем настройки
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		appsCount := 0
		for _, appConfig := range this.AppsConfigs {
			Debug("connect to %s", appConfig.URI)
			connect, err := amqp.Dial(appConfig.URI)
			if err == nil {
				Debug("got connection to %s, getting channel", appConfig.URI)
				channel, err := connect.Channel()
				if err == nil {
					Debug("got channel for %s", appConfig.URI)
					apps := make([]*ConsumerApplication, len(appConfig.Bindings))
					for i, binding := range appConfig.Bindings {
						if len(binding.Type) == 0 {
							binding.Type = EXCHANGE_TYPE_FANOUT
						}
						if len(binding.Name) > 0 {
							binding.Exchange = binding.Name
							binding.Queue = binding.Name
						}
						// по умолчанию очередь разбирают столько рутин сколько ядер
						if binding.Handlers == 0 {
							binding.Handlers = defaultWorkersCount
						}

						// объявляем очередь
						this.declare(channel, binding)
						binding.delayedBindings = make(map[DelayedBindingType]*Binding)
						// объявляем отложенные очереди
						for delayedBindingType, delayedBinding := range delayedBindings {
							delayedBinding.Exchange = fmt.Sprintf(delayedBinding.Name, binding.Exchange)
							delayedBinding.Queue = fmt.Sprintf(delayedBinding.Name, binding.Queue)
							delayedBinding.QueueArgs["x-dead-letter-exchange"] = binding.Exchange
							delayedBinding.Type = binding.Type
							this.declare(channel, delayedBinding)
							binding.delayedBindings[delayedBindingType] = delayedBinding
						}
						// создаем очередь для 500-ых ошибок
						failBinding := new(Binding)
						failBinding.Exchange = fmt.Sprintf(FAIL_BINDING, binding.Exchange)
						failBinding.Queue = fmt.Sprintf(FAIL_BINDING, binding.Queue)
						failBinding.Type = binding.Type
						this.declare(channel, failBinding)
						binding.failBinding = failBinding

						appsCount++
						app := NewConsumerApplication(appsCount, connect, binding, event.MailersCount)
						apps[i] = app
						Debug("create consumer app#%d", app.id)
					}
					this.connections[appConfig.URI] = connect
					this.appsByURI[appConfig.URI] = apps
					// слушаем закрытие соединения
					this.reconnect(connect, appConfig)
				} else {
					FailExit("consumer can't get channel to %s, error - %v", appConfig.URI, err)
				}
			} else {
				FailExit("consumer can't connect to %s, error - %v", appConfig.URI, err)
			}
		}
	} else {
		FailExit("consumer can't unmarshal config, error - %v", err)
	}
}

// объявляет точку обмена и очередь и связывает их
func (this *Consumer) declare(channel *amqp.Channel, binding *Binding) {
	Debug("declaring exchange - %s", binding.Exchange)
	err := channel.ExchangeDeclare(
		binding.Exchange,      // name of the exchange
		string(binding.Type),  // type
		true,                  // durable
		false,                 // delete when complete
		false,                 // internal
		false,                 // noWait
		binding.ExchangeArgs,  // arguments
	)
	if err == nil {
		Debug("declared exchange - %s", binding.Exchange)
	} else {
		FailExit("consumer can't declare exchange %s, error - %v", binding.Exchange, err)
	}

	Debug("declaring queue - %s", binding.Queue)
	_, err = channel.QueueDeclare(
		binding.Queue,     // name of the queue
		true,              // durable
		false,             // delete when usused
		false,             // exclusive
		false,             // noWait
		binding.QueueArgs, // arguments
	)
	if err == nil {
		Debug("declared queue - %s", binding.Queue)
	} else {
		FailExit("consumer can't declare queue %s, error - %v", binding.Queue, err)
	}

	Debug("binding to exchange key - \"%s\"", binding.Routing)
	err = channel.QueueBind(
		binding.Queue,    // name of the queue
		binding.Routing,  // bindingKey
		binding.Exchange, // sourceExchange
		false,            // noWait
		nil,              // arguments
	)
	if err == nil {
		Debug("queue %s bind to exchange %s", binding.Queue, binding.Exchange)
	} else {
		FailExit("consumer can't bind queue %s to exchange %s, error - %v", binding.Queue, binding.Exchange, err)
	}
}

// объявляет слушателя закрытия соединения
func (this *Consumer) reconnect(connect *amqp.Connection, appConfig *ConsumerApplicationConfig) {
	closeErrors := connect.NotifyClose(make(chan *amqp.Error))
	go this.notifyCloseError(appConfig, closeErrors)
}

// слушает закрытие соединения
func (this *Consumer) notifyCloseError(appConfig *ConsumerApplicationConfig, closeErrors chan *amqp.Error) {
	for closeError := range closeErrors {
		Warn("close connection %s with error - %v, restart...", appConfig.URI, closeError)
		connect, err := amqp.Dial(appConfig.URI)
		if err == nil {
			this.connections[appConfig.URI] = connect
			closeErrors = nil
			if apps, ok := this.appsByURI[appConfig.URI]; ok {
				for _, app := range apps {
					app.connect = connect
				}
				this.reconnect(connect, appConfig)
			}
			Debug("consumer reconnected to amqp server %s", appConfig.URI)
		} else {
			Warn("consumer can't reconnected to amqp server %s with error - %v", appConfig.URI, err)
		}
	}
}

// запускает получателей
func (this *Consumer) OnRun() {
	Debug("run consumers apps...")
	for _, apps := range this.appsByURI {
		this.runApps(apps)
	}
}

func (this *Consumer) runApps(apps []*ConsumerApplication) {
	for _, app := range apps {
		go app.Run()
	}
}

// останавливает получателей
func (this *Consumer) OnFinish() {
	Debug("stop consumers apps...")
	for _, connect := range this.connections {
		if connect != nil {
			err := connect.Close()
			if err != nil {
				WarnWithErr(err)
			}
		}
	}
}

func (this *Consumer) OnShowReport() {
	ticker := time.NewTicker(time.Millisecond * 250)
	go this.showWaiting(ticker)
	group := new(sync.WaitGroup)
	delta := 0
	for _, apps := range this.appsByURI {
		for _, app := range apps {
			delta += app.binding.Handlers
			for i := 0; i < app.binding.Handlers; i++ {
				go app.consumeFailMessages(group)
			}
		}
	}
	group.Add(delta)
	group.Wait()
	ticker.Stop()
	analyser.findReports([]string{})
}

func (this *Consumer) showWaiting(ticker *time.Ticker) {
	commas := []string{
		".  ",
		" . ",
		"  .",
	}
	i := 0
	for {
		<- ticker.C
		fmt.Printf("\rgetting fail messages, please wait%s", commas[i])
		if i == 2 {
			i = 0
		} else {
			i++
		}
	}
}

func (this *Consumer) OnPublish() {
	group := new(sync.WaitGroup)
	delta := 0
	for _, apps := range this.appsByURI {
		for _, app := range apps {
			delta += app.binding.Handlers
			for i := 0; i < app.binding.Handlers; i++ {
				go app.consumeAndPublishFailMessages(group)
			}
		}
	}
	group.Add(delta)
	group.Wait()
}

// получатель сообщений из очереди
type ConsumerApplicationConfig struct {
	URI      string     `yaml:"uri"`
	Bindings []*Binding `yaml:"bindings"`
}

// связка точки обмена и очереди
type Binding struct {
	Name            string       					`yaml:"name"`     // имя точки обмена и очереди
	Exchange        string       					`yaml:"exchange"` // имя точки обмена
	ExchangeArgs    amqp.Table                                        // аргументы точки обмена
	Queue           string       					`yaml:"queue"`    // имя очереди
	QueueArgs       amqp.Table                                        // аргументы очереди
	Type            ExchangeType 					`yaml:"type"`     // тип точки обмена
	Routing         string       					`yaml:"routing"`  // ключ маршрутизации
	Handlers        int                             `yaml:"workers"`  // количество потоков, разбирающих очередь
	delayedBindings map[DelayedBindingType]*Binding                   // отложенные очереди
	failBinding     *Binding                                          // очередь для 500-ых ошибок
}

// тип точки обмена
type ExchangeType string

const (
	EXCHANGE_TYPE_DIRECT ExchangeType = "direct"
	EXCHANGE_TYPE_FANOUT              = "fanout"
	EXCHANGE_TYPE_TOPIC               = "topic"
)

// получатель сообщений из очереди
type ConsumerApplication struct {
	id           int
	connect      *amqp.Connection
	binding      *Binding
	deliveries   <- chan amqp.Delivery
	mailersCount int
}

// создает нового получателя
func NewConsumerApplication(id int, connect *amqp.Connection, binding *Binding, mailersCount int) *ConsumerApplication {
	app := new(ConsumerApplication)
	app.id = id
	app.connect = connect
	app.binding = binding
	app.mailersCount = mailersCount
	return app
}

// запускает получение сообщений из очереди в заданное количество потоков
func (this *ConsumerApplication) Run() {
	for i := 0; i < this.binding.Handlers; i++ {
		go this.consume(i)
	}
}

// получает сообщения из очереди
func (this *ConsumerApplication) consume(id int) {
	channel, err := this.connect.Channel()
	// выбираем из очереди сообщения с запасом
	// это нужно для того, чтобы после отправки письма новое уже было готово к отправке
	// в тоже время нельзя выбираеть все сообщения из очереди разом, т.к. можно упереться в память
	channel.Qos(2, 0, false)
	deliveries, err := channel.Consume(
		this.binding.Queue,    // name
		"",                    // consumerTag,
		false,                 // noAck
		false,                 // exclusive
		false,                 // noLocal
		false,                 // noWait
		nil,                   // arguments
	)
	if err == nil {
		Debug("run consumer app#%d, handler#%d", this.id, id)
		go func() {
			for delivery := range deliveries {
				message := new(MailMessage)
				err = json.Unmarshal(delivery.Body, message)
				if err == nil {
					// инициализируем параметры письма
					message.Init()
					Info(
						"consumer app#%d, handler#%d send mail#%d: envelope - %s, recipient - %s to mailer",
						this.id,
						id,
						message.Id,
						message.Envelope,
						message.Recipient,
					)

					event := NewSendEvent(message)
					limiter.events <- event
					// ждем результата,
					// во время ожидания поток блокируется
					// если этого не сделать, тогда невозможно будет подтвердить получение сообщения из очереди
					switch <- event.Result {
					case SEND_EVENT_RESULT_ERROR:
						// если есть ошибка при отправке, значит мы попали в серый список https://ru.wikipedia.org/wiki/%D0%A1%D0%B5%D1%80%D1%8B%D0%B9_%D1%81%D0%BF%D0%B8%D1%81%D0%BE%D0%BA
						// или получили какую то ошибку от почтового сервиса, что он не может
						// отправить письмо указанному адресату или выполнить какую то команду
						var failBinding *Binding
						// если ошибка связана с невозможностью отправить письмо адресату
						// перекладываем письмо в очередь для плохих писем
						// и пусть отправители сами с ними разбираются
						if message.Error.Code >= 500 && message.Error.Code <= 600 {
							failBinding = this.binding.failBinding
						} else if message.Error.Code == 451 { // мы точно попали в серый список, надо повторить отправку письма попозже
//						} else { // мы точно попали в серый список, надо повторить отправку письма попозже
							failBinding = delayedBindings[DELAYED_BINDING_THIRTY_MINUTES]
						}
						// если очередь для ошибок нашлась
						if failBinding != nil {
							jsonMessage, err := json.Marshal(message)
							if err == nil {
								// кладем в очередь
								err = channel.Publish(
									failBinding.Exchange,
									failBinding.Routing,
									false,
									false,
									amqp.Publishing{
										ContentType : "text/plain",
										Body        : jsonMessage,
										DeliveryMode: amqp.Transient,
									},
								)
								if err == nil {
									Debug(
										"reason is %s with code %d, publish fail mail#%d to queue %s",
										message.Error.Message,
										message.Error.Code,
										message.Id,
										this.binding.failBinding.Queue,
									)
								} else {
									Debug(
										"can't publish fail mail#%d with error %s and code %d to queue %s",
										message.Id,
										message.Error.Message,
										message.Error.Code,
										failBinding.Queue,
									)
									WarnWithErr(err)
								}
							} else {
								WarnWithErr(err)
							}
						}
					case SEND_EVENT_RESULT_DELAY:
						bindingType := DELAYED_BINDING_UNKNOWN
						Debug("reason is transfer error, find dlx queue for mail#%d", message.Id)
						Debug("old dlx queue type %d for mail#%d", message.BindingType, message.Id)
						// если нам просто не удалось письмо, берем следующую очередь из цепочки
						if chainBinding, ok := bindingsChain[message.BindingType]; ok {
							bindingType = chainBinding
						}
						this.publishDelayedMessage(channel, bindingType, message)
					case SEND_EVENT_RESULT_OVERLIMIT:
						bindingType := DELAYED_BINDING_UNKNOWN
						Debug("reason is overlimit, find dlx queue for mail#%d", message.Id)
						for i := 0;i < limitBindingsLen;i++ {
							if limitBindings[i] == message.BindingType {
								bindingType = limitBindings[i]
								break
							}
						}
						this.publishDelayedMessage(channel, bindingType, message)
					}

					// всегда подтверждаем получение сообщения
					// даже если во время отправки письма возникли ошибки,
					// мы уже положили это письмо в другую очередь
					message = nil
					event = nil
					delivery.Ack(true)
				} else {
					err = channel.Publish(
						this.binding.failBinding.Exchange,
						this.binding.failBinding.Routing,
						false,
						false,
						amqp.Publishing{
							ContentType : "text/plain",
							Body        : delivery.Body,
							DeliveryMode: amqp.Transient,
						},
					)
					Warn("can't unmarshal delivery body, body should be json, body is %s", string(delivery.Body))
					delivery.Ack(true)
				}
			}
		}()
	} else {
		Warn("consumer app#%d, handler#%d can't consume queue %s", this.id, id, this.binding.Queue)
	}
}

func (this *ConsumerApplication) publishDelayedMessage(channel *amqp.Channel, bindingType DelayedBindingType, message *MailMessage) {
	Debug("dlx queue type %d for mail#%d", bindingType, message.Id)

	// получаем очередь, проверяем, что она реально есть
	// а что? а вдруг нет)
	if delayedBinding, ok := this.binding.delayedBindings[bindingType]; ok {
		message.BindingType = bindingType
		jsonMessage, err := json.Marshal(message)
		if err == nil {
			// кладем в очередь
			err = channel.Publish(
				delayedBinding.Exchange,
				delayedBinding.Routing,
				false,
				false,
				amqp.Publishing{
				ContentType : "text/plain",
				Body        : []byte(jsonMessage),
				DeliveryMode: amqp.Transient,
			},
			)
			if err == nil {
				Debug("publish fail mail#%d to queue %s", message.Id, delayedBinding.Queue)
			} else {
				Warn("can't publish fail mail#%d to queue %s, error - %v", message.Id, delayedBinding.Queue, err)
			}
		} else {
			Warn("can't marshal mail#%d to json", message.Id)
		}
	} else {
		Warn("unknow delayed type %v for mail#%d", bindingType, message.Id)
	}
}

func (this *ConsumerApplication) consumeFailMessages(group *sync.WaitGroup) {
	channel, err := this.connect.Channel()
	if err == nil {
		for {
			delivery, ok, _ := channel.Get(this.binding.failBinding.Queue, false)
			if ok {
				message := new(MailMessage)
				err = json.Unmarshal(delivery.Body, message)
				if err == nil {
					analyser.messages <- message
				}
			} else {
				break
			}
		}
		group.Done()
	} else {
		WarnWithErr(err)
	}
}

func (this *ConsumerApplication) consumeAndPublishFailMessages(group *sync.WaitGroup) {
	channel, err := this.connect.Channel()
	if err == nil {
		for {
			delivery, ok, _ := channel.Get(this.binding.failBinding.Queue, false)
			if ok {
				message := new(MailMessage)
				err = json.Unmarshal(delivery.Body, message)
				if err == nil && message.Error.Code == 511 {
					err = channel.Publish(
						this.binding.Exchange,
						this.binding.Routing,
						false,
						false,
						amqp.Publishing{
							ContentType : "text/plain",
							Body        : delivery.Body,
							DeliveryMode: amqp.Transient,
						},
					)
					if err == nil {
						delivery.Ack(true)
					}
				}
			} else {
				break
			}
		}
		group.Done()
	} else {
		WarnWithErr(err)
	}
}
