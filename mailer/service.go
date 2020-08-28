package mailer

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
	"github.com/byorty/dkim"
	yaml "gopkg.in/yaml.v3"
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

	Configs map[string]*Config `yaml:"postmans"`
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
		for name, config := range s.Configs {
			s.init(config, name)
		}
		// указываем заголовки для DKIM
		dkim.StdSignableHeaders = []string{
			"From",
			"To",
			"Subject",
		}
		if s.MailersCount == 0 {
			s.MailersCount = common.DefaultWorkersCount
		}
	} else {
		logger.All().FailExitWithErr(err)
	}
}

func (s *Service) init(conf *Config, hostname string) {
	// закрытый ключ должен быть указан обязательно
	// поэтому даже не проверяем что указано в переменной
	privateKey, err := ioutil.ReadFile(conf.PrivateKeyFilename)
	if err == nil {
		logger.By(hostname).Debug("mailer service private key %s read success", conf.PrivateKeyFilename)
		der, _ := pem.Decode(privateKey)
		conf.privateKey, err = x509.ParsePKCS1PrivateKey(der.Bytes)
		if err != nil {
			logger.By(hostname).Err("mailer service can't decode or parse private key %s", conf.PrivateKeyFilename)
			logger.By(hostname).FailExitWithErr(err)
		}
	} else {
		logger.By(hostname).Err("mailer service can't read private key %s", conf.PrivateKeyFilename)
		logger.By(hostname).FailExitWithErr(err)
	}
	// если не задан селектор, устанавливаем селектор по умолчанию
	if len(conf.DkimSelector) == 0 {
		conf.DkimSelector = "mail"
	}
}

// запускает отправителей и прием сообщений из очереди
func (s *Service) OnRun() {
	logger.All().Debug("run mailers apps...")
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

func (s *Service) getDkimSelector(hostname string) string {
	if conf, ok := s.Configs[hostname]; ok {
		return conf.DkimSelector
	} else {
		logger.By(hostname).Err("mailer service can't find dkim selector by %s", hostname)
		return common.EmptyStr
	}
}

func (s *Service) getPrivateKey(hostname string) *rsa.PrivateKey {
	if conf, ok := s.Configs[hostname]; ok {
		return conf.privateKey
	} else {
		logger.By(hostname).Err("mailer service can't find private key by %s", hostname)
		return nil
	}
}

type Config struct {
	// путь до закрытого ключа
	PrivateKeyFilename string `yaml:"privateKey"`

	// селектор
	DkimSelector string `yaml:"dkimSelector"`

	// содержимое приватного ключа
	privateKey *rsa.PrivateKey
}
