package common

import (
	"errors"
	"regexp"
	"time"
)

const (
	// Максимальное количество попыток подключения к почтовику за отправку письма
	MaxTryConnectionCount int    = 30
	AllDomains            string = "*"
	EmptyStr              string = ""
)

var (
	// EmailRegexp Регулярка для проверки адреса почты, сразу компилируем, чтобы при отправке не терять на этом время
	//nolint:lll
	EmailRegexp   = regexp.MustCompile("^(?:[a-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\\.[a-z0-9!#$%&'*+/=?^_`{|}~-]+)*|\"(?:[\\x01-\\x08\\x0b\\x0c\\x0e-\\x1f\\x21\\x23-\\x5b\\x5d-\\x7f]|\\\\[\\x01-\\x09\\x0b\\x0c\\x0e-\\x7f])*\")@((?:(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?|\\[(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?|[a-z0-9-]*[a-z0-9]:(?:[\\x01-\\x08\\x0b\\x0c\\x0e-\\x1f\\x21-\\x5a\\x53-\\x7f]|\\\\[\\x01-\\x09\\x0b\\x0c\\x0e-\\x7f])+)\\]))$")
	EmptyStrSlice = []string{}
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
	Body []byte `json:"body"`

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
}

// инициализирует письмо
func (this *MailMessage) Init() {
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
