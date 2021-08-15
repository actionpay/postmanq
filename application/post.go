package application

import (
	"runtime"

	"gopkg.in/yaml.v3"

	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/connector"
	"github.com/Halfi/postmanq/consumer"
	"github.com/Halfi/postmanq/guardian"
	"github.com/Halfi/postmanq/limiter"
	"github.com/Halfi/postmanq/logger"
	"github.com/Halfi/postmanq/mailer"
)

// Post приложение, рассылающее письма
type Post struct {
	Abstract

	// Workers количество отправителей
	Workers int `yaml:"workers"`
}

// NewPost создает новое приложение
func NewPost() common.Application {
	return new(Post)
}

// Run запускает приложение
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

// Init инициализирует приложение
func (p *Post) Init(event *common.ApplicationEvent) {
	// получаем настройки
	err := yaml.Unmarshal(event.Data, p)
	if err == nil {
		p.CommonTimeout.Init()
		common.DefaultWorkersCount = p.Workers
		runtime.GOMAXPROCS(common.DefaultWorkersCount * 2)
		logger.All().Debug("app workers count %d", p.Workers)
	} else {
		logger.All().FailExitErr(err)
	}
}

// FireRun запускает сервисы приложения
func (p *Post) FireRun(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.SendingService)
	go service.OnRun()
}

// FireFinish останавливает сервисы приложения
func (p *Post) FireFinish(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.SendingService)
	go service.OnFinish()
}
