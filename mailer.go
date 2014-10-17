package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"net/url"
)

type MailMessage struct {
	Envelope  string `json:"envelope"`
	Recipient string `json:"recipient"`
	Body      string `json:"body"`
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
					app.Init()
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
	Init()
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
}

func (this *AbstractMailerApplication) MessagesCount() int64 {
	return this.messagesCount
}

func (this *AbstractMailerApplication) Channel() chan <- *MailMessage {
	return this.messages
}

func (this *AbstractMailerApplication) Init() {
	this.messagesCount = 0
	this.messages = make(chan *MailMessage)
}

func (this *AbstractMailerApplication) IncrMessagesCount() {
	this.messagesCount++
}

type SmtpMailerApplication struct {
	AbstractMailerApplication
}

func NewSmtpMailerApplication() MailerApplication {
	return new(SmtpMailerApplication)
}

func (this *SmtpMailerApplication) Run() {
	for message := range this.messages {
		if message != nil {}
//		Info("messages count: %d", this.messagesCount)
//		time.Sleep(16 * time.Second)
//		atomic.AddInt64(&this.messagesCount, 1)

//		Info("%v", message)
	}
}
