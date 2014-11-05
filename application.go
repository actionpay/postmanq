package postmanq

import (
	"sync"
	"runtime"
	"io/ioutil"
	"time"
)

const (
	EXAMPLE_CONFIG_YAML = "/path/to/config/file.yaml"
)

var (
	app *Application
)

type InitEvent struct {
	Data         []byte
	MailersCount int
}

type FinishEvent struct {
	Group *sync.WaitGroup
}

type Service interface {
	OnRegister()
	OnInit(*InitEvent)
	OnRun()
	OnFinish(*FinishEvent)
}

type SendEvent struct {
	Client  *SmtpClient
	Message *MailMessage
}

type SendService interface {
	OnSend(*SendEvent)
}

type ApplicationEventKind int

const (
	APPLICATION_EVENT_KIND_REGISTER ApplicationEventKind = iota
	APPLICATION_EVENT_KIND_INIT
	APPLICATION_EVENT_KIND_RUN
	APPLICATION_EVENT_KIND_FINISH
)

type ApplicationEvent struct {
	kind ApplicationEventKind
}

func NewApplicationEvent(kind ApplicationEventKind) *ApplicationEvent {
	return &ApplicationEvent{kind: kind}
}

type Application struct {
	ConfigFilename string
	services       []Service
	servicesCount  int
	events         chan *ApplicationEvent
	done           chan bool
	handlers       map[ApplicationEventKind]func()
}

func NewApplication() *Application {
	if app == nil {
		app = new(Application)
		app.services = []Service{
			NewLogger(),
			NewMailer(),
			NewConsumer(),
		}
		app.servicesCount = len(app.services)
		app.events = make(chan *ApplicationEvent, 4)
		app.done = make(chan bool)
		app.handlers = map[ApplicationEventKind]func(){
			APPLICATION_EVENT_KIND_REGISTER: app.registerServices,
			APPLICATION_EVENT_KIND_INIT    : app.initServices,
			APPLICATION_EVENT_KIND_RUN     : app.runServices,
			APPLICATION_EVENT_KIND_FINISH  : app.finishServices,
		}
	}
	return app
}

func (this *Application) registerServices() {
	for _, service := range this.services {
		service.OnRegister()
	}
	this.events <- NewApplicationEvent(APPLICATION_EVENT_KIND_INIT)
}

func (this *Application) initServices() {
	if len(this.ConfigFilename) > 0 && this.ConfigFilename != EXAMPLE_CONFIG_YAML {
		bytes, err := ioutil.ReadFile(this.ConfigFilename)
		if err == nil {
			event := new(InitEvent)
			event.Data = bytes
			for _, service := range this.services {
				service.OnInit(event)
			}
			this.events <- NewApplicationEvent(APPLICATION_EVENT_KIND_RUN)
		} else {
			FailExitWithErr(err)
		}
	} else {
		FailExit("configuration file not found")
	}
}

func (this *Application) runServices() {
	for _, service := range this.services {
		go service.OnRun()
	}
}

func (this *Application) finishServices() {
	event := new(FinishEvent)
	event.Group = new(sync.WaitGroup)
	event.Group.Add(this.servicesCount)
	for _, service := range this.services {
		go service.OnFinish(event)
	}
	event.Group.Wait()
	time.Sleep(time.Second)
	this.done <- true
}

func (this *Application) Run() {
	runtime.GOMAXPROCS(runtime.NumCPU())
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
	this.events <- NewApplicationEvent(APPLICATION_EVENT_KIND_REGISTER)
	<- this.done
}
