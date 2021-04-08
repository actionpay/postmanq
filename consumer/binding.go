package consumer

import (
	"fmt"
	"github.com/sergw3x/postmanq/common"
	"github.com/sergw3x/postmanq/logger"
	"github.com/streadway/amqp"
	"time"
)

// тип точки обмена
type ExchangeType string

const (
	DirectExchangeType ExchangeType = "direct"
	FanoutExchangeType              = "fanout"
	TopicExchangeType               = "topic"
)

// тип точки обмена для неотправленного письма
type FailureBindingType int

const (
	// проблемы с адресатом
	RecipientFailureBindingType FailureBindingType = iota

	// технические проблемы: неверная последовательность команд, косяки с dns
	TechnicalFailureBindingType

	// проблемы с подключеним к почтовому сервису
	ConnectionFailureBindingType

	// неизвестная проблема
	UnknownFailureBindingType
)

var (
	failureBindingTypeTplNames = map[FailureBindingType]string{
		RecipientFailureBindingType:  "%s.failure.recipient",
		TechnicalFailureBindingType:  "%s.failure.technical",
		ConnectionFailureBindingType: "%s.failure.connection",
		UnknownFailureBindingType:    "%s.failure.unknown",
	}

	// отложенные очереди вообще
	// письмо отправляется повторно при возниковении ошибки во время отправки
	delayedBindings = map[common.DelayedBindingType]*Binding{
		common.SecondDelayedBinding:        newDelayedBinding("%s.dlx.second", time.Second),
		common.ThirtySecondDelayedBinding:  newDelayedBinding("%s.dlx.thirty.second", time.Second*30),
		common.MinuteDelayedBinding:        newDelayedBinding("%s.dlx.minute", time.Minute),
		common.FiveMinutesDelayedBinding:   newDelayedBinding("%s.dlx.five.minutes", time.Minute*5),
		common.TenMinutesDelayedBinding:    newDelayedBinding("%s.dlx.ten.minutes", time.Minute*10),
		common.TwentyMinutesDelayedBinding: newDelayedBinding("%s.dlx.twenty.minutes", time.Minute*20),
		common.ThirtyMinutesDelayedBinding: newDelayedBinding("%s.dlx.thirty.minutes", time.Minute*30),
		common.FortyMinutesDelayedBinding:  newDelayedBinding("%s.dlx.forty.minutes", time.Minute*40),
		common.FiftyMinutesDelayedBinding:  newDelayedBinding("%s.dlx.fifty.minutes", time.Minute*50),
		common.HourDelayedBinding:          newDelayedBinding("%s.dlx.hour", time.Hour),
		common.SixHoursDelayedBinding:      newDelayedBinding("%s.dlx.six.hours", time.Hour*6),
		common.DayDelayedBinding:           newDelayedBinding("%s.dlx.day", time.Hour*24),
		common.NotSendDelayedBinding:       newBinding("%s.not.send"),
	}

	// отложенные очереди для лимитов
	limitBindings = []common.DelayedBindingType{
		common.SecondDelayedBinding,
		common.MinuteDelayedBinding,
		common.HourDelayedBinding,
		common.DayDelayedBinding,
	}

	limitBindingsLen = len(limitBindings)

	// цепочка очередей, используемых для повторной отправки писем
	// в качестве ключа используется текущий тип очереди, а в качестве значения следующий
	bindingsChain = map[common.DelayedBindingType]common.DelayedBindingType{
		common.UnknownDelayedBinding:       common.SecondDelayedBinding,
		common.SecondDelayedBinding:        common.ThirtySecondDelayedBinding,
		common.ThirtySecondDelayedBinding:  common.MinuteDelayedBinding,
		common.MinuteDelayedBinding:        common.FiveMinutesDelayedBinding,
		common.FiveMinutesDelayedBinding:   common.TenMinutesDelayedBinding,
		common.TenMinutesDelayedBinding:    common.TwentyMinutesDelayedBinding,
		common.TwentyMinutesDelayedBinding: common.ThirtyMinutesDelayedBinding,
		common.ThirtyMinutesDelayedBinding: common.FortyMinutesDelayedBinding,
		common.FortyMinutesDelayedBinding:  common.FiftyMinutesDelayedBinding,
		common.FiftyMinutesDelayedBinding:  common.HourDelayedBinding,
		common.HourDelayedBinding:          common.SixHoursDelayedBinding,
		common.SixHoursDelayedBinding:      common.NotSendDelayedBinding,
	}
)

// связка точки обмена и очереди
type Binding struct {
	// имя точки обмена и очереди
	Name string `yaml:"name"`

	// имя точки обмена
	Exchange string `yaml:"exchange"`

	// аргументы точки обмена
	ExchangeArgs amqp.Table

	// имя очереди
	Queue string `yaml:"queue"`

	// аргументы очереди
	QueueArgs amqp.Table

	// тип точки обмена
	Type ExchangeType `yaml:"type"`

	// ключ маршрутизации
	Routing string `yaml:"routing"`

	// количество потоков, разбирающих очередь
	Handlers int `yaml:"workers"`

	// количество сообщений, получаемых одновременно
	PrefetchCount int `yaml:"prefetchCount"`

	// отложенные очереди
	delayedBindings map[common.DelayedBindingType]*Binding

	// очереди для ошибок
	failureBindings map[FailureBindingType]*Binding
}

// создает связку обложенной точки обмена и очереди
func newDelayedBinding(name string, duration time.Duration) *Binding {
	binding := newBinding(name)
	binding.QueueArgs = amqp.Table{
		"x-message-ttl": int64(duration.Seconds()) * 1000,
	}
	return binding
}

// создает связку точки обмена и очереди
func newBinding(name string) *Binding {
	return &Binding{Name: name}
}

// инициализирует связку параметрами по умолчанию
func (b *Binding) init() {
	if len(b.Type) == 0 {
		b.Type = FanoutExchangeType
	}
	if len(b.Name) > 0 {
		b.Exchange = b.Name
		b.Queue = b.Name
	}
	// по умолчанию очередь разбирают столько рутин сколько ядер
	if b.Handlers == 0 {
		b.Handlers = common.DefaultWorkersCount
	}
	if b.PrefetchCount == 0 {
		b.PrefetchCount = 2
	}
}

// объявляет точку обмена и очередь и связывает их
func (b *Binding) declare(channel *amqp.Channel) {
	err := channel.ExchangeDeclare(
		b.Exchange,     // name of the exchange
		string(b.Type), // type
		true,           // durable
		false,          // delete when complete
		false,          // internal
		false,          // noWait
		b.ExchangeArgs, // arguments
	)
	if err != nil {
		logger.FailExit("consumer can't declare exchange %s, error - %v", b.Exchange, err)
	}

	_, err = channel.QueueDeclare(
		b.Queue,     // name of the queue
		true,        // durable
		false,       // delete when usused
		false,       // exclusive
		false,       // noWait
		b.QueueArgs, // arguments
	)
	if err != nil {
		logger.FailExit("consumer can't declare queue %s, error - %v", b.Queue, err)
	}

	err = channel.QueueBind(
		b.Queue,    // name of the queue
		b.Routing,  // bindingKey
		b.Exchange, // sourceExchange
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		logger.FailExit("consumer can't bind queue %s to exchange %s, error - %v", b.Queue, b.Exchange, err)
	}
}

// объявляет отложенную точку обмена и очередь и связывает их
func (b *Binding) declareDelayed(binding *Binding, channel *amqp.Channel) {
	b.Exchange = fmt.Sprintf(b.Name, binding.Exchange)
	b.Queue = fmt.Sprintf(b.Name, binding.Queue)
	if b.QueueArgs != nil {
		b.QueueArgs["x-dead-letter-exchange"] = binding.Exchange
	}
	b.Type = binding.Type
	b.declare(channel)
}
