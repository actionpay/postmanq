package application

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/consumer"
	"github.com/AdOnWeb/postmanq/analyser"
)

type Report struct {
	Abstract
}

func NewReport() common.Application {
	return new(Report)
}

func (r *Report) Run() {
	common.App = r
	common.Services = []interface{}{
		analyser.Inst(),
	}
	r.services = []interface{}{
		consumer.Inst(),
		analyser.Inst(),
	}
	r.run(r, common.NewApplicationEvent(common.InitApplicationEventKind))
}

func (r *Report) FireRun(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.ReportService)
	go service.OnShowReport()
}
