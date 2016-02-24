package application

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/connector"
	"github.com/AdOnWeb/postmanq/consumer"
	"github.com/AdOnWeb/postmanq/guardian"
	"github.com/AdOnWeb/postmanq/limiter"
	"github.com/AdOnWeb/postmanq/logger"
	"github.com/AdOnWeb/postmanq/mailer"
	yaml "gopkg.in/yaml.v2"
)

// приложение, рассылающее письма
type Post struct {
	Abstract

	// количество отправителей
	Workers int `yaml:"workers"`
}

// создает новое приложение
func NewPost() common.Application {
	return new(Post)
}

// запускает приложение
func (p *Post) Run() {
	common.App = p
	common.Services = []interface{}{
		guardian.Inst(),
		limiter.Inst(),
		connector.Inst(),
		mailer.Inst(),
	}
	p.services = []interface{}{
		logger.Inst(),
		consumer.Inst(),
		guardian.Inst(),
		limiter.Inst(),
		connector.Inst(),
		mailer.Inst(),
	}
	p.run(p, common.NewApplicationEvent(common.InitApplicationEventKind))
}

// инициализирует приложение
func (p *Post) Init(event *common.ApplicationEvent) {
	// получаем настройки
	err := yaml.Unmarshal(event.Data, p)
	if err == nil {
		p.CommonTimeout.Init()
		common.DefaultWorkersCount = p.Workers
		logger.All().Debug("app workers count %d", p.Workers)
	} else {
		logger.All().FailExitWithErr(err)
	}
}

// запускает сервисы приложения
func (p *Post) FireRun(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.SendingService)
	go service.OnRun()
}

// останавливает сервисы приложения
func (p *Post) FireFinish(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.SendingService)
	go service.OnFinish()
}
