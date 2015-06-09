package postmanq

import (
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"runtime"
	"time"
)

const (
	ExampleConfigYaml  = "/path/to/config/file.yaml"
	InvalidInputString = ""
	InvalidInputInt    = 0
)

var (
	app                 Application
	defaultWorkersCount = runtime.NumCPU()
	PrintUsage          = func(f *flag.Flag) {
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
	SuccessSendEventResult SendEventResult = iota
	OverlimitSendEventResult
	ErrorSendEventResult
	DelaySendEventResult
)

// Событие отправки письма
type SendEvent struct {
	Client           *SmtpClient    // объект, содержащий подключение и клиент для отправки писем
	CertPool         *x509.CertPool // пул сертификатов
	CertBytes        []byte         // для каждого почтового сервиса необходим подписывать сертификат, поэтому в событии храним сырые данные сертификата
	CertBytesLen     int            // длина сертификата, по если длина больше 0, тогда пытаемся отправлять письма через TLS
	Message          *MailMessage   // само письмо, полученное из очереди
	DefaultPrevented bool           // флаг, сигнализирующий обрабатывать ли событие
	CreateDate       time.Time      // дата создания необходима при получении подключения к почтовому сервису
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
	InitApplicationEventKind   ApplicationEventKind = iota // событие инициализации сервисов
	RunApplicationEventKind                                // событие запуска сервисов
	FinishApplicationEventKind                             // событие завершения сервисов
)

// событие приложения
type ApplicationEvent struct {
	kind ApplicationEventKind
	Data []byte
	args map[string]interface{}
}

func (e *ApplicationEvent) GetBoolArg(key string) bool {
	return e.args[key].(bool)
}

func (e *ApplicationEvent) GetIntArg(key string) int {
	return e.args[key].(int)
}

func (e *ApplicationEvent) GetStringArg(key string) string {
	return e.args[key].(string)
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
	configFilename string                 // путь до конфигурационного файла
	services       []interface{}          // сервисы приложения, отправляющие письма
	events         chan *ApplicationEvent // канал событий приложения
	done           chan bool              // флаг, сигнализирующий окончание работы приложения
}

func (a *AbstractApplication) IsValidConfigFilename(filename string) bool {
	return len(filename) > 0 && filename != ExampleConfigYaml
}

func (a *AbstractApplication) run(app Application, event *ApplicationEvent) {
	app.SetDone(make(chan bool))
	// создаем каналы для событий
	app.SetEvents(make(chan *ApplicationEvent, 3))
	go func() {
		for {
			select {
			case event := <-app.Events():
				if event.kind == InitApplicationEventKind {
					// пытаемся прочитать конфигурационный файл
					bytes, err := ioutil.ReadFile(a.configFilename)
					if err == nil {
						event.Data = bytes
					} else {
						FailExit("application can't read configuration file, error -  %v", err)
					}
				}

				for _, service := range app.Services() {
					switch event.kind {
					case InitApplicationEventKind:
						app.FireInit(event, service)
					case RunApplicationEventKind:
						app.FireRun(event, service)
					case FinishApplicationEventKind:
						app.FireFinish(event, service)
					}
				}

				switch event.kind {
				case InitApplicationEventKind:
					event.kind = RunApplicationEventKind
					app.Events() <- event
				case FinishApplicationEventKind:
					time.Sleep(2 * time.Second)
					app.Done() <- true
				}
			}
		}
		close(app.Events())
	}()
	app.Events() <- event
	<-app.Done()
}

func (a *AbstractApplication) SetConfigFilename(configFilename string) {
	a.configFilename = configFilename
}

func (a *AbstractApplication) SetEvents(events chan *ApplicationEvent) {
	a.events = events
}

func (a *AbstractApplication) Events() chan *ApplicationEvent {
	return a.events
}

func (a *AbstractApplication) SetDone(done chan bool) {
	a.done = done
}

func (a *AbstractApplication) Done() chan bool {
	return a.done
}

func (a *AbstractApplication) Services() []interface{} {
	return a.services
}

func (a *AbstractApplication) FireInit(event *ApplicationEvent, abstractService interface{}) {
	service := abstractService.(Service)
	service.OnInit(event)
}

func (a *AbstractApplication) Run() {}

func (a *AbstractApplication) RunWithArgs(args ...interface{}) {}

func (a *AbstractApplication) FireRun(event *ApplicationEvent, abstractService interface{}) {}

func (a *AbstractApplication) FireFinish(event *ApplicationEvent, abstractService interface{}) {}

type PostApplication struct {
	AbstractApplication
}

func NewPostApplication() Application {
	app = new(PostApplication)
	return app
}

func (a *PostApplication) Run() {
	a.services = []interface{}{
		LoggerOnce(),
		LimiterOnce(),
		ConnectorOnce(),
		MailerOnce(),
		ConsumerOnce(),
	}
	a.run(a, NewApplicationEvent(InitApplicationEventKind))

}

func (a *PostApplication) FireRun(event *ApplicationEvent, abstractService interface{}) {
	service := abstractService.(SendingService)
	go service.OnRun()
}

func (a *PostApplication) FireFinish(event *ApplicationEvent, abstractService interface{}) {
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

func (a *ReportApplication) Run() {
	a.services = []interface{}{
		AnalyserOnce(),
		ConsumerOnce(),
	}
	a.run(a, NewApplicationEvent(InitApplicationEventKind))
}

func (a *ReportApplication) FireRun(event *ApplicationEvent, abstractService interface{}) {
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

func (a *PublishApplication) RunWithArgs(args ...interface{}) {
	a.services = []interface{}{
		ConsumerOnce(),
	}

	event := NewApplicationEvent(InitApplicationEventKind)
	event.args = make(map[string]interface{})
	event.args["srcQueue"] = args[0]
	event.args["destQueue"] = args[1]
	event.args["host"] = args[2]
	event.args["code"] = args[3]
	event.args["envelope"] = args[4]
	event.args["recipient"] = args[5]

	a.run(a, event)
}

func (a *PublishApplication) FireRun(event *ApplicationEvent, abstractService interface{}) {
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

func (a *GrepApplication) RunWithArgs(args ...interface{}) {
	a.services = []interface{}{
		SearcherOnce(),
	}

	event := NewApplicationEvent(InitApplicationEventKind)
	event.args = make(map[string]interface{})
	event.args["envelope"] = args[0]
	event.args["recipient"] = args[1]
	event.args["numberLines"] = args[2]

	a.run(a, event)
}

func (a *GrepApplication) FireRun(event *ApplicationEvent, abstractService interface{}) {
	service := abstractService.(GrepService)
	go service.OnGrep(event)
}
