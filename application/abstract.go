package application

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
	"io/ioutil"
	"time"
)

type AbstractApplication struct {
	configFilename string                        // путь до конфигурационного файла
	services       []interface{}                 // сервисы приложения, отправляющие письма
	events         chan *common.ApplicationEvent // канал событий приложения
	done           chan bool                     // флаг, сигнализирующий окончание работы приложения
}

func (a *AbstractApplication) IsValidConfigFilename(filename string) bool {
	return len(filename) > 0 && filename != common.ExampleConfigYaml
}

func (a *AbstractApplication) run(app Application, event *common.ApplicationEvent) {
	app.SetDone(make(chan bool))
	// создаем каналы для событий
	app.SetEvents(make(chan *common.ApplicationEvent, 3))
	go func() {
		for {
			select {
			case event := <-app.Events():
				if event.kind == common.InitApplicationEventKind {
					// пытаемся прочитать конфигурационный файл
					bytes, err := ioutil.ReadFile(a.configFilename)
					if err == nil {
						event.Data = bytes
					} else {
						logger.FailExit("application can't read configuration file, error -  %v", err)
					}
				}

				for _, service := range app.Services() {
					switch event.kind {
					case common.InitApplicationEventKind:
						app.FireInit(event, service)
					case common.RunApplicationEventKind:
						app.FireRun(event, service)
					case common.FinishApplicationEventKind:
						app.FireFinish(event, service)
					}
				}

				switch event.kind {
				case common.InitApplicationEventKind:
					event.kind = common.RunApplicationEventKind
					app.Events() <- event
				case common.FinishApplicationEventKind:
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

func (a *AbstractApplication) SetEvents(events chan *common.ApplicationEvent) {
	a.events = events
}

func (a *AbstractApplication) Events() chan *common.ApplicationEvent {
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

func (a *AbstractApplication) FireInit(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(Service)
	service.OnInit(event)
}

func (a *AbstractApplication) Run() {}

func (a *AbstractApplication) RunWithArgs(args ...interface{}) {}

func (a *AbstractApplication) FireRun(event *common.ApplicationEvent, abstractService interface{}) {}

func (a *AbstractApplication) FireFinish(event *common.ApplicationEvent, abstractService interface{}) {
}
