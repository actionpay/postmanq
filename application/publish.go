package application

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/consumer"
)

type Publish struct {
	Abstract
}

func NewPublish() common.Application {
	return new(Publish)
}

func (a *Publish) RunWithArgs(args ...interface{}) {
	a.services = []interface{}{
		consumer.Inst(),
	}

	event := common.NewApplicationEvent(common.InitApplicationEventKind)
	event.Args = make(map[string]interface{})
	event.Args["srcQueue"] = args[0]
	event.Args["destQueue"] = args[1]
	event.Args["host"] = args[2]
	event.Args["code"] = args[3]
	event.Args["envelope"] = args[4]
	event.Args["recipient"] = args[5]

	a.run(a, event)
}

func (a *Publish) FireRun(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.PublishService)
	go service.OnPublish(event)
}
