package application

import (
	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/grep"
)

// приложение, ищущее логи по адресату или получателю
type Grep struct {
	Abstract
}

// создает новое приложение
func NewGrep() common.Application {
	return new(Grep)
}

// запускает приложение с аргументами
func (g *Grep) RunWithArgs(args ...interface{}) {
	common.App = g
	g.services = []interface{}{
		grep.Inst(),
	}

	event := common.NewApplicationEvent(common.InitApplicationEventKind)
	event.Args = make(map[string]interface{})
	event.Args["envelope"] = args[0]
	event.Args["recipient"] = args[1]
	event.Args["numberLines"] = args[2]

	g.run(g, event)
}

// запускает сервисы приложения
func (g *Grep) FireRun(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.GrepService)
	go service.OnGrep(event)
}
