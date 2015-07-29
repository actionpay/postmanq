package common

import "time"

// тип события приложения
type ApplicationEventKind int

const (
	// инициализации сервисов
	InitApplicationEventKind ApplicationEventKind = iota

	// запуск сервисов
	RunApplicationEventKind

	// завершение сервисов
	FinishApplicationEventKind
)

// событие приложения
type ApplicationEvent struct {
	// тип события
	Kind ApplicationEventKind

	// данные из файла настроек
	Data []byte

	// аргументы командной строки
	Args map[string]interface{}
}

// возвращает аргумент, как булевый тип
func (e *ApplicationEvent) GetBoolArg(key string) bool {
	return e.Args[key].(bool)
}

// возвращает аргумент, как число
func (e *ApplicationEvent) GetIntArg(key string) int {
	return e.Args[key].(int)
}

// возвращает аргумент, как строку
func (e *ApplicationEvent) GetStringArg(key string) string {
	return e.Args[key].(string)
}

// создает событие с указанным типом
func NewApplicationEvent(kind ApplicationEventKind) *ApplicationEvent {
	return &ApplicationEvent{Kind: kind}
}

// результат отправки письма
type SendEventResult int

const (
	// успех
	SuccessSendEventResult SendEventResult = iota

	// превышение лимита
	OverlimitSendEventResult

	// ошибка
	ErrorSendEventResult

	// повторная отправка через некоторое время
	DelaySendEventResult

	// отмена отправки
	RevokeSendEventResult
)

// событие отправки письма
type SendEvent struct {
	// елиент для отправки писем
	Client *SmtpClient

	// письмо, полученное из очереди
	Message *MailMessage

	// дата создания необходима при получении подключения к почтовому сервису
	CreateDate time.Time

	// результат
	Result chan SendEventResult

	// количество попыток отправок письма
	TryCount int

	// итератор сервисов, участвующих в отправке письма
	Iterator *Iterator

	// очередь, в которую необходимо будет положить клиента после отправки письма
	Queue *LimitedQueue
}

// создает событие отправки сообщения
func NewSendEvent(message *MailMessage) *SendEvent {
	event := new(SendEvent)
	event.Message = message
	event.CreateDate = time.Now()
	event.Result = make(chan SendEventResult)
	event.Iterator = NewIterator(Services)
	return event
}
