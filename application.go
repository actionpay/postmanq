package postmanq

import (
	"runtime"
	"io/ioutil"
	"time"
	"crypto/x509"
	"fmt"
	"flag"
)

const (
	EXAMPLE_CONFIG_YAML  = "/path/to/config/file.yaml"
	INVALID_INPUT_STRING = ""
	INVALID_INPUT_INT    = 0
)

var (
	app Application
	defaultWorkersCount = runtime.NumCPU()
	PrintUsage = func(f *flag.Flag) {
		format := "  -%s %s\n"
		fmt.Printf(format, f.Name, f.Usage)
	}
)

func init() {
	runtime.GOMAXPROCS(defaultWorkersCount)
}

// Программа отправки почты получилась довольно сложной, т.к. она выполняет обработку и отправку писем,
// работает с диском и с сетью, ведет логирование и проверяет ограничения перед отправкой.
// Из - за такого насыщенного функционала, было принято решение разбить программу на логические части - сервисы.
// Сервис - это модуль программы, отвечающий за выполнение одной конкретной задачи, например логирование.
// Сервис может сам выполнять эту задачу, либо управлять выполнением задачи.
type Service interface {
	OnInit(*ApplicationEvent)
}

type SendingService interface {
	Service
	OnRun()
	OnFinish()
}

type ReportService interface {
	Service
	OnShowReport()
}

type PublishService interface {
	Service
	OnPublish(*ApplicationEvent)
}

type GrepService interface {
	Service
	OnGrep(*ApplicationEvent)
}

type SendEventResult int

const (
	SEND_EVENT_RESULT_SUCCESS   SendEventResult = iota
	SEND_EVENT_RESULT_OVERLIMIT
	SEND_EVENT_RESULT_ERROR
	SEND_EVENT_RESULT_DELAY
)

// Событие отправки письма
type SendEvent struct {
	Client           *SmtpClient          // объект, содержащий подключение и клиент для отправки писем
	CertPool         *x509.CertPool       // пул сертификатов
	CertBytes        []byte               // для каждого почтового сервиса необходим подписывать сертификат, поэтому в событии храним сырые данные сертификата
	CertBytesLen     int                  // длина сертификата, по если длина больше 0, тогда пытаемся отправлять письма через TLS
	Message          *MailMessage         // само письмо, полученное из очереди
	DefaultPrevented bool                 // флаг, сигнализирующий обрабатывать ли событие
	CreateDate       time.Time            // дата создания необходима при получении подключения к почтовому сервису
	Result           chan SendEventResult
	MailServers      chan *MailServer
	MailServer       *MailServer
	TryCount         int
}

// отправляет письмо сервису отправки, который затем решит, какой поток будет отправлять письмо
func NewSendEvent(message *MailMessage) *SendEvent {
	Debug("create send event for message#%d", message.Id)
	event := new(SendEvent)
	event.DefaultPrevented = false
	event.Message = message
	event.CreateDate = time.Now()
	event.Result = make(chan SendEventResult)
	event.MailServers = make(chan *MailServer)
	return event
}

// тип гловального события приложения
type ApplicationEventKind int

const (
	APPLICATION_EVENT_KIND_INIT    ApplicationEventKind = iota // событие инициализации сервисов
	APPLICATION_EVENT_KIND_RUN                                 // событие запуска сервисов
	APPLICATION_EVENT_KIND_FINISH                              // событие завершения сервисов
)

// событие приложения
type ApplicationEvent struct {
	kind ApplicationEventKind
	Data []byte
	args map[string]interface{}
}

func (this *ApplicationEvent) GetBoolArg(key string) bool {
	return this.args[key].(bool)
}

func (this *ApplicationEvent) GetIntArg(key string) int {
	return this.args[key].(int)
}

func (this *ApplicationEvent) GetStringArg(key string) string {
	return this.args[key].(string)
}

// создает событие с указанным типом
func NewApplicationEvent(kind ApplicationEventKind) *ApplicationEvent {
	return &ApplicationEvent{kind: kind}
}

type Application interface {
	SetConfigFilename(string)
	IsValidConfigFilename(string) bool
	SetEvents(chan *ApplicationEvent)
	Events() chan *ApplicationEvent
	SetDone(chan bool)
	Done() chan bool
	Services() []interface{}
	FireInit(*ApplicationEvent, interface{})
	FireRun(*ApplicationEvent, interface{})
	FireFinish(*ApplicationEvent, interface{})
	Run()
	RunWithArgs(...interface{})
}

type AbstractApplication struct {
	configFilename  string                 // путь до конфигурационного файла
	services        []interface{}          // сервисы приложения, отправляющие письма
	events          chan *ApplicationEvent // канал событий приложения
	done            chan bool              // флаг, сигнализирующий окончание работы приложения
}

func (this *AbstractApplication) IsValidConfigFilename(filename string) bool {
	return len(filename) > 0 && filename != EXAMPLE_CONFIG_YAML
}

func (this *AbstractApplication) run(app Application, event *ApplicationEvent) {
	app.SetDone(make(chan bool))
	// создаем каналы для событий
	app.SetEvents(make(chan *ApplicationEvent, 3))
	go func() {
		for {
			select {
			case event := <- app.Events():
				if event.kind == APPLICATION_EVENT_KIND_INIT {
					// пытаемся прочитать конфигурационный файл
					bytes, err := ioutil.ReadFile(this.configFilename)
					if err == nil {
						event.Data = bytes
					} else {
						FailExit("application can't read configuration file, error -  %v", err)
					}
				}

				for _, service := range app.Services() {
					switch event.kind {
					case APPLICATION_EVENT_KIND_INIT: app.FireInit(event, service)
					case APPLICATION_EVENT_KIND_RUN: app.FireRun(event, service)
					case APPLICATION_EVENT_KIND_FINISH: app.FireFinish(event, service)
					}
				}

				switch event.kind {
				case APPLICATION_EVENT_KIND_INIT:
					event.kind = APPLICATION_EVENT_KIND_RUN
					app.Events() <- event
				case APPLICATION_EVENT_KIND_FINISH:
					time.Sleep(2 * time.Second)
					app.Done() <- true
				}
			}
		}
		close(app.Events())
	}()
	app.Events() <- event
	<- app.Done()
}

func (this *AbstractApplication) SetConfigFilename(configFilename string) {
	this.configFilename = configFilename
}

func (this *AbstractApplication) SetEvents(events chan *ApplicationEvent) {
	this.events = events
}

func (this *AbstractApplication) Events() chan *ApplicationEvent {
	return this.events
}

func (this *AbstractApplication) SetDone(done chan bool) {
	this.done = done
}

func (this *AbstractApplication) Done() chan bool {
	return this.done
}

func (this *AbstractApplication) Services() []interface{} {
	return this.services
}

func (this *AbstractApplication) FireInit(event *ApplicationEvent, abstractService interface{}) {
	service := abstractService.(Service)
	service.OnInit(event)
}

func (this *AbstractApplication) Run() {}

func (this *AbstractApplication) RunWithArgs(args ...interface{}) {}

func (this *AbstractApplication) FireRun(event *ApplicationEvent, abstractService interface{}) {}

func (this *AbstractApplication) FireFinish(event *ApplicationEvent, abstractService interface{}) {}

type PostApplication struct {
	AbstractApplication
}

func NewPostApplication() Application {
	app = new(PostApplication)
	return app
}

func (this *PostApplication) Run() {
	this.services = []interface{} {
		LoggerOnce(),
		LimiterOnce(),
		ConnectorOnce(),
		MailerOnce(),
		ConsumerOnce(),
	}
	this.run(this, NewApplicationEvent(APPLICATION_EVENT_KIND_INIT))

}

func (this *PostApplication) FireRun(event *ApplicationEvent, abstractService interface{}) {
	service := abstractService.(SendingService)
	go service.OnRun()
}

func (this *PostApplication) FireFinish(event *ApplicationEvent, abstractService interface{}) {
	service := abstractService.(SendingService)
	go service.OnFinish()
}

type ReportApplication struct {
	AbstractApplication
}

func NewReportApplication() Application {
	app = new(ReportApplication)
	return app
}

func (this *ReportApplication) Run() {
	this.services = []interface{} {
		AnalyserOnce(),
		ConsumerOnce(),
	}
	this.run(this, NewApplicationEvent(APPLICATION_EVENT_KIND_INIT))
}

func (this *ReportApplication) FireRun(event *ApplicationEvent, abstractService interface{}) {
	service := abstractService.(ReportService)
	go service.OnShowReport()
}

type PublishApplication struct {
	AbstractApplication
}

func NewPublishApplication() Application {
	app = new(PublishApplication)
	return app
}

func (this *PublishApplication) RunWithArgs(args ...interface{}) {
	this.services = []interface{} {
		ConsumerOnce(),
	}

	event := NewApplicationEvent(APPLICATION_EVENT_KIND_INIT)
	event.args = make(map[string]interface{})
	event.args["srcQueue"] = args[0]
	event.args["destQueue"] = args[1]
	event.args["host"] = args[2]
	event.args["code"] = args[3]
	event.args["envelope"] = args[4]
	event.args["recipient"] = args[5]

	this.run(this, event)
}

func (this *PublishApplication) FireRun(event *ApplicationEvent, abstractService interface{}) {
	service := abstractService.(PublishService)
	go service.OnPublish(event)
}

type GrepApplication struct {
	AbstractApplication
}

func NewGrepApplication() Application {
	app = new(GrepApplication)
	return app
}

func (this *GrepApplication) RunWithArgs(args ...interface{}) {
	this.services = []interface{} {
		SearcherOnce(),
	}

	event := NewApplicationEvent(APPLICATION_EVENT_KIND_INIT)
	event.args = make(map[string]interface{})
	event.args["envelope"] = args[0]
	event.args["recipient"] = args[1]
	event.args["numberLines"] = args[2]

	this.run(this, event)
}

func (this *GrepApplication) FireRun(event *ApplicationEvent, abstractService interface{}) {
	service := abstractService.(GrepService)
	go service.OnGrep(event)
}








