package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"github.com/streadway/amqp"
	"encoding/json"
)

type Consumer struct {
	AppsConfigs  []*ConsumerApplicationConfig `yaml:"consumers"`
	apps         []*ConsumerApplication
}

func NewConsumer() *Consumer {
	consumer := new(Consumer)
	consumer.apps = make([]*ConsumerApplication, 0)
	return consumer
}

func (this *Consumer) OnRegister(event *RegisterEvent) {
	event.Group.Done()
}

func (this *Consumer) OnInit(event *InitEvent) {
	Info("init consumers...")
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		for _, appConfig := range this.AppsConfigs {
			Info("connect to %s", appConfig.URI)
			connect, err := amqp.Dial(appConfig.URI)
			if err == nil {
				Info("got connection to %s, getting channel", appConfig.URI)
				channel, err := connect.Channel()
				if err == nil {
					Info("got channel for %s", appConfig.URI)
					for _, binding := range appConfig.Bindings {
						if len(binding.Type) == 0 {
							binding.Type = EXCHANGE_TYPE_DIRECT
						}
						if len(binding.Name) > 0 {
							binding.Exchange = binding.Name
							binding.Queue = binding.Name
						}

						Info("declaring exchange - %s", binding.Exchange)
						err = channel.ExchangeDeclare(
							binding.Exchange,      // name of the exchange
							string(binding.Type),  // type
							true,                  // durable
							false,                 // delete when complete
							false,                 // internal
							false,                 // noWait
							nil,                   // arguments
						)
						if err == nil {
							Info("declared exchange - %s", binding.Exchange)
						} else {
							FailExitWithErr(err)
						}

						Info("declaring queue - %s", binding.Queue)
						_, err := channel.QueueDeclare(
							binding.Queue, // name of the queue
							true,          // durable
							false,         // delete when usused
							false,         // exclusive
							false,         // noWait
							nil,           // arguments
						)
						if err == nil {
							Info("declared queue - %s", binding.Queue)
						} else {
							FailExitWithErr(err)
						}

						Info("binding to exchange key - \"%s\"", binding.Routing)
						err = channel.QueueBind(
							binding.Queue,    // name of the queue
							binding.Routing,  // bindingKey
							binding.Exchange, // sourceExchange
							false,            // noWait
							nil,              // arguments
						)
						if err == nil {
							Info("queue %s bind to exchange %s", binding.Queue, binding.Exchange)
							app := NewConsumerApplication()
							app.connect = connect
							app.channel = channel
							app.binding = binding
							this.apps = append(this.apps, app)
						} else {
							FailExitWithErr(err)
						}
					}
				} else {
					FailExitWithErr(err)
				}
			} else {
				FailExitWithErr(err)
			}
		}
		event.Group.Done()
	} else {
		FailExitWithErr(err)
	}
	Info("consumers complete")
}

func (this *Consumer) OnRun() {
	for _, app := range this.apps {
		go app.Run()
	}
}

func (this *Consumer) OnFinish(event *FinishEvent) {
	for _, app := range this.apps {
		app.Close()
	}
	event.Group.Done()
}

type ConsumerApplicationConfig struct {
	URI string          `yaml:"uri"`
	Bindings []*Binding `yaml:"bindings"`
}

type Binding struct {
	Name     string       `yaml:"name"`
	Exchange string       `yaml:"exchange"`
	Queue    string       `yaml:"queue"`
	Type     ExchangeType `yaml:"type"`
	Routing  string       `yaml:"routing"`
	Handlers int          `yaml:"handlers"`
}

type ExchangeType string

const (
	EXCHANGE_TYPE_DIRECT ExchangeType = "direct"
	EXCHANGE_TYPE_FANOUT              = "fanout"
	EXCHANGE_TYPE_TOPIC               = "topic"
)

type ConsumerApplication struct {
	connect    *amqp.Connection
	channel    *amqp.Channel
	binding    *Binding
	deliveries <- chan amqp.Delivery
}

func NewConsumerApplication() *ConsumerApplication {
	return new(ConsumerApplication)
}

func (this *ConsumerApplication) Run() {
	deliveries, err := this.channel.Consume(
		this.binding.Exchange, // name
		"",                    // consumerTag,
		false,                 // noAck
		false,                 // exclusive
		false,                 // noLocal
		false,                 // noWait
		nil,                   // arguments
	)
	if err == nil {
		if this.binding.Handlers == 0 {
			this.binding.Handlers = 1
		}
		for i := 0; i < this.binding.Handlers; i++ {
			go this.consume(i, deliveries)
		}
	} else {
		FailExitWithErr(err)
	}
}

func (this *ConsumerApplication) consume(id int, deliveries <- chan amqp.Delivery) {
	for delivery := range deliveries {
		mail := new(MailMessage)
		err := json.Unmarshal(delivery.Body, mail)
		if err == nil {
			Info("consumer - %d, delivery - %s", id, delivery.Body)
			SendMail(mail)
		} else {
			SendMail(new(MailMessage))
//			Warn("mail has invalid format - %s", delivery.Body)
		}
	}
}

func (this *ConsumerApplication) Close() {
	if this.channel != nil {
		this.channel.Close()
	}
	if this.connect != nil {
		this.connect.Close()
	}
}

