package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"github.com/streadway/amqp"
	"encoding/json"
	"fmt"
	"time"
	"encoding/base64"
)

type DelayedBindingType int

const (
	DELAYED_BINDING_MINUTE      DelayedBindingType = iota + 1
	DELAYED_BINDING_TEN_MINUTES
	DELAYED_BINDING_HOUR
	DELAYED_BINDING_SIX_HOURS
)

var (
	delayedBindings = map[DelayedBindingType]*Binding {
		DELAYED_BINDING_MINUTE     : &Binding{Name: "%s.dlx.minute", QueueArgs: amqp.Table{"x-message-ttl": int64(time.Minute.Seconds()) * 1000}},
//		DELAYED_BINDING_TEN_MINUTES: &Binding{Name: "%s.dlx.ten.minutes", QueueArgs: amqp.Table{"x-message-ttl": int64((time.Minute * 10).Seconds())}},
//		DELAYED_BINDING_HOUR       : &Binding{Name: "%s.dlx.hour", QueueArgs: amqp.Table{"x-message-ttl": int64(time.Hour.Seconds())}},
//		DELAYED_BINDING_SIX_HOURS  : &Binding{Name: "%s.dlx.six.hours", QueueArgs: amqp.Table{"x-message-ttl": int64((time.Hour * 6).Seconds())}},
	}
)

type Consumer struct {
	AppsConfigs []*ConsumerApplicationConfig `yaml:"consumers"`
	apps        []*ConsumerApplication
	connect     *amqp.Connection
}

func NewConsumer() *Consumer {
	consumer := new(Consumer)
	consumer.apps = make([]*ConsumerApplication, 0)
	return consumer
}

func (this *Consumer) OnRegister() {}

func (this *Consumer) OnInit(event *InitEvent) {
	Debug("init consumers apps...")
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		appsCount := 0
		for _, appConfig := range this.AppsConfigs {
			Debug("connect to %s", appConfig.URI)
			this.connect, err = amqp.Dial(appConfig.URI)
			if err == nil {
				Debug("got connection to %s, getting channel", appConfig.URI)
				channel, err := this.connect.Channel()
				if err == nil {
					Debug("got channel for %s", appConfig.URI)
					for _, binding := range appConfig.Bindings {
						if len(binding.Type) == 0 {
							binding.Type = EXCHANGE_TYPE_FANOUT
						}
						if len(binding.Name) > 0 {
							binding.Exchange = binding.Name
							binding.Queue = binding.Name
						}

						this.declare(channel, binding)
						binding.delayedBindings = make(map[DelayedBindingType]*Binding)
						for delayedBindingType, delayedBinding := range delayedBindings {
							delayedBinding.Exchange = fmt.Sprintf(delayedBinding.Name, binding.Exchange)
							delayedBinding.Queue = fmt.Sprintf(delayedBinding.Name, binding.Queue)
							delayedBinding.QueueArgs["x-dead-letter-exchange"] = binding.Exchange
							delayedBinding.Type = EXCHANGE_TYPE_DIRECT
							this.declare(channel, delayedBinding)
							binding.delayedBindings[delayedBindingType] = delayedBinding
						}

						appsCount++
						app := NewConsumerApplication()
						app.id = appsCount
						app.binding = binding
						app.mailersCount = event.MailersCount
						this.apps = append(this.apps, app)
						Info("create consumer app#%d", app.id)
					}
				} else {
					FailExitWithErr(err)
				}
				this.reconnect(appConfig)
			} else {
				FailExitWithErr(err)
			}
		}
	} else {
		FailExitWithErr(err)
	}
}

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
		FailExitWithErr(err)
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
		FailExitWithErr(err)
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
		FailExitWithErr(err)
	}
}

func (this *Consumer) reconnect(appConfig *ConsumerApplicationConfig) {
	closeErrors := this.connect.NotifyClose(make(chan *amqp.Error))
	go this.notifyCloseError(appConfig, closeErrors)
}

func (this *Consumer) notifyCloseError(appConfig *ConsumerApplicationConfig, closeErrors chan *amqp.Error) {
	var err error
	for closeError := range closeErrors {
		WarnWithErr(closeError)
		Info("close connection %s, restart...", appConfig.URI)
		this.finishApps()
		this.connect, err = amqp.Dial(appConfig.URI)
		if err == nil {
			closeErrors = nil
			this.reconnect(appConfig)
			this.OnRun()
		} else {
			FailExitWithErr(err)
		}
	}
}

func (this *Consumer) OnRun() {
	Info("run consumers apps...")
	for _, app := range this.apps {
		app.connect = this.connect
		go app.Run()
	}
}

func (this *Consumer) OnFinish(event *FinishEvent) {
	Info("stop consumers apps...")
	this.finishApps()
	event.Group.Done()
}

func (this *Consumer) finishApps() {
	for _, app := range this.apps {
		app.Close()
	}
}

type ConsumerApplicationConfig struct {
	URI      string     `yaml:"uri"`
	Bindings []*Binding `yaml:"bindings"`
}

type Binding struct {
	Name            string       					`yaml:"name"`
	Exchange        string       					`yaml:"exchange"`
	ExchangeArgs    amqp.Table
	Queue           string       					`yaml:"queue"`
	QueueArgs       amqp.Table
	Type            ExchangeType 					`yaml:"type"`
	Routing         string       					`yaml:"routing"`
	Handlers        int                             `yaml:"handlers"`
	delayedBindings map[DelayedBindingType]*Binding
}

type ExchangeType string

const (
	EXCHANGE_TYPE_DIRECT ExchangeType = "direct"
	EXCHANGE_TYPE_FANOUT              = "fanout"
	EXCHANGE_TYPE_TOPIC               = "topic"
)

type ConsumerApplication struct {
	id           int
	connect      *amqp.Connection
	binding      *Binding
	deliveries   <- chan amqp.Delivery
	mailersCount int
}

func NewConsumerApplication() *ConsumerApplication {
	return new(ConsumerApplication)
}

func (this *ConsumerApplication) Run() {
	if this.binding.Handlers == 0 {
		this.binding.Handlers = 1
	}

	for i := 0; i < this.binding.Handlers; i++ {
		go this.consume(i)
	}
}

func (this *ConsumerApplication) consume(id int) {
	channel, err := this.connect.Channel()
	channel.Qos(this.mailersCount / this.binding.Handlers * 3, 0, false)
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
		Info("run consumer app#%d, handler#%d", this.id, id)
		go func() {
			for delivery := range deliveries {
				body, err := base64.StdEncoding.DecodeString(string(delivery.Body))
				if err == nil {
					message := new(MailMessage)
					err = json.Unmarshal(body, message)
					if err == nil {
						message.Init()
						Info("consumer app#%d, handler#%d send mail#%d to mailer", this.id, id, message.Id)
						SendMail(message)
						done := <- message.Done
						delivery.Ack(true)
						if !done {
							message.AttemptCount++
							dlxType := DelayedBindingType(message.AttemptCount)
							if delayedBinding, ok := this.binding.delayedBindings[dlxType]; ok {
								jsonMessage, err := json.Marshal(message)
								if err == nil {
									encodedMessage := base64.StdEncoding.EncodeToString(jsonMessage)
									err = channel.Publish(
										delayedBinding.Exchange,
										delayedBinding.Routing,
										false,
										false,
										amqp.Publishing{
											ContentType:     "text/plain",
											Body:            []byte(encodedMessage),
											DeliveryMode:    amqp.Transient,
										},
									)
								} else {
									WarnWithErr(err)
								}
							} else {
								Warn("unknow delayed type %v", dlxType)
							}
						}
						message = nil
					} else {
						Warn("can't unmarshal delivery body, body should be json, body is %s", string(body))
					}
				} else {
					Warn("can't decode delivery body, body should be base64 decoded, body is %s", string(delivery.Body))
				}
			}
		}()
	} else {
		FailExitWithErr(err)
	}
}

func (this *ConsumerApplication) Close() {
	if this.connect != nil {
		this.connect.Close()
	}
}

