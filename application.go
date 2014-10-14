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

type RegisterEvent struct {
	LogChan chan *LogMessage
	Group   *sync.WaitGroup
}

type InitEvent struct {
	Data []byte
}

type FinishEvent struct {
	Group   *sync.WaitGroup
}

type Service interface {
	OnRegister(*RegisterEvent)
	OnInit(*InitEvent)
	OnFinish(*FinishEvent)
}

type ApplicationEventKind int

const (
	APPLICATION_EVENT_KIND_REGISTER ApplicationEventKind = iota
	APPLICATION_EVENT_KIND_INIT
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
	logChan        chan *LogMessage
	done           chan bool
	handlers       map[ApplicationEventKind]func()
}

func NewApplication() *Application {
	if app == nil {
		app = new(Application)
		app.services = []Service{
			NewLogger(),
		}
		app.servicesCount = len(app.services)
		app.events = make(chan *ApplicationEvent, 3)
		app.done = make(chan bool)
		app.handlers = map[ApplicationEventKind]func(){
			APPLICATION_EVENT_KIND_REGISTER: app.registerServices,
			APPLICATION_EVENT_KIND_INIT    : app.initServices,
			APPLICATION_EVENT_KIND_FINISH  : app.finishServices,
		}
	}
	return app
}

func (this *Application) registerServices() {
	event := new(RegisterEvent)
	event.Group = new(sync.WaitGroup)
	event.Group.Add(this.servicesCount)
	for _, service := range this.services {
		go service.OnRegister(event)
	}
	event.Group.Wait()
	this.logChan = event.LogChan
	this.events <- NewApplicationEvent(APPLICATION_EVENT_KIND_INIT)
}

func (this *Application) initServices() {
	if len(this.ConfigFilename) > 0 && this.ConfigFilename != EXAMPLE_CONFIG_YAML {
		bytes, err := ioutil.ReadFile(this.ConfigFilename)
		if err == nil {
			event := new(InitEvent)
			event.Data = bytes
			for _, service := range this.services {
				go service.OnInit(event)
			}
		} else {
			FailExit("%v", err)
		}
	} else {
		FailExit("configuration file not found")
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
	time.Sleep(1 * time.Second)
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
			default:
			}
		}
		close(this.events)
	}()
	this.events <- NewApplicationEvent(APPLICATION_EVENT_KIND_REGISTER)
	<- this.done
}

func FailExit(message string, args ...interface{}) {
	app.logChan <- NewLogMessage(LOG_LEVEL_CRITICAL, message, args...)
	app.events <- NewApplicationEvent(APPLICATION_EVENT_KIND_FINISH)
}

func Err(message string, args ...interface{}) {
	app.logChan <- NewLogMessage(LOG_LEVEL_ERROR, message, args...)
}

func Warn(message string, args ...interface{}) {
	app.logChan <- NewLogMessage(LOG_LEVEL_WARNING, message, args...)
}

func Info(message string, args ...interface{}) {
	app.logChan <- NewLogMessage(LOG_LEVEL_INFO, message, args...)
}


