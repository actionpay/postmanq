package application

import (
	"io/ioutil"
	"time"

	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
)

type FireAction interface {
	Fire(common.Application, *common.ApplicationEvent, interface{})
}

type PreFireAction interface {
	FireAction
	PreFire(common.Application, *common.ApplicationEvent)
}

type PostFireAction interface {
	FireAction
	PostFire(common.Application, *common.ApplicationEvent)
}

var (
	actions = map[common.ApplicationEventKind]FireAction{
		common.InitApplicationEventKind:   InitFireAction((*Abstract).FireInit),
		common.RunApplicationEventKind:    RunFireAction((*Abstract).FireRun),
		common.FinishApplicationEventKind: FinishFireAction((*Abstract).FireFinish),
	}
)

// Abstract базовое приложение
type Abstract struct {
	// путь до конфигурационного файла
	configFilename string

	// сервисы приложения, отправляющие письма
	services []interface{}

	// канал событий приложения
	events       chan *common.ApplicationEvent
	eventsClosed bool

	// флаг, сигнализирующий окончание работы приложения
	done       chan bool
	doneClosed bool

	CommonTimeout common.Timeout `yaml:"timeouts"`
}

// IsValidConfigFilename проверяет валидность пути к файлу с настройками
func (a *Abstract) IsValidConfigFilename(filename string) bool {
	return len(filename) > 0 && filename != common.ExampleConfigYaml
}

// запускает основной цикл приложения
func (a *Abstract) run(app common.Application, event *common.ApplicationEvent) {
	// создаем каналы для событий
	app.InitChannels(3)
	defer app.CloseEvents()

	app.OnEvent(func(ev *common.ApplicationEvent) {
		action := actions[event.Kind]

		if preAction, ok := action.(PreFireAction); ok {
			preAction.PreFire(app, event)
		}

		for _, service := range app.Services() {
			action.Fire(app, event, service)
		}

		if postAction, ok := action.(PostFireAction); ok {
			postAction.PostFire(app, event)
		}
	})

	app.SendEvents(event)
	<-app.Done()
}

func (a Abstract) GetConfigFilename() string {
	return a.configFilename
}

// SetConfigFilename устанавливает путь к файлу с настройками
func (a *Abstract) SetConfigFilename(configFilename string) {
	a.configFilename = configFilename
}

// SetEvents устанавливает канал событий приложения
func (a *Abstract) SetEvents(events chan *common.ApplicationEvent) {
	a.events = events
}

// InitChannels init channels
func (a *Abstract) InitChannels(cBufSize int) {
	a.events = make(chan *common.ApplicationEvent, cBufSize)
	a.done = make(chan bool)
}

func (a *Abstract) OnEvent(f func(ev *common.ApplicationEvent)) {
	go func() {
		for ev := range a.events {
			go f(ev)
		}
	}()
}

// CloseEvents close events channel
func (a *Abstract) CloseEvents() {
	if !a.eventsClosed {
		a.eventsClosed = true
		close(a.events)
	}
}

func (a *Abstract) SendEvents(ev *common.ApplicationEvent) bool {
	if a.eventsClosed {
		return false
	}

	a.events <- ev
	return true
}

// Done возвращает канал завершения приложения
func (a *Abstract) Done() <-chan bool {
	return a.done
}

func (a *Abstract) Close() {
	if !a.doneClosed {
		a.doneClosed = true
		a.done <- true
	}
}

// Services возвращает сервисы, используемые приложением
func (a *Abstract) Services() []interface{} {
	return a.services
}

// FireInit инициализирует сервисы
func (a *Abstract) FireInit(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.Service)
	service.OnInit(event)
}

// Init инициализирует приложение
func (a *Abstract) Init(event *common.ApplicationEvent) {}

// Run запускает приложение
func (a *Abstract) Run() {}

// RunWithArgs запускает приложение с аргументами
func (a *Abstract) RunWithArgs(args ...interface{}) {}

// FireRun запускает сервисы приложения
func (a *Abstract) FireRun(event *common.ApplicationEvent, abstractService interface{}) {}

// FireFinish останавливает сервисы приложения
func (a *Abstract) FireFinish(event *common.ApplicationEvent, abstractService interface{}) {}

// Timeout возвращает таймауты приложения
func (a *Abstract) Timeout() common.Timeout {
	return a.CommonTimeout
}

type InitFireAction func(*Abstract, *common.ApplicationEvent, interface{})

func (i InitFireAction) Fire(app common.Application, event *common.ApplicationEvent, abstractService interface{}) {
	app.FireInit(event, abstractService)
}

func (i InitFireAction) PreFire(app common.Application, event *common.ApplicationEvent) {
	// пытаемся прочитать конфигурационный файл
	bytes, err := ioutil.ReadFile(app.GetConfigFilename())
	if err == nil {
		event.Data = bytes
		app.Init(event)
	} else {
		logger.All().FailExitWithErr(err, "application can't read configuration file")
	}
}

func (i InitFireAction) PostFire(app common.Application, event *common.ApplicationEvent) {
	event.Kind = common.RunApplicationEventKind
	app.SendEvents(event)
}

type RunFireAction func(*Abstract, *common.ApplicationEvent, interface{})

func (r RunFireAction) Fire(app common.Application, event *common.ApplicationEvent, abstractService interface{}) {
	app.FireRun(event, abstractService)
}

type FinishFireAction func(*Abstract, *common.ApplicationEvent, interface{})

func (f FinishFireAction) Fire(app common.Application, event *common.ApplicationEvent, abstractService interface{}) {
	app.FireFinish(event, abstractService)
}

func (f FinishFireAction) PostFire(app common.Application, event *common.ApplicationEvent) {
	time.Sleep(2 * time.Second)
	app.Close()
}
