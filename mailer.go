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
	Id        int64
	Envelope  string `json:"envelope"`
	Recipient string `json:"recipient"`
	Body      string `json:"body"`
	Delivery  amqp.Delivery
	Done      chan bool
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
		Info("init mailers apps...")
		this.apps = make([]MailerApplication, 0)
		for j, appConfig := range this.AppsConfigs {
			appUrl, err := url.Parse(appConfig.URI)
			if err == nil {
				appType := MailerApplicationType(appUrl.Scheme)
				if mailerConstruct, ok := mailerConstructs[appType]; ok {
					if appConfig.Handlers == 0 {
						appConfig.Handlers = 1
					}
					for i := 0; i < appConfig.Handlers; i++ {
						app := mailerConstruct()
						app.SetId(j * appConfig.Handlers + i)
						app.Init(appConfig)
						Info("create mailer app#%d", app.GetId())
						this.apps = append(this.apps, app)
					}
				} else {
					Warn("mailer application with type %s not found", appType)
				}
			} else {
				FailExitWithErr(err)
			}
		}
		event.Mailers = this.apps
		event.MailersCount = len(this.apps)
		event.Group.Done()
	} else {
		FailExitWithErr(err)
	}
}

func (this *Mailer) OnRun() {
	Info("run mailers apps...")
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
	Handlers int           `yaml:"handlers"`
}

type MailerApplication interface {
	Init(*MailerApplicationConfig)
	GetId() int
	SetId(int)
	Run()
	IncrMessagesCount()
	MessagesCount() int64
	Channel() chan <- *MailMessage
}

type MailerApplicationType string

const (
	MAILER_APPLICATION_TYPE_MTA  MailerApplicationType = "mta"
	MAILER_APPLICATION_TYPE_SMTP                       = "smtp"
	MAILER_APPLICATION_TYPE_LOCAL                      = "local"
)

var (
	mailerConstructs = map[MailerApplicationType]func() MailerApplication {
		MAILER_APPLICATION_TYPE_SMTP : NewSmtpMailerApplication,
		MAILER_APPLICATION_TYPE_LOCAL: NewLocalMailerApplication,
	}
)

type AbstractMailerApplication struct {
	id            int
	messagesCount int64
	messages      chan *MailMessage
	config        *MailerApplicationConfig
	uri           *url.URL
}

func (this *AbstractMailerApplication) GetId() int {
	return this.id
}

func (this *AbstractMailerApplication) SetId(id int) {
	this.id = id
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
		Debug("url parsed %v", this.uri)
	} else {
		FailExit("can't parse url %s", config.URI)
	}
}

func (this *AbstractMailerApplication) IncrMessagesCount() {
	this.messagesCount++
}

func (this *AbstractMailerApplication) isValidMessage(message *MailMessage) bool {
	return emailRegexp.MatchString(message.Envelope) && emailRegexp.MatchString(message.Recipient)
}

func (this *AbstractMailerApplication) returnMessageToQueueWithErr(message *MailMessage, err error) {
	message.Done <- false
	WarnWithErr(err)
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
	Info("run smtp mailer app%d", this.id)
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
}

func NewLocalMailerApplication() MailerApplication {
	return new(LocalMailerApplication)
}

func (this *LocalMailerApplication) Run() {
	Info("run local mailer app#%d", this.id)
	for message := range this.messages {
		if this.isValidMessage(message) {
			Info("local mailer#%d receive mail#%d", this.id, message.Id)
			client, err := smtp.Dial(this.uri.Host)
			err = client.Mail(message.Envelope)
			if err == nil {
				Debug("MAIL FROM: %s", message.Envelope)
				err = client.Rcpt(message.Recipient)
				if err == nil {
					Debug("RCPT TO: %s", message.Recipient)
					wc, err := client.Data()
					if err == nil {
						Debug("DATA")
						_, err = fmt.Fprintf(wc, message.Body)
						if err == nil {
							Debug("%s", message.Body)
							err = wc.Close()
							if err == nil {
								Debug(".")
								err = client.Quit()
								if err == nil {
									Debug("QUIT")
									Info("local mailer#%d send mail#%d to mta", this.id, message.Id)
									message.Done <- true
								} else {
									this.returnMessageToQueueWithErr(message, err)
								}
							} else {
								this.returnMessageToQueueWithErr(message, err)
							}
						} else {
							this.returnMessageToQueueWithErr(message, err)
						}
					} else {
						this.returnMessageToQueueWithErr(message, err)
					}
				} else {
					this.returnMessageToQueueWithErr(message, err)
				}
			} else {
				this.returnMessageToQueueWithErr(message, err)
			}
		} else {
			this.returnMessageToQueueWithErr(message, nil)
		}
	}
}
