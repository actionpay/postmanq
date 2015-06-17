package application

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/consumer"
	"github.com/AdOnWeb/postmanq/limiter"
	"github.com/AdOnWeb/postmanq/logger"
	"github.com/AdOnWeb/postmanq/connector"
	"github.com/AdOnWeb/postmanq/mailer"
)

type PostApplication struct {
	AbstractApplication
}

func NewPost() common.Application {
	return new(PostApplication)
}

func (a *PostApplication) Run() {
	a.services = []interface{}{
		logger.Inst(),
		consumer.Inst(),
		limiter.Inst(),
		connector.Inst(),
		mailer.Inst(),
	}
	common.SendindServices = []interface {}{
		limiter.Inst(),
		connector.Inst(),
		mailer.Inst(),
	}
	a.run(a, NewApplicationEvent(InitApplicationEventKind))
}
