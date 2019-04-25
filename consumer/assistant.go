package consumer

import (
	"encoding/json"
	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
	"github.com/streadway/amqp"
)

type Assistant struct {
	id           int
	connect      *amqp.Connection
	srcBinding   *AssistantBinding
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
		"",                 // consumerTag,
		false,              // noAck
		false,              // exclusive
		false,              // noLocal
		false,              // noWait
		nil,                // arguments
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
			logger.
				By(message.HostnameFrom).
				Info(
					"assistant#%d-%d, handler#%d requeue mail#%d: envelope - %s, recipient - %s to %s",
					a.id,
					message.Id,
					id,
					message.Id,
					message.Envelope,
					message.Recipient,
					message.HostnameFrom,
				)
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
				if err == nil {
					logger.
						By(message.HostnameFrom).
						Info(
							"assistant#%d-%d publish mail#%d to exchange %s",
							a.id,
							message.Id,
							message.Id,
							binding.Exchange,
						)
					delivery.Ack(true)
					return
				} else {
					logger.
						By(message.HostnameFrom).
						Warn(
							"assistant#%d-%d can't publish mail#%d, error - %v",
							a.id,
							message.Id,
							message.Id,
							err,
						)
				}
			} else {
				logger.
					By(message.HostnameFrom).
					Warn(
						"assistant#%d-%d can't publish mail#%d, not found exchange for %s",
						a.id,
						message.Id,
						message.Id,
						message.HostnameFrom,
					)
			}
		} else {
			logger.All().Warn("assistant#%d can't unmarshal delivery body, body should be json, %v given, error - %v", a.id, delivery.Body, err)
		}
		delivery.Nack(true, true)
	}
}
