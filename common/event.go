package common

import "time"

// Тип события приложения
type ApplicationEventKind int

const (
	// Инициализации сервисов
	InitApplicationEventKind ApplicationEventKind = iota

	// Запуска сервисов
	RunApplicationEventKind

	// Завершение сервисов
	FinishApplicationEventKind
)

// Событие приложения
type ApplicationEvent struct {
	// Тип события
	Kind ApplicationEventKind

	// Данные из файла настроек
	Data []byte

	// Аргументы командной строки
	Args map[string]interface{}
}

func (e *ApplicationEvent) GetBoolArg(key string) bool {
	return e.Args[key].(bool)
}

func (e *ApplicationEvent) GetIntArg(key string) int {
	return e.Args[key].(int)
}

func (e *ApplicationEvent) GetStringArg(key string) string {
	return e.Args[key].(string)
}

// Создает событие с указанным типом
func NewApplicationEvent(kind ApplicationEventKind) *ApplicationEvent {
	return &ApplicationEvent{Kind: kind}
}

// Результат отправки письма
type SendEventResult int

const (
	// Успех
	SuccessSendEventResult SendEventResult = iota

	// Превышение лимита
	OverlimitSendEventResult

	// Ошибка
	ErrorSendEventResult

	// Повторная отправка через некоторое время
	DelaySendEventResult

	// Отмена отправки
	RevokeSendEventResult
)

// Событие отправки письма
type SendEvent struct {
	// Клиент для отправки писем
	Client *SmtpClient

	// Письмо, полученное из очереди
	Message *MailMessage

	// Флаг, сигнализирующий обрабатывать ли событие
	DefaultPrevented bool

	// Дата создания необходима при получении подключения к почтовому сервису
	CreateDate time.Time

	// Результат
	Result chan SendEventResult

	// Количество попыток отправок письма
	TryCount int

	// Итератор сервисов, участвующих в отправке письма
	Iterator *Iterator

	// Очередь, в которую необходимо будет положить клиента после отправки письма
	Queue *LimitedQueue
}

// Создает событие отправки сообщения
func NewSendEvent(message *MailMessage) *SendEvent {
	event := new(SendEvent)
	event.DefaultPrevented = false
	event.Message = message
	event.CreateDate = time.Now()
	event.Result = make(chan SendEventResult)
	event.Iterator = NewIterator(Services)
	return event
}
