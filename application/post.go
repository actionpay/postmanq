package application

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/connector"
	"github.com/AdOnWeb/postmanq/consumer"
	"github.com/AdOnWeb/postmanq/limiter"
	"github.com/AdOnWeb/postmanq/logger"
	"github.com/AdOnWeb/postmanq/mailer"
)

type Post struct {
	Abstract
}

func NewPost() common.Application {
	return new(Post)
}

func (p *Post) Run() {
	common.App = p
	common.Services = []interface{}{
		limiter.Inst(),
		connector.Inst(),
		mailer.Inst(),
	}
	p.services = []interface{}{
		logger.Inst(),
		consumer.Inst(),
		limiter.Inst(),
		connector.Inst(),
		mailer.Inst(),
	}
	p.run(p, common.NewApplicationEvent(common.InitApplicationEventKind))
}

func (p *Post) FireRun(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.SendingService)
	go service.OnRun()
}

func (p *Post) FireFinish(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.SendingService)
	go service.OnFinish()
}
