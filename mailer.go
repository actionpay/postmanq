package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"net/url"
	"github.com/streadway/amqp"
	"regexp"
	"net/smtp"
	"strings"
	"fmt"
	"time"
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
	URI      string        `yaml:"uri"`
	Username string        `yaml:"username"`
	Password string        `yaml:"password"`
	Timeout  time.Duration `yaml:"timeout"`

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
	MAILER_APPLICATION_TYPE_LOCAL                      = "local"
)

var (
	mailerConstructs = map[MailerApplicationType]func() MailerApplication {
		MAILER_APPLICATION_TYPE_SMTP : NewSmtpMailerApplication,
		MAILER_APPLICATION_TYPE_LOCAL: NewLocalMailerApplication,
	}
)

type AbstractMailerApplication struct {
	messagesCount int64
	messages      chan *MailMessage
	config        *MailerApplicationConfig
	uri           *url.URL
}

func (this *AbstractMailerApplication) MessagesCount() int64 {
	return this.messagesCount
}

func (this *AbstractMailerApplication) Channel() chan <- *MailMessage {
	return this.messages
}

func (this *AbstractMailerApplication) Init(config *MailerApplicationConfig) {
	var err error
	this.config = config
	this.messagesCount = 0
	this.messages = make(chan *MailMessage)
	this.uri, err = url.Parse(config.URI)
	if err == nil {
		Info("url parsed %v", this.uri)
	} else {
		Warn("can't parse url %s", config.URI)
	}
}

func (this *AbstractMailerApplication) IncrMessagesCount() {
	this.messagesCount++
}

func (this *AbstractMailerApplication) isValidMessage(message *MailMessage) bool {
	return emailRegexp.MatchString(message.Envelope) && emailRegexp.MatchString(message.Recipient)
}

type SmtpMailerApplication struct {
	AbstractMailerApplication
	auth   smtp.Auth
}

func NewSmtpMailerApplication() MailerApplication {
	return new(SmtpMailerApplication)
}

func (this *SmtpMailerApplication) Init(config *MailerApplicationConfig) {
	this.AbstractMailerApplication.Init(config)
	if len(config.Username) > 0 && len(config.Password) > 0 {
		hostParts := strings.Split(this.uri.Host, ":")
		this.auth = smtp.PlainAuth("", config.Username, config.Password, hostParts[0])
	}
}

func (this *SmtpMailerApplication) Run() {
	for message := range this.messages {
		if this.isValidMessage(message) {
			this.send(message)
		}
	}
}

func (this *SmtpMailerApplication) send(message *MailMessage) {
	connect, err := smtp.Dial(this.uri.Host)
	if err == nil {
		err = connect.Mail(message.Envelope)
		if err == nil {
			err = connect.Rcpt(message.Recipient)
			if err == nil {
				wc, err := connect.Data()
				if err == nil {
					_, err = fmt.Fprintf(wc, message.Body)
					if err != nil {
						WarnWithErr(err)
					}
					err = wc.Close()
					if err != nil {
						WarnWithErr(err)
					}
				} else {
					WarnWithErr(err)
				}
			} else {
				WarnWithErr(err)
			}
		} else {
			WarnWithErr(err)
		}
		err = connect.Quit()
		if err != nil {
			WarnWithErr(err)
		}
	} else {
		WarnWithErr(err)
	}
}

type LocalMailerApplication struct {
	AbstractMailerApplication
	client *smtp.Client
	timer  *time.Timer
}

func NewLocalMailerApplication() MailerApplication {
	return new(LocalMailerApplication)
}

func (this *LocalMailerApplication) Init(config *MailerApplicationConfig) {
	this.AbstractMailerApplication.Init(config)
	this.connect()
}

func (this *LocalMailerApplication) Run() {
	if this.client != nil {
		for message := range this.messages {
			this.timer.Reset(this.config.Timeout * time.Second)
			if this.isValidMessage(message) {
				err := this.client.Mail(message.Envelope)
				if err == nil {
					Info("MAIL FROM: %s", message.Envelope)
					err = this.client.Rcpt(message.Recipient)
					if err == nil {
						Info("RCPT TO: %s", message.Recipient)
						wc, err := this.client.Data()
						if err == nil {
							Info("DATA")
							_, err = fmt.Fprintf(wc, message.Body)
							if err == nil {
								Info("%s", message.Body)
								err = wc.Close()
								if err == nil {
									Info(".")
									err = this.client.Reset()
									if err == nil {
										Info("RSET")
										err := message.Delivery.Ack(true)
										if err == nil {
											Info("message is acknowledged")
										} else {
											WarnWithErr(err)
										}
									} else {
										WarnWithErr(err)
									}
								} else {
									WarnWithErr(err)
								}
							} else {
								WarnWithErr(err)
							}
						} else {
							WarnWithErr(err)
						}
					} else {
						WarnWithErr(err)
					}
				} else {
					WarnWithErr(err)
				}
			}
		}
	}
}

func (this *LocalMailerApplication) connect() {
	if this.client == nil {
		var err error
		this.client, err = smtp.Dial(this.uri.Host)
		if err == nil {
			Info("connect to %s", this.uri.Host)
			this.timer = time.AfterFunc(this.config.Timeout * time.Second, this.reconnect)
		} else {
			WarnWithErr(err)
		}
	}
}

func (this *LocalMailerApplication) reconnect() {
	if this.client != nil {
		err := this.client.Quit()
		if err != nil {
			WarnWithErr(err)
		}
		this.client = nil
		this.connect()
	}
}
