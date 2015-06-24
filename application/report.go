package application

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/consumer"
	"github.com/AdOnWeb/postmanq/analyser"
)

type ReportApplication struct {
	AbstractApplication
}

func NewReport() common.Application {
	return new(ReportApplication)
}

func (a *ReportApplication) Run() {
	a.services = []interface{}{
		consumer.Inst(),
		analyser.Inst(),
	}
	common.Services = []interface{}{
		analyser.Inst(),
	}
	a.run(a, common.NewApplicationEvent(common.InitApplicationEventKind))
}

func (a *ReportApplication) FireRun(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.ReportService)
	go service.OnShowReport()
}
