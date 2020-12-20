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
	for i := 0; i < a.srcBinding.Binding.Handlers; i++ {
		go a.consume(i)
	}
}

func (a *Assistant) consume(id int) {
	channel, err := a.connect.Channel()
	if err != nil {
		logger.All().Warn("consumer#%d, handler#%d can't get channel %s", a.id, id, a.srcBinding.Binding.Queue)
		return
	}

	// выбираем из очереди сообщения с запасом
	// это нужно для того, чтобы после отправки письма новое уже было готово к отправке
	// в тоже время нельзя выбираеть все сообщения из очереди разом, т.к. можно упереться в память
	err = channel.Qos(a.srcBinding.Binding.PrefetchCount, 0, false)
	if err != nil {
		logger.All().Warn("consumer#%d, handler#%d can't set qos %s", a.id, id, a.srcBinding.Binding.Queue)
		return
	}

	deliveries, err := channel.Consume(
		a.srcBinding.Binding.Queue, // name
		"",                         // consumerTag,
		false,                      // noAck
		false,                      // exclusive
		false,                      // noLocal
		false,                      // noWait
		nil,                        // arguments
	)
	if err == nil {
		go a.publish(id, channel, deliveries)
	} else {
		logger.All().Warn("assistant#%d, handler#%d can't consume queue %s", a.id, id, a.srcBinding.Binding.Queue)
	}
}

func (a *Assistant) publish(id int, channel *amqp.Channel, deliveries <-chan amqp.Delivery) {
	for delivery := range deliveries {
		message := new(common.MailMessage)
		err := json.Unmarshal(delivery.Body, message)
		if err != nil {
			logger.All().WarnWithErr(err, "assistant#%d can't unmarshal delivery body, body should be json, %v given", a.id, delivery.Body)
			continue
		}

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
			if err != nil {
				logger.By(message.HostnameFrom).WarnWithErr(err, "assistant#%d-%d can't publish mail#%d", a.id, message.Id, message.Id)
				continue
			}

			logger.By(message.HostnameFrom).
				Info("assistant#%d-%d publish mail#%d to exchange %s", a.id, message.Id, message.Id, binding.Exchange)

			if err := delivery.Ack(true); err != nil {
				logger.All().
					WarnWithErr(err, "assistant#%d-%d can't ack mail#%d to exchange %s", a.id, message.Id, message.Id, binding.Exchange)
			}

			return
		}

		logger.By(message.HostnameFrom).
			Warn("assistant#%d-%d can't publish mail#%d, not found exchange for %s", a.id, message.Id, message.Id, message.HostnameFrom)

		if err := delivery.Nack(true, true); err != nil {
			logger.All().WarnWithErr(err, "assistant#%d-%d can't nack email message", a.id, message.Id)
		}
	}
}
