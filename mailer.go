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
		for _, appConfig := range this.AppsConfigs {
			appUrl, err := url.Parse(appConfig.URI)
			if err == nil {
				appType := MailerApplicationType(appUrl.Scheme)
				if _, ok := mailerApps[appType]; ok {

				} else {
					Warn("mailer application with type %s not found", appType)
				}
			} else {
				FailExitWithErr(err)
			}
		}
	} else {
		FailExitWithErr(err)
	}
}

func (this *Mailer) OnFinish(event *FinishEvent) {
	event.Group.Done()
}

type MailerApplicationConfig struct {
	URI string `yaml:"uri"`
}

type MailerApplication interface {
	MessagesCount() int
	Channel() chan <- *MailMessage
	Run(*MailerApplicationConfig)
}

type MailerApplicationType string

const (
	MAILER_APPLICATION_TYPE_MTA  MailerApplicationType = "mta"
	MAILER_APPLICATION_TYPE_SMTP                       = "smtp"
	MAILER_APPLICATION_TYPE_MX                         = "mx"
)

var (
	mailerApps = map[MailerApplicationType]MailerApplication {
		MAILER_APPLICATION_TYPE_SMTP: new(SmtpMailerApplication),
	}
)

type AbstractMailerApplication struct {
	messagesCount int
	messages      chan *MailMessage
}

func (this *AbstractMailerApplication) MessagesCount() int {
	return this.messagesCount
}

func (this *AbstractMailerApplication) Channel() chan <- *MailMessage {
	return this.messages
}

type SmtpMailerApplication struct {
	AbstractMailerApplication
}

func (this *SmtpMailerApplication) Run(*MailerApplicationConfig) {

}
