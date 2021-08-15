package common

import (
	"time"
)

// ApplicationEventKind тип события приложения
type ApplicationEventKind int

const (
	// InitApplicationEventKind инициализации сервисов
	InitApplicationEventKind ApplicationEventKind = iota

	// RunApplicationEventKind запуск сервисов
	RunApplicationEventKind

	// FinishApplicationEventKind завершение сервисов
	FinishApplicationEventKind
)

// ApplicationEvent событие приложения
type ApplicationEvent struct {
	// Kind тип события
	Kind ApplicationEventKind

	// Data данные из файла настроек
	Data []byte

	// Args аргументы командной строки
	Args map[string]interface{}
}

// GetBoolArg возвращает аргумент, как булевый тип
func (e *ApplicationEvent) GetBoolArg(key string) bool {
	return e.Args[key].(bool)
}

// GetIntArg возвращает аргумент, как число
func (e *ApplicationEvent) GetIntArg(key string) int {
	return e.Args[key].(int)
}

// GetStringArg возвращает аргумент, как строку
func (e *ApplicationEvent) GetStringArg(key string) string {
	return e.Args[key].(string)
}

// NewApplicationEvent создает событие с указанным типом
func NewApplicationEvent(kind ApplicationEventKind) *ApplicationEvent {
	return &ApplicationEvent{Kind: kind}
}

// SendEventResult результат отправки письма
type SendEventResult int

const (
	// SuccessSendEventResult успех
	SuccessSendEventResult SendEventResult = iota

	// OverlimitSendEventResult превышение лимита
	OverlimitSendEventResult

	// ErrorSendEventResult ошибка
	ErrorSendEventResult

	// DelaySendEventResult повторная отправка через некоторое время
	DelaySendEventResult

	// RevokeSendEventResult отмена отправки
	RevokeSendEventResult
)

// SendEvent событие отправки письма
type SendEvent struct {
	// Client клиент для отправки писем
	Client *SmtpClient

	// Message письмо, полученное из очереди
	Message *MailMessage

	// CreateDate дата создания необходима при получении подключения к почтовому сервису
	CreateDate time.Time

	// Result результат
	Result chan SendEventResult

	// TryCount количество попыток отправок письма
	TryCount int

	// Iterator итератор сервисов, участвующих в отправке письма
	Iterator *Iterator

	// Queue очередь, в которую необходимо будет положить клиента после отправки письма
	Queue *LimitedQueue
}

// NewSendEvent создает событие отправки сообщения
func NewSendEvent(message *MailMessage) *SendEvent {
	event := new(SendEvent)
	event.Message = message
	event.CreateDate = time.Now()
	event.Result = make(chan SendEventResult)
	event.Iterator = NewIterator(Services)
	return event
}
