package postmanq

import (
	"sync"
	"runtime"
	"io/ioutil"
	"time"
	"runtime/debug"
	"fmt"
)

const (
	EXAMPLE_CONFIG_YAML = "/path/to/config/file.yaml"
)

var (
	app *Application
)

type RegisterEvent struct {
	LogChan  chan *LogMessage
	MailChan chan *MailMessage
	Group    *sync.WaitGroup
}

type InitEvent struct {
	Data         []byte
	Mailers      []MailerApplication
	MailersCount int
	Group        *sync.WaitGroup
}

type FinishEvent struct {
	Group   *sync.WaitGroup
}

type Service interface {
	OnRegister(*RegisterEvent)
	OnInit(*InitEvent)
	OnRun()
	OnFinish(*FinishEvent)
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
	logChan        chan *LogMessage
	done           chan bool
	handlers       map[ApplicationEventKind]func()
	mailers        []MailerApplication
	mutex          *sync.Mutex
	mailsCount     int64
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
		app.mutex = new(sync.Mutex)
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
			event.Group = new(sync.WaitGroup)
			event.Group.Add(this.servicesCount)
			for _, service := range this.services {
				go service.OnInit(event)
			}
			event.Group.Wait()
			this.mailers = event.Mailers
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
			}
		}
		close(this.events)
	}()
	this.events <- NewApplicationEvent(APPLICATION_EVENT_KIND_REGISTER)
	<- this.done
}

func (this *Application) log(message *LogMessage) {
	defer func(){recover()}()
	app.logChan <- message
}

func Err(message string, args ...interface{}) {
	args = append(args, debug.Stack())
	app.log(NewLogMessage(LOG_LEVEL_ERROR, fmt.Sprint(message, "\n%s"), args...))
}

func FailExit(message string, args ...interface{}) {
	Err(message, args...)
	app.events <- NewApplicationEvent(APPLICATION_EVENT_KIND_FINISH)
}

func FailExitWithErr(err error) {
	FailExit("%v", err)
}

func Warn(message string, args ...interface{}) {
	app.log(NewLogMessage(LOG_LEVEL_WARNING, message, args...))
}

func WarnWithErr(err error) {
	Warn("%v\n%s", err, debug.Stack())
}

func Info(message string, args ...interface{}) {
	app.log(NewLogMessage(LOG_LEVEL_INFO, message, args...))
}

func Debug(message string, args ...interface{}) {
	app.log(NewLogMessage(LOG_LEVEL_DEBUG, message, args...))
}

func SendMail(message *MailMessage) {
	app.mutex.Lock()
	index := -1
	var count int64 = -1
	for i, mailer := range app.mailers {
		Debug("mailer#%d, messages count - %d", i, mailer.MessagesCount())
		if count == -1 || count > mailer.MessagesCount() {
			count = mailer.MessagesCount()
			index = i
		}
	}
	if 0 <= index && index <= len(app.mailers) {
		mailer := app.mailers[index]
		Debug("send message to mailer#%d", index)
		mailer.IncrMessagesCount()
		mailer.Channel() <- message
	}
	app.mutex.Unlock()
}

func GetMailMessageId() int64 {
	app.mutex.Lock()
	app.mailsCount++
	app.mutex.Unlock()
	return app.mailsCount
}

