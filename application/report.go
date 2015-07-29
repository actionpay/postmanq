package application

import (
	"github.com/AdOnWeb/postmanq/analyser"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/consumer"
)

// приложение, анализирующее неотправленные сообщения
type Report struct {
	Abstract
}

// создает новое приложение
func NewReport() common.Application {
	return new(Report)
}

// запускает приложение
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

// запускает сервисы приложения
func (r *Report) FireRun(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.ReportService)
	go service.OnShowReport()
}
