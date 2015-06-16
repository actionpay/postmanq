package mailer

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
	"github.com/byorty/dkim"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"sync/atomic"
	"time"
)

var (
	// хранит количество отправленных писем за минуту, используется для отладки
	mailsPerMinute int64
	service        *Service
	// канал для писем
	events = make(chan *common.SendEvent)
)

// сервис отправки писем
type Service struct {
	// количество отправителей
	MailersCount int `yaml:"workers"`
	// путь до закрытого ключа
	PrivateKeyFilename string `yaml:"privateKey"`
	// селектор
	DkimSelector string `yaml:"dkimSelector"`
	// содержимое приватного ключа
	privateKey []byte
}

// создает новый сервис отправки писем
func Inst() common.SendingService {
	if service == nil {
		service = new(Service)
	}
	return service
}

// инициализирует сервис отправки писем
func (this *Service) OnInit(event *common.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		logger.Debug("read private key file %s", this.PrivateKeyFilename)
		// закрытый ключ должен быть указан обязательно
		// поэтому даже не проверяем что указано в переменной
		this.privateKey, err = ioutil.ReadFile(this.PrivateKeyFilename)
		if err == nil {
			logger.Debug("private key read success")
		} else {
			logger.Debug("can't read private key")
			logger.FailExitWithErr(err)
		}
		// указываем заголовки для DKIM
		dkim.StdSignableHeaders = []string{
			"From",
			"To",
			"Subject",
		}
		// если не задан селектор, устанавливаем селектор по умолчанию
		if len(this.DkimSelector) == 0 {
			this.DkimSelector = "mail"
		}
		if this.MailersCount == 0 {
			this.MailersCount = common.DefaultWorkersCount
		}
	} else {
		logger.FailExitWithErr(err)
	}
}

// запускает отправителей и прием сообщений из очереди
func (this *Service) OnRun() {
	logger.Debug("run mailers apps...")
	// выводим количество отправленных писем в минуту
	//	go this.showMailsPerMinute()

	for i := 0; i < this.MailersCount; i++ {
		go newMailer(i + 1)
	}
}

// выводит количество отправленных писем раз в минуту
func (this *Service) showMailsPerMinute() {
	tick := time.Tick(time.Minute)
	for {
		select {
		case <-tick:
			logger.Debug("mailers send %d mails per minute", atomic.LoadInt64(&mailsPerMinute))
			atomic.StoreInt64(&mailsPerMinute, 0)
			break
		}
	}
}

func (this *Service) OnFinish() {
	close(events)
}
