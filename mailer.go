package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"net/url"
	"github.com/streadway/amqp"
	"regexp"
	"strings"
	"fmt"
	"time"
	"errors"
	"github.com/byorty/dkim"
	"io/ioutil"
	"sync/atomic"
	"sync"
	"runtime"
	"strconv"
)

var (
	// сразу компилирует регулярку для проверки адреса почты, чтобы при отправке не терять на этом время
	emailRegexp = regexp.MustCompile(`^[\w\d\.\_\%\+\-]+@([\w\d\.\-]+\.\w{2,4})$`)
	// минимальные набор заголовков, который должен быть в письме
	// если какого-нибудь заголовка не будет в письме, отправитель создаст заголовок со значением по умолчанию
	defaultHeaders = map[string]func(*MailMessage) string {
//		"Return-Path"              : getReturnPath,
//		"MIME-Version"             : getMimeVersion,
		"From"                     : getFrom,
		"To"                       : getTo,
//		"Date"                     : getDate,
		"Subject"                  : getSubject,
//		"Content-Type"             : getContentType,
	}
	// используется для создания id письма, удобно для отладки
	mailsCount     int64
	// хранит количество отправленных писем за минуту, используется для отладки
	mailsPerMinute int64
	mailer         *Mailer
)

func getReturnPath(message *MailMessage) string {
	return message.Envelope
}

func getMimeVersion(message *MailMessage) string {
	return "1.0"
}

func getFrom(message *MailMessage) string {
	return message.Envelope
}

func getTo(message *MailMessage) string {
	return message.Recipient
}

func getDate(message *MailMessage) string {
	return fmt.Sprintf("%s", message.CreatedDate.Format(time.RFC1123Z))
}

func getSubject(message *MailMessage) string {
	return "No subject"
}

func getContentType(message *MailMessage) string {
	return "text/plain; charset=utf-8"
}

func getContentTransferEncoding(message *MailMessage) string {
	return "7bit"
}

// ошибка во время отпрвки письма
type MailError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// письмо
type MailMessage struct {
	Id           int64              `json:"-"`           // номер сообщения
	Envelope     string             `json:"envelope"`    // отправитель
	Recipient    string             `json:"recipient"`   // получатель
	Body         string             `json:"body"`        // тело письма
	Delivery     amqp.Delivery      `json:"-"`           // получение сообщения очереди
	Done         chan bool          `json:"-"`           // канал, блокирующий получателя сообщений очереди
	HostnameFrom string             `json:"-"`           // домен отправителя, удобно сразу получить и использовать при работе с соединением и сертификатом
	HostnameTo   string             `json:"-"`           // домен получателя, удобно сразу получить и использовать при работе с соединением и сертификатом
	CreatedDate  time.Time          `json:"-"`           // дата создания, используется в основном сервисом ограничений
	Overlimit    bool               `json:"-"`           // флаг, говорящий, что сообщение не отправлено, т.к. превышено ограничение для конкретного постового сервиса
	BindingType  DelayedBindingType `json:"bindingType"` // тип очереди, в которою письмо уже было отправлено после неудачной отправки, ипользуется для цепочки очередей
	Error        *MailError         `json:"error"`       // ошибка отправки
	App          MailerApplication  `json:"-"`           // ссылка на отправщика письма, используется для логирования после неудачной отправки
}

// инициализирует письмо
func (this *MailMessage) Init() {
	atomic.AddInt64(&mailsCount, 1)
	// удобно во время отладки просматривать, что происходит с письмом
	this.Id = atomic.LoadInt64(&mailsCount)
	this.Done = make(chan bool)
	this.CreatedDate = time.Now()
	this.Overlimit = false
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
	AppsConfigs        []*MailerApplicationConfig `yaml:"mailers"`      // настройки приложений отправителей писем
	PrivateKeyFilename string                     `yaml:"privateKey"`   // путь до закрытого ключа
	DkimSelector       string                     `yaml:"dkimSelector"` // селектор
	messages           chan *MailMessage                                // канал писем
	apps               []MailerApplication                              // приложения отправители писем
	sendServices       []SendService                                    // сервисы, принимающие участие в отправке письма
	mutex              *sync.Mutex                                      // семафор
}

// создает новый сервис отправки писем
func NewMailer() *Mailer {
	if mailer == nil {
		mailer = new(Mailer)
	}
	return mailer
}

// инициализирует сервис отправки писем
func (this *Mailer) OnInit(event *InitEvent) {
	var privateKey []byte
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		Debug("read private key file %s", this.PrivateKeyFilename)
		// закрытый ключ должен быть указан обязательно
		// поэтому даже не проверяем что указано в переменной
		privateKey, err = ioutil.ReadFile(this.PrivateKeyFilename)
		if err == nil {
			Debug("private key read success")
		} else {
			Debug("can't read private key")
			FailExitWithErr(err)
		}
		// указываем заголовки для DKIM
		dkim.StdSignableHeaders = make([]string, 0)
		for key, _ := range defaultHeaders {
			dkim.StdSignableHeaders = append(dkim.StdSignableHeaders, key)
		}

		// если не задан селектор, устанавливаем селектор по умолчанию
		if len(this.DkimSelector) == 0 {
			this.DkimSelector = "mail"
		}
		this.mutex = new(sync.Mutex)
		// создаем канал писем
		this.messages = make(chan *MailMessage)
		this.apps = make([]MailerApplication, 0)
		// указываем сервисы, принимающие участие в отправке письма
		this.sendServices = []SendService{
			// сначала смотрим, что не превышено ограничение на отправку писем
			NewLimiter(),
			// затем получаем соединение к почтовому сервису
			NewConnector(),
			// и только потом отправляем письмо
			NewMailer(),
		}
		// создаем отправителей для каждого IP
		for j, appConfig := range this.AppsConfigs {
			// если в настройке не указано количество отправителей
			// задаем одного отправителя для IP
			if appConfig.Handlers == 0 {
				appConfig.Handlers = 1
			}
			for i := 0; i < appConfig.Handlers; i++ {
				app := NewBaseMailerApplication()
				app.SetId(j * appConfig.Handlers + i)
				app.SetPrivateKey(privateKey)
				app.SetDkimSelector(this.DkimSelector)
				app.Init(appConfig)
				Debug("create mailer app#%d", app.GetId())
				this.apps = append(this.apps, app)
			}
		}
		event.MailersCount = len(this.apps)
	} else {
		FailExitWithErr(err)
	}
}

// запускает отправителей и прием сообщений из очереди
func (this *Mailer) OnRun() {
	Debug("run mailers apps...")
	// выводим количество отправленных писем в минуту
	go this.showMailsPerMinute()
	// у каждого отправителя свой канал, говорим, чтобы отправитель начал его слушать
	for _, app := range this.apps {
		go this.runApp(app)
	}
	// начинаем получать письма из очереди в отдельном потоке
	for i := 0;i < runtime.NumCPU();i++ {
		go this.listenMessages()
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

// запускает получение писем из очереди в отдельном потоке
func (this *Mailer) listenMessages() {
	for message := range this.messages {
		// создает событие отправки письма и оповещает слушателей
		go this.triggerSendEvent(message)
	}
}

// создает событие отправки письма и оповещает слушателей
func (this *Mailer) triggerSendEvent(message *MailMessage) {
	event := new(SendEvent)
	event.DefaultPrevented = false
	event.Message = message
	event.CreateDate = time.Now()
	for _, service := range this.sendServices {
		if event.DefaultPrevented {
			break
		} else {
			service.OnSend(event)
		}
	}
	event = nil
}

// запускает получение писем от сервиса отправки в отдельном потоке
func (this *Mailer) runApp(app MailerApplication) {
	for event := range app.Channel() {
		// если письмо имеет нормальные адреса отправителя и получателя
		if app.IsValidMessage(event.Message) {
			// создаем DKIM
			app.PrepareMail(event.Message)
			// отправляем письмо
			app.Send(event)
		}
	}
}

// при отправке письма ищет менее загруженного отправителя и отдает ему письмо для отправки
func (this *Mailer) OnSend(event *SendEvent) {
	this.mutex.Lock()
	index := -1
	var count int64 = -1
	for i, app := range this.apps {
		// просматриваем количество отправляемых писем отправителем по домену
		messageCount := app.MessagesCountByHostname(event.Message.HostnameTo)
		Debug("mailer#%d, messages count - %d", i, messageCount)
		// ищем менее загруженного
		if count == -1 || count > messageCount {
			count = messageCount
			index = i
		}
	}
	// если отправитель нашелся
	if 0 <= index && index <= len(this.apps) {
		app := this.apps[index]
		Debug("send message to mailer#%d", index)
		// увеличиваем количество отправляемых писем
		app.IncrMessagesCountByHostname(event.Message.HostnameTo)
		// передаем ему письмо для отправки
		app.Channel() <- event
	}
	this.mutex.Unlock()
}

func (this *Mailer) OnFinish() {}

// отправляет письмо сервису отправки, который затем решит, какой поток будет отправлять письмо
func SendMail(message *MailMessage) {
	mailer.messages <- message
}

// возвращает письмо обратно в очередь после ошибки во время отправки
func ReturnMail(message *MailMessage, err error) {
	// необходимо проверить сообщение на наличие кода ошибки
	// обычно код идет первым
	parts := strings.Split(err.Error(), " ")
	if len(parts) > 0 {
		// пытаемся получить код
		code, e := strconv.Atoi(parts[0])
		// и создать ошибку
		// письмо с ошибкой вернется в отличную очередь, чем письмо без ошибки
		if e == nil {
			message.Error = &MailError{strings.Join(parts[1:], " "), code}
		}
	}
	// отпускаем поток получателя сообщений из очереди
	message.Done <- false
	if message.App == nil {
		WarnWithErr(err)
	} else {
		Warn("mailer#%d error - %v", message.App.GetId(), err)
	}
}

// настройки отправителя
type MailerApplicationConfig struct {
	URI      string        `yaml:"uri"`      // IP c портом
	Handlers int           `yaml:"handlers"` // количество потоков, которые будут отправлять письма с одного IP
}

// отправитель
type MailerApplication interface {
	//инициализирует отправителя
	Init(*MailerApplicationConfig)
	// возвращает номер отправителя, используется во время отладки
	GetId() int
	// устанавливает номер отправителя
	SetId(int)
	// увеличивает количество отправленных писем по домену
	IncrMessagesCountByHostname(string)
	// возвращает количество отправленных писем по домену
	MessagesCountByHostname(string) int64
	// задает закрытый ключ, который будет использоваться во время создания DKIM
	SetPrivateKey([]byte)
	// возвращает канал для отправки писем отправителю
	Channel() chan *SendEvent
	// проверяет заголовки письма, при необходимости добавляет заголовки со значениями по умолчанию
	PrepareMail(*MailMessage)
	// отправляет письмо
	Send(*SendEvent)
	// проверяет формат адресов отправителя и получателя
	IsValidMessage(*MailMessage) bool
	// устанавливает селектор DKIM, должен совпадать с селектором в DNS записи
	SetDkimSelector(string)
}

var (
	CRLF = "\r\n"
)

// реализация отправителя
type BaseMailerApplication struct {
	id             int
	messagesCounts map[string]int64
	events         chan *SendEvent
	config         *MailerApplicationConfig
	uri            *url.URL
	privateKey     []byte
	hostname       string
	dkimSelector   string
}

// создает нового отправителя
func NewBaseMailerApplication() MailerApplication {
	return new(BaseMailerApplication)
}

// возвращает номер отправителя, используется во время отладки
func (this *BaseMailerApplication) GetId() int {
	return this.id
}

// устанавливает номер отправителя
func (this *BaseMailerApplication) SetId(id int) {
	this.id = id
}

// возвращает количество отправленных писем по домену
func (this *BaseMailerApplication) MessagesCountByHostname(hostname string) int64 {
	if _, ok := this.messagesCounts[hostname]; !ok {
		this.messagesCounts[hostname] = 0
	}
	return this.messagesCounts[hostname]
}

// задает закрытый ключ, который будет использоваться во время создания DKIM
func (this *BaseMailerApplication) SetPrivateKey(privateKey []byte) {
	this.privateKey = privateKey
}

// возвращает канал для отправки писем отправителю
func (this *BaseMailerApplication) Channel() chan *SendEvent {
	return this.events
}

//инициализирует отправителя
func (this *BaseMailerApplication) Init(config *MailerApplicationConfig) {
	var err error
	this.config = config
	this.messagesCounts = make(map[string]int64)
	this.events = make(chan *SendEvent)
	this.uri, err = url.Parse(config.URI)
	if err == nil {
		Debug("url parsed %v", this.uri)
	} else {
		FailExitWithErr(err)
	}
}

// увеличивает количество отправленных писем по домену
func (this *BaseMailerApplication) IncrMessagesCountByHostname(hostname string) {
	if _, ok := this.messagesCounts[hostname]; ok {
		this.messagesCounts[hostname]++
	} else {
		this.messagesCounts[hostname] = 1
	}
}

// проверяет формат адресов отправителя и получателя
func (this *BaseMailerApplication) IsValidMessage(message *MailMessage) bool {
	return emailRegexp.MatchString(message.Envelope) && emailRegexp.MatchString(message.Recipient)
}

// создает DKIM
func (this *BaseMailerApplication) PrepareMail(message *MailMessage) {
	conf, err := dkim.NewConf(message.HostnameFrom, this.dkimSelector)
	if err != nil {
		WarnWithErr(err)
	}
	conf[dkim.AUIDKey] = message.Envelope
	conf[dkim.CanonicalizationKey] = "relaxed/relaxed"
	signer, err := dkim.New(conf, this.privateKey)
	if err == nil {
		signed, err := signer.Sign([]byte(message.Body))
		if err == nil {
			message.Body = string(signed)
		} else {
			WarnWithErr(err)
		}
	} else {
		WarnWithErr(err)
	}
}

// отправляет письмо, если возникает ошибка при выполнении какой либо команды, возвращает письмо обратно в очередь
func (this *BaseMailerApplication) Send(event *SendEvent) {
	event.Message.App = this
	// передаем отправителю smtp клиента
	if event.Client == nil {
		ReturnMail(event.Message, errors.New(fmt.Sprintf("can't create client for mailer#%d", this.id)))
		return
	}
	worker := event.Client.Worker
	Info("mailer#%d receive mail#%d", this.id, event.Message.Id)
	Debug("mailer#%d receive smtp client#%d", this.id, event.Client.Id)

	err := worker.Mail(event.Message.Envelope)
	if err == nil {
		Debug("mailer#%d send command MAIL FROM: %s", this.id, event.Message.Envelope)
		err = worker.Rcpt(event.Message.Recipient)
		if err == nil {
			Debug("mailer#%d send command RCPT TO: %s", this.id, event.Message.Recipient)
			wc, err := worker.Data()
			if err == nil {
				Debug("mailer#%d send command DATA", this.id)
				_, err = fmt.Fprint(wc, event.Message.Body)
				if err == nil {
					err = wc.Close()
					Debug("%s", event.Message.Body)
					if err == nil {
						Debug("mailer#%d send command .", this.id)
						// стараемся слать письма через уже созданное соединение,
						// поэтому после отправки письма не закрываем соединение
						err = worker.Reset()
						if err == nil {
							Debug("mailer#%d send command RSET", this.id)
							Info("mailer#%d send mail#%d", this.id, event.Message.Id)
							// разгружаем отправителя
							this.messagesCounts[event.Message.HostnameTo]--
							// для статы
							atomic.AddInt64(&mailsPerMinute, 1)
							// отпускаем поток получателя сообщений из очереди
							event.Message.Done <- true
						} else {
							ReturnMail(event.Message, err)
						}
					} else {
						ReturnMail(event.Message, err)
					}
				} else {
					ReturnMail(event.Message, err)
				}
			} else {
				ReturnMail(event.Message, err)
			}
		} else {
			ReturnMail(event.Message, err)
		}
	} else {
		ReturnMail(event.Message, err)
	}
	// говорим, что соединение свободно, его можно передать другому отправителю
	event.Client.Status = SMTP_CLIENT_STATUS_WAITING
}

func (this *BaseMailerApplication) SetDkimSelector(dkimSelector string) {
	this.dkimSelector = dkimSelector
}
