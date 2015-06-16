package common

import (
	"errors"
	"github.com/streadway/amqp"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	UnlimitedConnectionCount = -1              // безлимитное количество соединений к почтовому сервису
	ReceiveConnectionTimeout = 5 * time.Minute // время ожидания для получения соединения к почтовому сервису
	SleepTimeout             = 1000 * time.Millisecond
	HelloTimeout             = 5 * time.Minute
	MailTimeout              = 5 * time.Minute
	RcptTimeout              = 5 * time.Minute
	DataTimeout              = 10 * time.Minute
	WaitingTimeout           = 30 * time.Second
	TryConnectCount          = 30
)

var (
	// сразу компилирует регулярку для проверки адреса почты, чтобы при отправке не терять на этом время
	EmailRegexp = regexp.MustCompile(`^[\w\d\.\_\%\+\-]+@([\w\d\.\-]+\.\w{2,4})$`)
)

// тип отложенной очереди
type DelayedBindingType int

const (
	UnknownDelayedBinding DelayedBindingType = iota
	SecondDelayedBinding
	ThirtySecondDelayedBinding
	MinuteDelayedBinding
	FiveMinutesDelayedBinding
	TenMinutesDelayedBinding
	TwentyMinutesDelayedBinding
	ThirtyMinutesDelayedBinding
	FortyMinutesDelayedBinding
	FiftyMinutesDelayedBinding
	HourDelayedBinding
	SixHoursDelayedBinding
	DayDelayedBinding
	NotSendDelayedBinding
)

// ошибка во время отпрвки письма
type MailError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// письмо
type MailMessage struct {
	Id           int64              `json:"-"`           // номер сообщения для логов
	Envelope     string             `json:"envelope"`    // отправитель
	Recipient    string             `json:"recipient"`   // получатель
	Body         string             `json:"body"`        // тело письма
	Delivery     amqp.Delivery      `json:"-"`           // получение сообщения очереди
	HostnameFrom string             `json:"-"`           // домен отправителя, удобно сразу получить и использовать при работе с соединением и сертификатом
	HostnameTo   string             `json:"-"`           // домен получателя, удобно сразу получить и использовать при работе с соединением и сертификатом
	CreatedDate  time.Time          `json:"-"`           // дата создания, используется в основном сервисом ограничений
	BindingType  DelayedBindingType `json:"bindingType"` // тип очереди, в которою письмо уже было отправлено после неудачной отправки, ипользуется для цепочки очередей
	Error        *MailError         `json:"error"`       // ошибка отправки
}

// инициализирует письмо
func (this *MailMessage) Init() {
	// удобно во время отладки просматривать, что происходит с письмом
	this.Id = time.Now().UnixNano()
	this.CreatedDate = time.Now()
	if hostname, err := this.getHostnameFromEmail(this.Envelope); err == nil {
		this.HostnameFrom = hostname
	}
	if hostname, err := this.getHostnameFromEmail(this.Recipient); err == nil {
		this.HostnameTo = hostname
	}
}

// получает домен из адреса
func (this *MailMessage) getHostnameFromEmail(email string) (string, error) {
	matches := EmailRegexp.FindAllStringSubmatch(email, -1)
	if len(matches) == 1 && len(matches[0]) == 2 {
		return matches[0][1], nil
	} else {
		return "", errors.New("invalid email address")
	}
}

// возвращает письмо обратно в очередь после ошибки во время отправки
func ReturnMail(event *SendEvent, err error) {
	// необходимо проверить сообщение на наличие кода ошибки
	// обычно код идет первым
	parts := strings.Split(err.Error(), " ")
	if len(parts) > 0 {
		// пытаемся получить код
		code, e := strconv.Atoi(strings.TrimSpace(parts[0]))
		// и создать ошибку
		// письмо с ошибкой вернется в отличную очередь, чем письмо без ошибки
		if e == nil {
			event.Message.Error = &MailError{strings.Join(parts[1:], " "), code}
		}
	}
	if event.Client != nil {
		if event.Client.Worker != nil {
			event.Client.Worker.Reset()
		}
	}
	//	Warn("mail#%d sending error - %v", event.Message.Id, err)
	// отпускаем поток получателя сообщений из очереди
	if event.Message.Error == nil {
		event.Result <- DelaySendEventResult
	} else {
		event.Result <- ErrorSendEventResult
	}
}
