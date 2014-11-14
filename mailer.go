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
	"bytes"
	"github.com/eaigner/dkim"
	"io/ioutil"
	"sync/atomic"
	"sync"
)

var (
	emailRegexp = regexp.MustCompile(`^[\w\d\.\_\%\+\-]+@([\w\d\.\-]+\.\w{2,4})$`)
	defaultHeaders = map[string]func(*MailMessage) string {
		"Return-Path"              : getReturnPath,
		"MIME-Version"             : getMimeVersion,
		"From"                     : getFrom,
		"To"                       : getTo,
		"Date"                     : getDate,
		"Subject"                  : getSubject,
		"Content-Type"             : getContentType,
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
	AttemptCount int 		   `json:"attemptCount"`
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
	DkimSelector       string                     `yaml:"dkimSelector"`
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
			"From",
			"To",
			"Date",
			"Subject",
			"Content-Type",
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
			if appConfig.Handlers == 0 {
				appConfig.Handlers = 1
			}
			for i := 0; i < appConfig.Handlers; i++ {
				app := NewBaseMailerApplication()
				app.SetId(j * appConfig.Handlers + i)
				app.SetPrivateKey(privateKey)
				app.SetDkimSelector(this.DkimSelector)
				app.Init(appConfig)
				Info("create mailer app#%d", app.GetId())
				this.apps = append(this.apps, app)
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
	PrepareMail(*MailMessage)
	CreateDkim(*MailMessage)
	Send(*SendEvent)
	IsValidMessage(*MailMessage) bool
	SetDkimSelector(string)
}

var (
	CRLF = "\r\n"
)

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

func NewBaseMailerApplication() MailerApplication {
	return new(BaseMailerApplication)
}

func (this *BaseMailerApplication) GetId() int {
	return this.id
}

func (this *BaseMailerApplication) SetId(id int) {
	this.id = id
}

func (this *BaseMailerApplication) MessagesCountByHostname(hostname string) int64 {
	if _, ok := this.messagesCounts[hostname]; !ok {
		this.messagesCounts[hostname] = 0
	}
	return this.messagesCounts[hostname]
}

func (this *BaseMailerApplication) SetPrivateKey(privateKey []byte) {
	this.privateKey = privateKey
}

func (this *BaseMailerApplication) Channel() chan *SendEvent {
	return this.events
}

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

func (this *BaseMailerApplication) IncrMessagesCountByHostname(hostname string) {
	if _, ok := this.messagesCounts[hostname]; ok {
		this.messagesCounts[hostname]++
	} else {
		this.messagesCounts[hostname] = 1
	}
}

func (this *BaseMailerApplication) IsValidMessage(message *MailMessage) bool {
	return emailRegexp.MatchString(message.Envelope) && emailRegexp.MatchString(message.Recipient)
}

func (this *BaseMailerApplication) returnMessageToQueueWithErr(message *MailMessage, err error) {
	message.Done <- false
	Warn("mailer#%d error - %v", this.id, err)
}

func (this *BaseMailerApplication) PrepareMail(message *MailMessage) {
	var head, body string
	parts := strings.SplitN(message.Body, CRLF + CRLF, 2);
	if len(parts) == 2 {
		head = parts[0]
		body = parts[1]
	} else {
		body = parts[0]
	}

	preparedHeaders := make(map[string]string)
	rawHeaders := strings.Split(head, CRLF)
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
		buf.WriteString(CRLF)
	}
	buf.WriteString(CRLF)
	buf.WriteString(body)
	buf.WriteString(CRLF)
	message.Body = buf.String()
}

func (this *BaseMailerApplication) CreateDkim(message *MailMessage) {
	conf, err := dkim.NewConf(message.HostnameFrom, this.dkimSelector)
	if err != nil {
		WarnWithErr(err)
	}
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

func (this *BaseMailerApplication) Send(event *SendEvent) {
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
				_, err = fmt.Fprint(wc, event.Message.Body)
				if err == nil {
					err = wc.Close()
					Debug("%s", event.Message.Body)
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

func (this *BaseMailerApplication) SetDkimSelector(dkimSelector string) {
	this.dkimSelector = dkimSelector
}
