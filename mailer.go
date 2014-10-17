package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"net/url"
	"github.com/streadway/amqp"
	"regexp"
	"net/smtp"
)

var (
	emailRegexp = regexp.MustCompile(`^[\w\d\.\_\%\+\-]+@([\w\d\.\-]+\.\w{2,4})$`)
)

type MailMessage struct {
	Envelope  string `json:"envelope"`
	Recipient string `json:"recipient"`
	Body      string `json:"body"`
	Delivery  amqp.Delivery
}

type Mailer struct {
	AppsConfigs []*MailerApplicationConfig `yaml:"mailers"`
	apps        []MailerApplication
}

func NewMailer() *Mailer {
	return new(Mailer)
}

func (this *Mailer) OnRegister(event *RegisterEvent) {
	event.Group.Done()
}

func (this *Mailer) OnInit(event *InitEvent) {
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		this.apps = make([]MailerApplication, 0)
		for _, appConfig := range this.AppsConfigs {
			appUrl, err := url.Parse(appConfig.URI)
			if err == nil {
				appType := MailerApplicationType(appUrl.Scheme)
				if mailerConstruct, ok := mailerConstructs[appType]; ok {
					app := mailerConstruct()
					app.Init(appConfig)
					this.apps = append(this.apps, app)
				} else {
					Warn("mailer application with type %s not found", appType)
				}
			} else {
				FailExitWithErr(err)
			}
		}
		event.Mailers = this.apps
		event.Group.Done()
	} else {
		FailExitWithErr(err)
	}
}

func (this *Mailer) OnRun() {
	for _, app := range this.apps {
		go app.Run()
	}
}

func (this *Mailer) OnFinish(event *FinishEvent) {
	event.Group.Done()
}

type MailerApplicationConfig struct {
	URI string `yaml:"uri"`
}

type MailerApplication interface {
	Init(*MailerApplicationConfig)
	Run()
	IncrMessagesCount()
	MessagesCount() int64
	Channel() chan <- *MailMessage
}

type MailerApplicationType string

const (
	MAILER_APPLICATION_TYPE_MTA  MailerApplicationType = "mta"
	MAILER_APPLICATION_TYPE_SMTP                       = "smtp"
	MAILER_APPLICATION_TYPE_MX                         = "mx"
)

var (
	mailerConstructs = map[MailerApplicationType]func() MailerApplication {
		MAILER_APPLICATION_TYPE_SMTP: NewSmtpMailerApplication,
	}
)

type AbstractMailerApplication struct {
	messagesCount int64
	messages      chan *MailMessage
	config        *MailerApplicationConfig
}

func (this *AbstractMailerApplication) MessagesCount() int64 {
	return this.messagesCount
}

func (this *AbstractMailerApplication) Channel() chan <- *MailMessage {
	return this.messages
}

func (this *AbstractMailerApplication) Init(config *MailerApplicationConfig) {
	this.config = config
	this.messagesCount = 0
	this.messages = make(chan *MailMessage)
}

func (this *AbstractMailerApplication) IncrMessagesCount() {
	this.messagesCount++
}

type SmtpMailerApplication struct {
	AbstractMailerApplication
	uri  *url.URL
	auth *smtp.Auth
}

func NewSmtpMailerApplication() MailerApplication {
	return new(SmtpMailerApplication)
}

func (this *SmtpMailerApplication) Init(config *MailerApplicationConfig) {
	var err error
	this.AbstractMailerApplication.Init(config)
	this.uri, err = url.Parse(config.URI)
	if err == nil {
		Info("url parsed %v", this.uri)
		//		if this.uri.User != nil {
//			this.auth = smtp.CRAMMD5Auth(this.uri.User.Username(), this.uri.User.Password())
//		}
	} else {
		Warn("can't parse url %s", config.URI)
	}
}

func (this *SmtpMailerApplication) Run() {
	for message := range this.messages {
		if emailRegexp.MatchString(message.Envelope) && emailRegexp.MatchString(message.Recipient) {
//			Info("%v", emailRegexp.FindAllString(message.Recipient, -1))
//			Info("%v", emailRegexp.FindAllStringSubmatch(message.Recipient, -1))
		}
	}
}
