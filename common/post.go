package common

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// Таймауты
	ReceiveConnectionTimeout = 5 * time.Minute
	SleepTimeout             = 1000 * time.Millisecond
	HelloTimeout             = 5 * time.Minute
	MailTimeout              = 5 * time.Minute
	RcptTimeout              = 5 * time.Minute
	DataTimeout              = 10 * time.Minute
	WaitingTimeout           = 30 * time.Second

	// Максимальное количество попыток подключения к почтовику за отправку письма
	MaxTryConnectionCount = 30
)

var (
	// Регулярка для проверки адреса почты, сразу компилируем, чтобы при отправке не терять на этом время
	EmailRegexp = regexp.MustCompile(`^[\w\d\.\_\%\+\-]+@([\w\d\.\-]+\.\w{2,4})$`)
)

// Тип отложенной очереди
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

// Ошибка во время отпрвки письма
type MailError struct {
	// Сообщение
	Message string `json:"message"`

	// Код ошибки
	Code int `json:"code"`
}

// Письмо
type MailMessage struct {
	// Идентификатор для логов
	Id int64 `json:"-"`

	// Отправитель
	Envelope string `json:"envelope"`

	// Получатель
	Recipient string `json:"recipient"`

	// Тело письма
	Body string `json:"body"`

	// Домен отправителя, удобно сразу получить и использовать далее
	HostnameFrom string `json:"-"`

	// Домен получателя, удобно сразу получить и использовать далее
	HostnameTo string `json:"-"`

	// Дата создания, используется в основном сервисом ограничений
	CreatedDate time.Time `json:"-"`

	// Тип очереди, в которою письмо уже было отправлено после неудачной отправки, ипользуется для цепочки очередей
	BindingType DelayedBindingType `json:"bindingType"`

	// Ошибка отправки
	Error *MailError `json:"error"`
}

// Инициализирует письмо
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

// Получает домен из адреса
func (this *MailMessage) getHostnameFromEmail(email string) (string, error) {
	matches := EmailRegexp.FindAllStringSubmatch(email, -1)
	if len(matches) == 1 && len(matches[0]) == 2 {
		return matches[0][1], nil
	} else {
		return "", errors.New("invalid email address")
	}
}

// Возвращает письмо обратно в очередь после ошибки во время отправки
func ReturnMail(event *SendEvent, err error) {
	// необходимо проверить сообщение на наличие кода ошибки
	// обычно код идет первым
	errorMessage := err.Error()
	parts := strings.Split(errorMessage, " ")
	if len(parts) > 0 {
		// пытаемся получить код
		code, e := strconv.Atoi(strings.TrimSpace(parts[0]))
		// и создать ошибку
		// письмо с ошибкой вернется в другую очередь, отличную от письмо без ошибки
		if e == nil {
			event.Message.Error = &MailError{errorMessage, code}
		}
	}

	// если в событии уже создан клиент
	if event.Client != nil {
		if event.Client.Worker != nil {
			// сбрасываем цепочку команд к почтовому сервису
			event.Client.Worker.Reset()
		}
	}

	// отпускаем поток получателя сообщений из очереди
	if event.Message.Error == nil {
		event.Result <- DelaySendEventResult
	} else {
		event.Result <- ErrorSendEventResult
	}
}
