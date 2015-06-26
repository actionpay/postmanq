package application

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/grep"
)

type Grep struct {
	Abstract
}

func NewGrep() common.Application {
	return new(Grep)
}

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

func (g *Grep) FireRun(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.GrepService)
	go service.OnGrep(event)
}
