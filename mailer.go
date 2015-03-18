package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"github.com/streadway/amqp"
	"regexp"
	"strings"
	"fmt"
	"time"
	"errors"
	"github.com/byorty/dkim"
	"io/ioutil"
	"sync/atomic"
	"strconv"
)

var (
	// сразу компилирует регулярку для проверки адреса почты, чтобы при отправке не терять на этом время
	emailRegexp = regexp.MustCompile(`^[\w\d\.\_\%\+\-]+@([\w\d\.\-]+\.\w{2,4})$`)
	// хранит количество отправленных писем за минуту, используется для отладки
	mailsPerMinute int64
	mailer         *Mailer
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
	matches := emailRegexp.FindAllStringSubmatch(email, -1)
	if len(matches) == 1 && len(matches[0]) == 2 {
		return matches[0][1], nil
	} else {
		Warn("can't receive hostname from %s", email)
		return "", errors.New("invalid email address")
	}
}

// сервис отправки писем
type Mailer struct {
	MailersCount       int                 `yaml:"workers"`      // количество отправителей
	PrivateKeyFilename string              `yaml:"privateKey"`   // путь до закрытого ключа
	DkimSelector       string              `yaml:"dkimSelector"` // селектор
	events             chan *SendEvent                           // канал для писем
	privateKey         []byte                                    // содержимое приватного ключа
}

// создает новый сервис отправки писем
func MailerOnce() *Mailer {
	if mailer == nil {
		mailer = new(Mailer)
		mailer.events = make(chan *SendEvent)
	}
	return mailer
}

// инициализирует сервис отправки писем
func (this *Mailer) OnInit(event *ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		Debug("read private key file %s", this.PrivateKeyFilename)
		// закрытый ключ должен быть указан обязательно
		// поэтому даже не проверяем что указано в переменной
		this.privateKey, err = ioutil.ReadFile(this.PrivateKeyFilename)
		if err == nil {
			Debug("private key read success")
		} else {
			Debug("can't read private key")
			FailExitWithErr(err)
		}
		// указываем заголовки для DKIM
		dkim.StdSignableHeaders = []string {
			"From",
			"To",
			"Subject",
		}
		// если не задан селектор, устанавливаем селектор по умолчанию
		if len(this.DkimSelector) == 0 {
			this.DkimSelector = "mail"
		}
		if this.MailersCount == 0 {
			this.MailersCount = defaultWorkersCount
		}
	} else {
		FailExitWithErr(err)
	}
}

// запускает отправителей и прием сообщений из очереди
func (this *Mailer) OnRun() {
	Debug("run mailers apps...")
	// выводим количество отправленных писем в минуту
	go this.showMailsPerMinute()

	for i := 0;i < this.MailersCount;i++ {
		go this.sendMails(i + 1)
	}
}

// выводит количество отправленных писем раз в минуту
func (this *Mailer) showMailsPerMinute() {
	tick := time.Tick(time.Minute)
	for {
		select {
		case <- tick:
			Debug("mailers send %d mails per minute", atomic.LoadInt64(&mailsPerMinute))
			atomic.StoreInt64(&mailsPerMinute, 0)
			break
		}
	}
}

func (this *Mailer) sendMails(id int) {
	for event := range this.events {
		this.sendMail(id, event)
	}
}

func (this *Mailer) sendMail(id int, event *SendEvent) {
	if emailRegexp.MatchString(event.Message.Envelope) && emailRegexp.MatchString(event.Message.Recipient) {
		this.prepareMail(id, event.Message)
		this.send(id, event)
	} else {
		ReturnMail(event, errors.New(fmt.Sprintf("511 mailer#%d can't send mail#%d, envelope or ricipient is invalid", id, event.Message.Id)))
	}
}

func (this *Mailer) prepareMail(id int, message *MailMessage) {
	conf, err := dkim.NewConf(message.HostnameFrom, this.DkimSelector)
	if err == nil {
		conf[dkim.AUIDKey] = message.Envelope
		conf[dkim.CanonicalizationKey] = "relaxed/relaxed"
		signer, err := dkim.New(conf, this.privateKey)
		if err == nil {
			signed, err := signer.Sign([]byte(message.Body))
			if err == nil {
				message.Body = string(signed)
			} else {
				Warn("mailer#%d can't sign mail#%d, error - %v", id, message.Id, err)
			}
		} else {
			Warn("mailer#%d can't create dkim for mail#%d, error - %v", id, message.Id, err)
		}
	} else {
		Warn("mailer#%d can't create dkim config for mail#%d, error - %v", id, message.Id, err)
	}
}

func (this *Mailer) send(id int, event *SendEvent) {
	worker := event.Client.Worker
	Info("mailer#%d try send mail#%d", id, event.Message.Id)
	Debug("mailer#%d receive smtp client#%d", id, event.Client.Id)

	err := worker.Mail(event.Message.Envelope)
	if err == nil {
		Debug("mailer#%d send command MAIL FROM: %s", id, event.Message.Envelope)
		event.Client.SetTimeout(RCPT_TIMEOUT)
		err = worker.Rcpt(event.Message.Recipient)
		if err == nil {
			Debug("mailer#%d send command RCPT TO: %s", id, event.Message.Recipient)
			event.Client.SetTimeout(DATA_TIMEOUT)
			wc, err := worker.Data()
			if err == nil {
				Debug("mailer#%d send command DATA", id)
				_, err = fmt.Fprint(wc, event.Message.Body)
				if err == nil {
					err = wc.Close()
					Debug("%s", event.Message.Body)
					if err == nil {
						Debug("mailer#%d send command .", id)
						// стараемся слать письма через уже созданное соединение,
						// поэтому после отправки письма не закрываем соединение
						err = worker.Reset()
						if err == nil {
							Debug("mailer#%d send command RSET", id)
							Info("mailer#%d success send mail#%d", id, event.Message.Id)
							// для статы
							atomic.AddInt64(&mailsPerMinute, 1)
							// отпускаем поток получателя сообщений из очереди
							event.Result <- SEND_EVENT_RESULT_SUCCESS
						} else {
							ReturnMail(event, err)
						}
					} else {
						ReturnMail(event, err)
					}
				} else {
					ReturnMail(event, err)
				}
			} else {
				ReturnMail(event, err)
			}
		} else {
			ReturnMail(event, err)
		}
	} else {
		ReturnMail(event, err)
	}
	// говорим, что соединение свободно, его можно передать другому отправителю
	if event.Client.IsExpireByNow() {
		atomic.StoreInt32(&(event.Client.Status), SMTP_CLIENT_STATUS_EXPIRE)
	} else {
		event.Client.SetTimeout(WAITING_TIMEOUT)
		atomic.StoreInt32(&(event.Client.Status), SMTP_CLIENT_STATUS_WAITING)
	}
}

func (this *Mailer) OnFinish() {
	close(this.events)
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
	Warn("mail#%d sending error - %v", event.Message.Id, err)
	// отпускаем поток получателя сообщений из очереди
	if event.Message.Error == nil {
		event.Result <- SEND_EVENT_RESULT_DELAY
	} else {
		event.Result <- SEND_EVENT_RESULT_ERROR
	}
}
