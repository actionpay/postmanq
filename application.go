package postmanq

import (
	"runtime"
	"io/ioutil"
	"time"
	"crypto/x509"
)

const (
	EXAMPLE_CONFIG_YAML = "/path/to/config/file.yaml"
)

var (
	app *Application
	defaultWorkersCount = runtime.NumCPU()
)

func init() {
	runtime.GOMAXPROCS(defaultWorkersCount)
}

// Событие инициализации. Вызывается после регистрации сервисов и содержит данные для настройки сервисов.
type InitEvent struct {
	Data         []byte // данные с настройками каждого сервиса
	MailersCount int    // количество потоков, отправляющих письма
}

// Программа отправки почты получилась довольно сложной, т.к. она выполняет обработку и отправку писем,
// работает с диском и с сетью, ведет логирование и проверяет ограничения перед отправкой.
// Из - за такого насыщенного функционала, было принято решение разбить программу на логические части - сервисы.
// Сервис - это модуль программы, отвечающий за выполнение одной конкретной задачи, например логирование.
// Сервис может сам выполнять эту задачу, либо управлять выполнением задачи.
type Service interface {
	OnInit(*InitEvent)
	OnRun()
	OnFinish()
}

type ReportService interface {
	OnInit(*InitEvent)
	OnShowReport()
}

type PublishService interface {
	OnInit(*InitEvent)
	OnPublish()
}

type GrepService interface {
	OnInit(*InitEvent)
	OnGrep()
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
	APPLICATION_EVENT_KIND_REPORT
	APPLICATION_EVENT_KIND_SEARCH
	APPLICATION_EVENT_KIND_PUBLISH
	APPLICATION_EVENT_KIND_GREP
)

// событие приложения
type ApplicationEvent struct {
	kind ApplicationEventKind
}

// создает событие с указанным типом
func NewApplicationEvent(kind ApplicationEventKind) *ApplicationEvent {
	return &ApplicationEvent{kind: kind}
}

// приложение
type Application struct {
	ConfigFilename  string                          // путь до конфигурационного файла
	sendingServices []Service                       // сервисы приложения, отправляющие письма
	reportServices  []ReportService                 // сервисы приложения, анализирующие очередь писем с ошибками
	publishServices []PublishService                // сервисы приложения, перекладывающие сообщения с ошибкой в очередь для отправки
	searchServices  []GrepService                   // сервисы приложения, ищущие по логу
	events          chan *ApplicationEvent          // канал событий приложения
	done            chan bool                       // флаг, сигнализирующий окончание работы приложения
	handlers        map[ApplicationEventKind]func() // обработчики событий приложения
}

// создает приложение
func NewApplication() *Application {
	if app == nil {
		app = new(Application)
		// создаем канал ожидания окончания программы
		app.done = make(chan bool)
	}
	return app
}

func (this *Application) IsValidConfigFilename(filename string) bool {
	return len(filename) > 0 && filename != EXAMPLE_CONFIG_YAML
}

// инициализирует сервисы приложения
func (this *Application) initSendingServices() {
	this.initServices(func(event *InitEvent) {
		for _, service := range this.sendingServices {
			service.OnInit(event)
		}
		this.events <- NewApplicationEvent(APPLICATION_EVENT_KIND_RUN)
	})
}

func (this *Application) initServices(callback func(event *InitEvent)) {
	// пытаемся прочитать конфигурационный файл
	bytes, err := ioutil.ReadFile(this.ConfigFilename)
	if err == nil {
		// создаем событие инициализации
		event := new(InitEvent)
		event.Data = bytes
		// и оповещаем сервисы
		callback(event)
	} else {
		FailExit("application can't read configuration file, error -  %v", err)
	}
}

// запускает приложения
func (this *Application) runServices() {
	for _, service := range this.sendingServices {
		go service.OnRun()
	}
}

// останавливает приложения
func (this *Application) finishServices() {
	for _, service := range this.sendingServices {
		service.OnFinish()
	}
	time.Sleep(2 * time.Second)
	this.done <- true
}

// запускает и контролирует работу всего приложения
func (this *Application) Run() {
	// инициализируем сервисы приложения
	this.sendingServices = []Service{
		LoggerOnce(),
		LimiterOnce(),
		ConnectorOnce(),
		MailerOnce(),
		ConsumerOnce(),
	}

	this.handlers = map[ApplicationEventKind]func() {
		APPLICATION_EVENT_KIND_INIT    : this.initSendingServices,
		APPLICATION_EVENT_KIND_RUN     : this.runServices,
		APPLICATION_EVENT_KIND_FINISH  : this.finishServices,
	}

	this.run(APPLICATION_EVENT_KIND_INIT)
}

func (this *Application) run(kind ApplicationEventKind) {
	// создаем каналы для событий
	this.events = make(chan *ApplicationEvent, len(this.handlers))
	go func() {
		for {
			select {
			case event := <- this.events:
				if handler, ok := this.handlers[event.kind]; ok {
					handler()
				}
			}
		}
		close(this.events)
	}()
	this.events <- NewApplicationEvent(kind)
	<- this.done
}

func (this *Application) ShowFailReport() {
	this.reportServices = []ReportService {
		AnalyserOnce(),
		ConsumerOnce(),
	}

	this.handlers = map[ApplicationEventKind]func() {
		APPLICATION_EVENT_KIND_INIT  : this.initReportServices,
		APPLICATION_EVENT_KIND_REPORT: this.runReportServices,
	}

	this.run(APPLICATION_EVENT_KIND_INIT)
}

func (this *Application) initReportServices() {
	this.initServices(func(event *InitEvent) {
		for _, service := range this.reportServices {
			service.OnInit(event)
		}
		this.events <- NewApplicationEvent(APPLICATION_EVENT_KIND_REPORT)
	})
}

func (this *Application) runReportServices() {
	for _, service := range this.reportServices {
		go service.OnShowReport()
	}
}

func (this *Application) PublishFailMessages() {
	this.publishServices = []PublishService {
		ConsumerOnce(),
	}

	this.handlers = map[ApplicationEventKind]func() {
		APPLICATION_EVENT_KIND_INIT   : this.initPublishServices,
		APPLICATION_EVENT_KIND_PUBLISH: this.runPublishServices,
	}

	this.run(APPLICATION_EVENT_KIND_INIT)
}

func (this *Application) initPublishServices() {
	this.initServices(func(event *InitEvent) {
		for _, service := range this.publishServices {
			service.OnInit(event)
		}
		this.events <- NewApplicationEvent(APPLICATION_EVENT_KIND_PUBLISH)
	})
}

func (this *Application) runPublishServices() {
	for _, service := range this.publishServices {
		service.OnPublish()
	}
}
