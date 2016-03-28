package mailer

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/actionpay/postmanq/common"
	"github.com/actionpay/postmanq/logger"
	"github.com/byorty/dkim"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
)

var (
	// сервис отправки писем
	service *Service
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
	privateKey *rsa.PrivateKey
}

// создает новый сервис отправки писем
func Inst() common.SendingService {
	if service == nil {
		service = new(Service)
	}
	return service
}

// инициализирует сервис отправки писем
func (s *Service) OnInit(event *common.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		logger.Debug("read private key file %s", s.PrivateKeyFilename)
		// закрытый ключ должен быть указан обязательно
		// поэтому даже не проверяем что указано в переменной
		privateKey, err := ioutil.ReadFile(s.PrivateKeyFilename)
		if err == nil {
			logger.Debug("private key read success")
			der, _ := pem.Decode(privateKey)
			s.privateKey, err = x509.ParsePKCS1PrivateKey(der.Bytes)
			if err != nil {
				logger.Debug("can't decode or parse private key")
				logger.FailExitWithErr(err)
			}
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
		if len(s.DkimSelector) == 0 {
			s.DkimSelector = "mail"
		}
		if s.MailersCount == 0 {
			s.MailersCount = common.DefaultWorkersCount
		}
	} else {
		logger.FailExitWithErr(err)
	}
}

// запускает отправителей и прием сообщений из очереди
func (s *Service) OnRun() {
	logger.Debug("run mailers apps...")
	for i := 0; i < s.MailersCount; i++ {
		go newMailer(i + 1)
	}
}

// канал для приема событий отправки писем
func (s *Service) Events() chan *common.SendEvent {
	return events
}

// завершает работу сервиса отправки писем
func (s *Service) OnFinish() {
	close(events)
}
