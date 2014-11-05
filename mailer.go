package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"net/url"
	"github.com/streadway/amqp"
	"regexp"
	"strings"
	"fmt"
	"time"
	"net"
	"errors"
	"bytes"
	"crypto/sha1"
	"io"
	"github.com/eaigner/dkim"
	"io/ioutil"
	"sync/atomic"
	"sync"
	"runtime/debug"
)

var (
	emailRegexp = regexp.MustCompile(`^[\w\d\.\_\%\+\-]+@([\w\d\.\-]+\.\w{2,4})$`)
	defaultHeaders = map[string]func(*MailMessage) string {
		"Return-Path"              : getReturnPath,
		"MIME-Version"             : getMimeVersion,
		"From"                     : getFrom,
		"To"                       : getTo,
		"Reply-To"                 : getReplyTo,
		"Date"                     : getDate,
		"Subject"                  : getSubject,
		"Content-Type"             : getContentType,
		"Content-Transfer-Encoding": getContentTransferEncoding,
		"Message-ID"               : getMessageId,
	}
	mailsCount int64
	mailer     *Mailer
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

func getReplyTo(message *MailMessage) string {
	return message.Envelope
}

func getDate(message *MailMessage) string {
	return fmt.Sprintf("%s (PDT)", message.CreatedDate.Format(time.RFC1123Z))
}

func getSubject(message *MailMessage) string {
	return "=?utf-8?B?No subject?="
}

func getContentType(message *MailMessage) string {
	return "text/plain; charset=utf-8"
}

func getContentTransferEncoding(message *MailMessage) string {
	return "7bit"
}

func getMessageId(message *MailMessage) string {
	hash := sha1.New()
	io.WriteString(hash, message.Body)
	return fmt.Sprintf("%x.%d@%s", hash.Sum(nil), time.Now().UnixNano(), message.HostnameFrom)
}

type MailMessage struct {
	Id           int64
	Envelope     string        `json:"envelope"`
	Recipient    string        `json:"recipient"`
	Body         string        `json:"body"`
	Delivery     amqp.Delivery
	Done         chan bool
	HostnameFrom string
	HostnameTo   string
	CreatedDate  time.Time
}

func (this *MailMessage) Init() {
	atomic.AddInt64(&mailsCount, 1)
	this.Id = atomic.LoadInt64(&mailsCount)
	this.Done = make(chan bool)
	this.CreatedDate = time.Now()
	if hostname, err := this.getHostnameFromEmail(this.Envelope); err == nil {
		this.HostnameFrom = hostname
	}
	if hostname, err := this.getHostnameFromEmail(this.Recipient); err == nil {
		this.HostnameTo = hostname
	}
}

func (this *MailMessage) getHostnameFromEmail(email string) (string, error) {
	matches := emailRegexp.FindAllStringSubmatch(email, -1)
	if len(matches) == 1 && len(matches[0]) == 2 {
		return matches[0][1], nil
	} else {
		Warn("can't receive hostname from %s", email)
		return "", errors.New("invalid email address")
	}
}

type Mailer struct {
	AppsConfigs        []*MailerApplicationConfig `yaml:"mailers"`
	PrivateKeyFilename string                     `yaml:"privateKey"`
	messages           chan *MailMessage
	apps               []MailerApplication
	sendServices       []SendService
	mutex              *sync.Mutex
}

func NewMailer() *Mailer {
	if mailer == nil {
		mailer = new(Mailer)
	}
	return mailer
}

func (this *Mailer) OnRegister() {}

func (this *Mailer) OnInit(event *InitEvent) {
	var privateKey []byte
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		Info("read private key file...")
		Debug("%s", this.PrivateKeyFilename)
		privateKey, err = ioutil.ReadFile(this.PrivateKeyFilename)
		if err == nil {
			Info("...success")
		} else {
			FailExitWithErr(err)
		}
		dkim.StdSignableHeaders = []string{
			"Return-Path",
			"MIME-Version",
			"From",
			"To",
			"Reply-To",
			"Date",
			"Subject",
			"Content-Type",
			"Content-Transfer-Encoding",
			"Message-ID",
		}
		Info("init mailers apps...")
		this.mutex = new(sync.Mutex)
		this.messages = make(chan *MailMessage)
		this.apps = make([]MailerApplication, 0)
		this.sendServices = []SendService{
			NewConnector(),
			NewMailer(),
		}
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
						app.SetPrivateKey(privateKey)
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
		event.MailersCount = len(this.apps)
	} else {
		FailExitWithErr(err)
	}
}

func (this *Mailer) OnRun() {
	Info("run mailers apps...")
	go func() {
		for message := range this.messages {
			go this.triggerSendEvent(message)
		}
	}()
	go func() {
		for _, app := range this.apps {
			go this.runApp(app)
		}
	}()
}

func (this *Mailer) triggerSendEvent(message *MailMessage) {
	event := new(SendEvent)
	event.Message = message
	for _, service := range this.sendServices {
		service.OnSend(event)
	}
}

func (this *Mailer) runApp(app MailerApplication) {
	for event := range app.Channel() {
		if app.IsValidMessage(event.Message) {
			app.PrepareMail(event.Message)
			app.CreateDkim(event.Message)
			app.Send(event)
		}
	}
}

func (this *Mailer) OnSend(event *SendEvent) {
	this.mutex.Lock()
	index := -1
	var count int64 = -1
	for i, app := range this.apps {
		messageCount := app.MessagesCountByHostname(event.Message.HostnameTo)
		Debug("mailer#%d, messages count - %d", i, messageCount)
		if count == -1 || count > messageCount {
			count = messageCount
			index = i
		}
	}
	if 0 <= index && index <= len(this.apps) {
		app := this.apps[index]
		Debug("send message to mailer#%d", index)
		app.IncrMessagesCountByHostname(event.Message.HostnameTo)
		app.Channel() <- event
	}
	this.mutex.Unlock()
}

func (this *Mailer) OnFinish(event *FinishEvent) {
	event.Group.Done()
}

func SendMail(message *MailMessage) {
	mailer.messages <- message
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
	IncrMessagesCountByHostname(string)
	MessagesCountByHostname(string) int64
	SetPrivateKey([]byte)
	Channel() chan *SendEvent
	PrepareMail(message *MailMessage)
	CreateDkim(message *MailMessage)
	Send(event *SendEvent)
	IsValidMessage(message *MailMessage) bool
}

type MailerApplicationType string

const (
	MAILER_APPLICATION_TYPE_MTA  MailerApplicationType = "mta"
	MAILER_APPLICATION_TYPE_SMTP                       = "smtp"
	MAILER_APPLICATION_TYPE_LOCAL                      = "local"
)

var (
	mailerConstructs = map[MailerApplicationType]func() MailerApplication {
		MAILER_APPLICATION_TYPE_MTA  : NewMtaMailerApplication,
//		MAILER_APPLICATION_TYPE_SMTP : NewSmtpMailerApplication,
//		MAILER_APPLICATION_TYPE_LOCAL: NewLocalMailerApplication,
	}
)

type AbstractMailerApplication struct {
	id             int
	messagesCounts map[string]int64
	events         chan *SendEvent
	config         *MailerApplicationConfig
	uri            *url.URL
	multipleSend   bool
	privateKey     []byte
}

func (this *AbstractMailerApplication) GetId() int {
	return this.id
}

func (this *AbstractMailerApplication) SetId(id int) {
	this.id = id
}

func (this *AbstractMailerApplication) MessagesCountByHostname(hostname string) int64 {
	if _, ok := this.messagesCounts[hostname]; !ok {
		this.messagesCounts[hostname] = 0
	}
	return this.messagesCounts[hostname]
}

func (this *AbstractMailerApplication) SetPrivateKey(privateKey []byte) {
	this.privateKey = privateKey
}

func (this *AbstractMailerApplication) Channel() chan *SendEvent {
	return this.events
}

func (this *AbstractMailerApplication) Init(config *MailerApplicationConfig) {
	var err error
	this.multipleSend = false
	this.config = config
	this.messagesCounts = make(map[string]int64)
	this.events = make(chan *SendEvent)
	this.uri, err = url.Parse(config.URI)
	if err == nil {
		Debug("url parsed %v", this.uri)
	} else {
		FailExit("can't parse url %s", config.URI)
	}
}

func (this *AbstractMailerApplication) IncrMessagesCountByHostname(hostname string) {
	if _, ok := this.messagesCounts[hostname]; ok {
		this.messagesCounts[hostname]++
	} else {
		this.messagesCounts[hostname] = 1
	}
}

func (this *AbstractMailerApplication) IsValidMessage(message *MailMessage) bool {
	return emailRegexp.MatchString(message.Envelope) && emailRegexp.MatchString(message.Recipient)
}

func (this *AbstractMailerApplication) returnMessageToQueueWithErr(message *MailMessage, err error) {
	message.Done <- false
	Warn("mailer#%d error - %v\n%s", this.id, err, debug.Stack())
}

//func (this *AbstractMailerApplication) send(client *smtp.Client, message *MailMessage) {
//	err := client.Mail(message.Envelope)
//	if err == nil {
//		Debug("MAIL FROM: %s", message.Envelope)
//		err = client.Rcpt(message.Recipient)
//		if err == nil {
//			Debug("RCPT TO: %s", message.Recipient)
//			wc, err := client.Data()
//			if err == nil {
//				Debug("DATA")
//				_, err = fmt.Fprintf(wc, message.Body)
//				if err == nil {
////					Debug("%s", message.Body)
//					err = wc.Close()
//					if err == nil {
//						Debug(".")
//						if this.multipleSend {
//							err = client.Reset()
//						} else {
//							err = client.Quit()
//						}
//						if err == nil {
//							if this.multipleSend {
//								Debug("RSET")
//							} else {
//								Debug("QUIT")
//							}
//							Info("mailer#%d send mail#%d to mta", this.id, message.Id)
////							this.messagesCounts[message.HostnameTo]--
//							message.Done <- true
//						} else {
//							this.returnMessageToQueueWithErr(message, err)
//						}
//					} else {
//						this.returnMessageToQueueWithErr(message, err)
//					}
//				} else {
//					this.returnMessageToQueueWithErr(message, err)
//				}
//			} else {
//				this.returnMessageToQueueWithErr(message, err)
//			}
//		} else {
//			this.returnMessageToQueueWithErr(message, err)
//		}
//	} else {
//		this.returnMessageToQueueWithErr(message, err)
//	}
//}

func (this *AbstractMailerApplication) PrepareMail(message *MailMessage) {
	var head, body string
	parts := strings.SplitN(message.Body, "\r\n\r\n", 2);
	if len(parts) == 2 {
		head = parts[0]
		body = parts[1]
	} else {
		body = parts[0]
	}

	preparedHeaders := make(map[string]string)
	rawHeaders := strings.Split(head, "\r\n")
	for _, rawHeader := range rawHeaders {
		var key, value string
		rawHeaderParts := strings.Split(rawHeader, ":")
		key = strings.TrimSpace(rawHeaderParts[0])
		if len(rawHeaderParts) == 2 {
			value = strings.TrimSpace(rawHeaderParts[1])
		}
		if len(key) > 0 && len(value) > 0 {
			preparedHeaders[key] = value
		}
	}
	for key, fun := range defaultHeaders {
		if _, ok := preparedHeaders[key]; !ok {
			preparedHeaders[key] = fun(message)
		}
	}
	buf := new(bytes.Buffer)
	for key, value := range preparedHeaders {
		buf.WriteString(key)
		buf.WriteString(": ")
		buf.WriteString(value)
		buf.WriteString("\r\n")
	}
	message.Body = fmt.Sprintf("%s\r\n%s\r\n", buf.String(), body)
}

func (this *AbstractMailerApplication) CreateDkim(message *MailMessage) {
	conf, err := dkim.NewConf(message.HostnameFrom, "dkim")
	if err != nil {
		WarnWithErr(err)
	}
	conf[dkim.CanonicalizationKey] = "relaxed/relaxed"
	conf[dkim.TimestampKey] = fmt.Sprint(message.CreatedDate.Unix())
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

//type SmtpMailerApplication struct {
//	AbstractMailerApplication
//	auth smtp.Auth
//}
//
//func NewSmtpMailerApplication() MailerApplication {
//	return new(SmtpMailerApplication)
//}
//
//func (this *SmtpMailerApplication) Init(config *MailerApplicationConfig) {
//	this.AbstractMailerApplication.Init(config)
//	if len(config.Username) > 0 && len(config.Password) > 0 {
//		hostname, _, err := net.SplitHostPort(this.uri.Host)
//		if err == nil {
//			this.auth = smtp.PlainAuth("", config.Username, config.Password, hostname)
//		} else {
//			WarnWithErr(err)
//		}
//	}
//}
//
//func (this *SmtpMailerApplication) Send(message *MailMessage) {
//	Info("smtp mailer#%d receive mail#%d", this.id, message.Id)
//	client, err := smtp.Dial(this.uri.Host)
//	if err == nil {
//		this.send(client, message)
//	} else {
//		this.returnMessageToQueueWithErr(message, err)
//	}
//}

//type LocalMailerApplication struct {
//	AbstractMailerApplication
//}
//
//func NewLocalMailerApplication() MailerApplication {
//	return new(LocalMailerApplication)
//}
//
//func (this *LocalMailerApplication) Send(message *MailMessage) {
//	Info("local mailer#%d receive mail#%d", this.id, message.Id)
//	client, err := smtp.Dial(this.uri.Host)
//	if err == nil {
//		this.send(client, message)
//	} else {
//		this.returnMessageToQueueWithErr(message, err)
//	}
//}

type MtaMailerApplication struct {
	AbstractMailerApplication
	hostname string
}

func NewMtaMailerApplication() MailerApplication {
	return new(MtaMailerApplication)
}

func (this *MtaMailerApplication) Init(config *MailerApplicationConfig) {
	this.AbstractMailerApplication.Init(config)
	this.multipleSend = true
	hostname, _, err := net.SplitHostPort(this.uri.Host)
	if err == nil {
		this.hostname = hostname
	} else {
		WarnWithErr(err)
	}
}

func (this *MtaMailerApplication) Send(event *SendEvent) {
	client := event.Client.client
	Debug("mailer#%d receive message#%d", this.id, event.Message.Id)
	Debug("mailer#%d receive smtp client#%d", this.id, event.Client.Id)

	err := client.Mail(event.Message.Envelope)
	if err == nil {
		Debug("mailer#%d send command MAIL FROM: %s", this.id, event.Message.Envelope)
		err = client.Rcpt(event.Message.Recipient)
		if err == nil {
			Debug("mailer#%d send command RCPT TO: %s", this.id, event.Message.Recipient)
			wc, err := client.Data()
			if err == nil {
				Debug("mailer#%d send command DATA", this.id)
				_, err = fmt.Fprintf(wc, event.Message.Body)
				if err == nil {
//					Debug("%s", event.Message.Body)
					err = wc.Close()
					if err == nil {
						Debug("mailer#%d send command .", this.id)
						err = client.Reset()
						if err == nil {
							Debug("mailer#%d send command RSET", this.id)
							Info("mailer#%d send mail#%d to mta", this.id, event.Message.Id)
							this.messagesCounts[event.Message.HostnameTo]--
							event.Message.Done <- true
						} else {
							this.returnMessageToQueueWithErr(event.Message, err)
						}
					} else {
						this.returnMessageToQueueWithErr(event.Message, err)
					}
				} else {
					this.returnMessageToQueueWithErr(event.Message, err)
				}
			} else {
				this.returnMessageToQueueWithErr(event.Message, err)
			}
		} else {
			this.returnMessageToQueueWithErr(event.Message, err)
		}
	} else {
		this.returnMessageToQueueWithErr(event.Message, err)
	}
	event.Client.Status = SMTP_CLIENT_STATUS_WAITING
}
