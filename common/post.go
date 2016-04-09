package common

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// Максимальное количество попыток подключения к почтовику за отправку письма
	MaxTryConnectionCount int = 30
	MaxSendingCount       int = 96
)

var (
	// Регулярка для проверки адреса почты, сразу компилируем, чтобы при отправке не терять на этом время
	EmailRegexp = regexp.MustCompile(`^[\w\d\.\_\%\+\-]+@([\w\d\.\-]+\.\w{2,4})$`)
)

// таймауты приложения
type Timeout struct {
	Sleep      time.Duration `yaml:"sleep"`
	Waiting    time.Duration `yaml:"waiting"`
	Connection time.Duration `yaml:"connection"`
	Hello      time.Duration `yaml:"hello"`
	Mail       time.Duration `yaml:"mail"`
	Rcpt       time.Duration `yaml:"rcpt"`
	Data       time.Duration `yaml:"data"`
}

// инициализирует значения таймаутов по умолчанию
func (t *Timeout) Init() {
	if t.Sleep == 0 {
		t.Sleep = time.Second
	}
	if t.Waiting == 0 {
		t.Waiting = 30 * time.Second
	}
	if t.Connection == 0 {
		t.Connection = 5 * time.Minute
	}
	if t.Hello == 0 {
		t.Hello = 5 * time.Minute
	}
	if t.Mail == 0 {
		t.Mail = 5 * time.Minute
	}
	if t.Rcpt == 0 {
		t.Rcpt = 5 * time.Minute
	}
	if t.Data == 0 {
		t.Data = 10 * time.Minute
	}
}

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
	// сообщение
	Message string `json:"message"`

	// код ошибки
	Code int `json:"code"`
}

// письмо
type MailMessage struct {
	// идентификатор для логов
	Id int64 `json:"-"`

	// отправитель
	Envelope string `json:"envelope"`

	// получатель
	Recipient string `json:"recipient"`

	// тело письма
	Body string `json:"body"`

	// домен отправителя, удобно сразу получить и использовать далее
	HostnameFrom string `json:"-"`

	// Домен получателя, удобно сразу получить и использовать далее
	HostnameTo string `json:"-"`

	// дата создания, используется в основном сервисом ограничений
	CreatedDate time.Time `json:"-"`

	// тип очереди, в которою письмо уже было отправлено после неудачной отправки, ипользуется для цепочки очередей
	BindingType DelayedBindingType `json:"bindingType"`

	// ошибка отправки
	Error *MailError `json:"error"`

	TrySendingCount int `json:"trySendingCount"`
}

// инициализирует письмо
func (this *MailMessage) Init() {
	this.Id = time.Now().UnixNano()
	this.TrySendingCount++
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
	if err != nil {
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
		if event.Message.Error.Code == 421 {
			event.Result <- DelaySendEventResult
		} else {
			event.Result <- ErrorSendEventResult
		}
	}
}
