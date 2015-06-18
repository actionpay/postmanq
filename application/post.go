package application

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/connector"
	"github.com/AdOnWeb/postmanq/consumer"
	"github.com/AdOnWeb/postmanq/limiter"
	"github.com/AdOnWeb/postmanq/logger"
	"github.com/AdOnWeb/postmanq/mailer"
)

type PostApplication struct {
	AbstractApplication
}

func NewPost() common.Application {
	return new(PostApplication)
}

func (p *PostApplication) Run() {
	p.services = []interface{}{
		logger.Inst(),
		consumer.Inst(),
		limiter.Inst(),
		connector.Inst(),
		mailer.Inst(),
	}
	common.SendindServices = []interface{}{
		limiter.Inst(),
		connector.Inst(),
		mailer.Inst(),
	}
	p.run(p, common.NewApplicationEvent(common.InitApplicationEventKind))
}

func (p *PostApplication) FireRun(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.SendingService)
	go service.OnRun()
}

func (p *PostApplication) FireFinish(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.SendingService)
	go service.OnFinish()
}
