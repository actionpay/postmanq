package application

import (
	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
	"io/ioutil"
	"time"
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

// базовое приложение
type Abstract struct {
	// путь до конфигурационного файла
	configFilename string

	// сервисы приложения, отправляющие письма
	services []interface{}

	// канал событий приложения
	events chan *common.ApplicationEvent

	// флаг, сигнализирующий окончание работы приложения
	done chan bool

	CommonTimeout common.Timeout `yaml:"timeouts"`
}

// проверяет валидность пути к файлу с настройками
func (a *Abstract) IsValidConfigFilename(filename string) bool {
	return len(filename) > 0 && filename != common.ExampleConfigYaml
}

// запускает основной цикл приложения
func (a *Abstract) run(app common.Application, event *common.ApplicationEvent) {
	app.SetDone(make(chan bool))
	// создаем каналы для событий
	app.SetEvents(make(chan *common.ApplicationEvent, 3))
	go func() {
		for event := range app.Events() {
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
		}
		close(app.Events())
	}()
	app.Events() <- event
	<-app.Done()
}

func (a Abstract) GetConfigFilename() string {
	return a.configFilename
}

// устанавливает путь к файлу с настройками
func (a *Abstract) SetConfigFilename(configFilename string) {
	a.configFilename = configFilename
}

// устанавливает канал событий приложения
func (a *Abstract) SetEvents(events chan *common.ApplicationEvent) {
	a.events = events
}

// возвращает канал событий приложения
func (a *Abstract) Events() chan *common.ApplicationEvent {
	return a.events
}

// устанавливает канал завершения приложения
func (a *Abstract) SetDone(done chan bool) {
	a.done = done
}

// возвращает канал завершения приложения
func (a *Abstract) Done() chan bool {
	return a.done
}

// возвращает сервисы, используемые приложением
func (a *Abstract) Services() []interface{} {
	return a.services
}

// инициализирует сервисы
func (a *Abstract) FireInit(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.Service)
	service.OnInit(event)
}

// инициализирует приложение
func (a *Abstract) Init(event *common.ApplicationEvent) {}

// запускает приложение
func (a *Abstract) Run() {}

// запускает приложение с аргументами
func (a *Abstract) RunWithArgs(args ...interface{}) {}

// запускает сервисы приложения
func (a *Abstract) FireRun(event *common.ApplicationEvent, abstractService interface{}) {}

// останавливает сервисы приложения
func (a *Abstract) FireFinish(event *common.ApplicationEvent, abstractService interface{}) {}

// возвращает таймауты приложения
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
		logger.All().FailExit("application can't read configuration file, error -  %v", err)
	}
}

func (i InitFireAction) PostFire(app common.Application, event *common.ApplicationEvent) {
	event.Kind = common.RunApplicationEventKind
	app.Events() <- event
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
	app.Done() <- true
}
