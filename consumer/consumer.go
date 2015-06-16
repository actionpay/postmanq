package consumer

import (
	"encoding/json"
	"fmt"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/log"
	"github.com/streadway/amqp"
	"regexp"
	"sync"
)

var (
	resultHandlers = map[common.SendEventResult]func(channel *amqp.Channel, message *common.MailMessage){
		common.ErrorSendEventResult:     (*Consumer).handleErrorSend,
		common.DelaySendEventResult:     (*Consumer).handleDelaySend,
		common.OverlimitSendEventResult: (*Consumer).handleOverlimitSend,
	}
)

// получатель сообщений из очереди
type Consumer struct {
	id         int
	connect    *amqp.Connection
	binding    *Binding
	deliveries <-chan amqp.Delivery
}

// создает нового получателя
func NewConsumer(id int, connect *amqp.Connection, binding *Binding) *Consumer {
	app := new(Consumer)
	app.id = id
	app.connect = connect
	app.binding = binding
	return app
}

// запускает получение сообщений из очереди в заданное количество потоков
func (c *Consumer) run() {
	for i := 0; i < c.binding.Handlers; i++ {
		go c.consume(i)
	}
}

// получает сообщения из очереди
func (c *Consumer) consume(id int) {
	channel, err := c.connect.Channel()
	// выбираем из очереди сообщения с запасом
	// это нужно для того, чтобы после отправки письма новое уже было готово к отправке
	// в тоже время нельзя выбираеть все сообщения из очереди разом, т.к. можно упереться в память
	channel.Qos(2, 0, false)
	deliveries, err := channel.Consume(
		c.binding.Queue, // name
		"",              // consumerTag,
		false,           // noAck
		false,           // exclusive
		false,           // noLocal
		false,           // noWait
		nil,             // arguments
	)
	if err == nil {
		log.Debug("run consumer#%d, handler#%d", c.id, id)
		go c.consumeDeliveries(id, channel, deliveries)
	} else {
		log.Warn("consumer#%d, handler#%d can't consume queue %s", c.id, id, c.binding.Queue)
	}
}

func (c *Consumer) consumeDeliveries(id int, channel *amqp.Channel, deliveries <-chan amqp.Delivery) {
	for delivery := range deliveries {
		message := new(common.MailMessage)
		err := json.Unmarshal(delivery.Body, message)
		if err == nil {
			// инициализируем параметры письма
			message.Init()
			log.Info(
				"consumer#%d, handler#%d send mail#%d: envelope - %s, recipient - %s to mailer",
				c.id,
				id,
				message.Id,
				message.Envelope,
				message.Recipient,
			)

			event := common.NewSendEvent(message)
			log.Debug("consumer#%d create send event for message#%d", c.id, message.Id)
			event.Iterator.Next().(common.SendingService).Events() <- event
			// ждем результата,
			// во время ожидания поток блокируется
			// если этого не сделать, тогда невозможно будет подтвердить получение сообщения из очереди
			if handler, ok := resultHandlers[<-event.Result]; ok {
				handler(c, channel, message)
			}
			message = nil
			event = nil
		} else {
			err = channel.Publish(
				c.binding.failBinding.Exchange,
				c.binding.failBinding.Routing,
				false,
				false,
				amqp.Publishing{
					ContentType:  "text/plain",
					Body:         delivery.Body,
					DeliveryMode: amqp.Transient,
				},
			)
			log.Warn("can't unmarshal delivery body, body should be json, body is %s", string(delivery.Body))
		}
		// всегда подтверждаем получение сообщения
		// даже если во время отправки письма возникли ошибки,
		// мы уже положили это письмо в другую очередь
		delivery.Ack(true)
	}
}

func (c *Consumer) handleErrorSend(channel *amqp.Channel, message *common.MailMessage) {
	// если есть ошибка при отправке, значит мы попали в серый список https://ru.wikipedia.org/wiki/%D0%A1%D0%B5%D1%80%D1%8B%D0%B9_%D1%81%D0%BF%D0%B8%D1%81%D0%BE%D0%BA
	// или получили какую то ошибку от почтового сервиса, что он не может
	// отправить письмо указанному адресату или выполнить какую то команду
	var failBinding *Binding
	// если ошибка связана с невозможностью отправить письмо адресату
	// перекладываем письмо в очередь для плохих писем
	// и пусть отправители сами с ними разбираются
	if message.Error.Code >= 500 && message.Error.Code <= 600 {
		failBinding = c.binding.failBinding
	} else if message.Error.Code == 451 { // мы точно попали в серый список, надо повторить отправку письма попозже
		failBinding = delayedBindings[common.ThirtyMinutesDelayedBinding]
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
					ContentType:  "text/plain",
					Body:         jsonMessage,
					DeliveryMode: amqp.Transient,
				},
			)
			if err == nil {
				log.Debug(
					"reason is %s with code %d, publish failure mail#%d to queue %s",
					message.Error.Message,
					message.Error.Code,
					message.Id,
					failBinding.Queue,
				)
			} else {
				log.Debug(
					"can't publish failure mail#%d with error %s and code %d to queue %s",
					message.Id,
					message.Error.Message,
					message.Error.Code,
					failBinding.Queue,
				)
				log.WarnWithErr(err)
			}
		} else {
			log.WarnWithErr(err)
		}
	}
}

func (c *Consumer) handleDelaySend(channel *amqp.Channel, message *common.MailMessage) {
	bindingType := common.UnknownDelayedBinding
	log.Debug("reason is transfer error, find dlx queue for mail#%d", message.Id)
	log.Debug("old dlx queue type %d for mail#%d", message.BindingType, message.Id)
	// если нам просто не удалось письмо, берем следующую очередь из цепочки
	if chainBinding, ok := bindingsChain[message.BindingType]; ok {
		bindingType = chainBinding
	}
	log.Debug("new dlx queue type %d for mail#%d", bindingType, message.Id)
	c.publishDelayedMessage(channel, bindingType, message)
}

func (c *Consumer) handleOverlimitSend(channel *amqp.Channel, message *common.MailMessage) {
	bindingType := common.UnknownDelayedBinding
	log.Debug("reason is overlimit, find dlx queue for mail#%d", message.Id)
	for i := 0; i < limitBindingsLen; i++ {
		if limitBindings[i] == message.BindingType {
			bindingType = limitBindings[i]
			break
		}
	}
	c.publishDelayedMessage(channel, bindingType, message)
}

func (c *Consumer) publishDelayedMessage(channel *amqp.Channel, bindingType DelayedBindingType, message *MailMessage) {
	log.Debug("dlx queue type %d for mail#%d", bindingType, message.Id)

	// получаем очередь, проверяем, что она реально есть
	// а что? а вдруг нет)
	if delayedBinding, ok := c.binding.delayedBindings[bindingType]; ok {
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
					ContentType:  "text/plain",
					Body:         []byte(jsonMessage),
					DeliveryMode: amqp.Transient,
				},
			)
			if err == nil {
				log.Debug("publish failure mail#%d to queue %s", message.Id, delayedBinding.Queue)
			} else {
				log.Warn("can't publish failure mail#%d to queue %s, error - %v", message.Id, delayedBinding.Queue, err)
			}
		} else {
			log.Warn("can't marshal mail#%d to json", message.Id)
		}
	} else {
		log.Warn("unknow delayed type %v for mail#%d", bindingType, message.Id)
	}
}

func (c *Consumer) consumeFailMessages(group *sync.WaitGroup) {
	channel, err := c.connect.Channel()
	if err == nil {
		for {
			delivery, ok, _ := channel.Get(c.binding.failBinding.Queue, false)
			if ok {
				message := new(MailMessage)
				err = json.Unmarshal(delivery.Body, message)
				if err == nil {
					//					analyser.messages <- message
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

func (c *Consumer) consumeAndPublishMessages(event *common.ApplicationEvent, group *sync.WaitGroup) {
	channel, err := c.connect.Channel()
	if err == nil {
		var envelopeRegex, recipientRegex *regexp.Regexp
		srcBinding := c.findBindingByQueueName(event.GetStringArg("srcQueue"))
		if srcBinding == nil {
			fmt.Println("source queue should be defined")
			common.App.Events() <- NewApplicationEvent(FinishApplicationEventKind)
		}
		destBinding := c.findBindingByQueueName(event.GetStringArg("destQueue"))
		if destBinding == nil {
			fmt.Println("destination queue should be defined")
			common.App.Events() <- NewApplicationEvent(FinishApplicationEventKind)
		}
		if srcBinding == destBinding {
			fmt.Println("source and destination queue should be different")
			common.App.Events() <- NewApplicationEvent(FinishApplicationEventKind)
		}
		if len(event.GetStringArg("envelope")) > 0 {
			envelopeRegex, _ = regexp.Compile(event.GetStringArg("envelope"))
		}
		if len(event.GetStringArg("recipient")) > 0 {
			recipientRegex, _ = regexp.Compile(event.GetStringArg("recipient"))
		}

		publishDeliveries := make([]amqp.Delivery, 0)
		for {
			delivery, ok, _ := channel.Get(srcBinding.Queue, false)
			if ok {
				message := new(MailMessage)
				err = json.Unmarshal(delivery.Body, message)
				if err == nil {
					var necessaryPublish bool
					if (event.GetIntArg("code") > InvalidInputInt && event.GetIntArg("code") == message.Error.Code) ||
						(envelopeRegex != nil && envelopeRegex.MatchString(message.Envelope)) ||
						(recipientRegex != nil && recipientRegex.MatchString(message.Recipient)) ||
						(event.GetIntArg("code") == InvalidInputInt && envelopeRegex == nil && recipientRegex == nil) {
						necessaryPublish = true
					}
					if necessaryPublish {
						fmt.Printf(
							"find mail#%d: envelope - %s, recipient - %s\n",
							message.Id,
							message.Envelope,
							message.Recipient,
						)
						publishDeliveries = append(publishDeliveries, delivery)
					}
				}
			} else {
				break
			}
		}

		for _, delivery := range publishDeliveries {
			err = channel.Publish(
				destBinding.Exchange,
				destBinding.Routing,
				false,
				false,
				amqp.Publishing{
					ContentType:  "text/plain",
					Body:         delivery.Body,
					DeliveryMode: amqp.Transient,
				},
			)
			if err == nil {
				delivery.Ack(true)
			} else {
				delivery.Nack(true, true)
			}
		}
		group.Done()
	} else {
		log.WarnWithErr(err)
	}
}

func (c *Consumer) findBindingByQueueName(queueName string) *Binding {
	if c.binding.Queue == queueName {
		return c.binding
	} else if c.binding.failBinding.Queue == queueName {
		return c.binding.failBinding
	} else {
		var binding *Binding
		for _, delayedBinding := range c.binding.delayedBindings {
			if delayedBinding.Queue == queueName {
				binding = delayedBinding
				break
			}
		}
		return binding
	}
}
