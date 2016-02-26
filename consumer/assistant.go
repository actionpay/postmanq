package consumer

import (
	"github.com/streadway/amqp"
	"github.com/AdOnWeb/postmanq/logger"
	"github.com/AdOnWeb/postmanq/common"
	"encoding/json"
)

type Assistant struct {
	id           int
	connect      *amqp.Connection
	srcBinding   *Binding
	destBindings map[string]*Binding
}

func (a *Assistant) run() {
	for i := 0; i < a.srcBinding.Handlers; i++ {
		go a.consume(i)
	}
}

func (a *Assistant) consume(id int) {
	channel, err := a.connect.Channel()
	// выбираем из очереди сообщения с запасом
	// это нужно для того, чтобы после отправки письма новое уже было готово к отправке
	// в тоже время нельзя выбираеть все сообщения из очереди разом, т.к. можно упереться в память
	channel.Qos(a.srcBinding.PrefetchCount, 0, false)
	deliveries, err := channel.Consume(
		a.srcBinding.Queue, // name
		"", // consumerTag,
		false, // noAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil, // arguments
	)
	if err == nil {
		go a.publish(id, channel, deliveries)
	} else {
		logger.All().Warn("assistant#%d, handler#%d can't consume queue %s", a.id, id, a.srcBinding.Queue)
	}
}

func (a *Assistant) publish(id int, channel *amqp.Channel, deliveries <-chan amqp.Delivery) {
	for delivery := range deliveries {
		message := new(common.MailMessage)
		err := json.Unmarshal(delivery.Body, message)
		if err == nil {
			message.Init()
			if binding, ok := a.destBindings[message.HostnameFrom]; ok {
				err = channel.Publish(
					binding.Exchange,
					binding.Routing,
					false,
					false,
					amqp.Publishing{
						ContentType:  "text/plain",
						Body:         delivery.Body,
						DeliveryMode: amqp.Transient,
					},
				)
			} else {

			}
		} else {

		}
	}
}
