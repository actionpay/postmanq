package common

import (
	"crypto/x509"
	"time"
)

// тип гловального события приложения
type ApplicationEventKind int

const (
	InitApplicationEventKind   ApplicationEventKind = iota // событие инициализации сервисов
	RunApplicationEventKind                                // событие запуска сервисов
	FinishApplicationEventKind                             // событие завершения сервисов
)

// событие приложения
type ApplicationEvent struct {
	Kind ApplicationEventKind
	Data []byte
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

// создает событие с указанным типом
func NewApplicationEvent(kind ApplicationEventKind) *ApplicationEvent {
	return &ApplicationEvent{Kind: kind}
}

type SendEventResult int

const (
	SuccessSendEventResult SendEventResult = iota
	OverlimitSendEventResult
	ErrorSendEventResult
	DelaySendEventResult
)

// Событие отправки письма
type SendEvent struct {
	Client           *SmtpClient    // объект, содержащий подключение и клиент для отправки писем
	CertPool         *x509.CertPool // пул сертификатов
	CertBytes        []byte         // для каждого почтового сервиса необходим подписывать сертификат, поэтому в событии храним сырые данные сертификата
	CertBytesLen     int            // длина сертификата, по если длина больше 0, тогда пытаемся отправлять письма через TLS
	Message          *MailMessage   // само письмо, полученное из очереди
	DefaultPrevented bool           // флаг, сигнализирующий обрабатывать ли событие
	CreateDate       time.Time      // дата создания необходима при получении подключения к почтовому сервису
	Result           chan SendEventResult
	//	MailServers      chan *MailServer
	//	MailServer       *MailServer
	TryCount int
	Iterator *Iterator
	Queue *LimitedQueue
}

// отправляет письмо сервису отправки, который затем решит, какой поток будет отправлять письмо
func NewSendEvent(message *MailMessage) *SendEvent {
	event := new(SendEvent)
	event.DefaultPrevented = false
	event.Message = message
	event.CreateDate = time.Now()
	event.Result = make(chan SendEventResult)
	event.Iterator = NewIterator(SendindServices)
	return event
}
