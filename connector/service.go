package connector

import (
	"crypto/x509"
	"encoding/pem"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
)

var (
	// сервис создания соединения
	service *Service

	// канал для приема событий отправки писем
	events = make(chan *common.SendEvent)

	// почтовые сервисы будут хранится в карте по домену
	mailServers = make(map[string]*MailServer)
)

// сервис, управляющий соединениями к почтовым сервисам
// письма могут отсылаться в несколько потоков, почтовый сервис может разрешить несколько подключений с одного IP
// количество подключений может быть не равно количеству отсылающих потоков
// если доверить управление подключениями отправляющим потокам, тогда это затруднит общее управление подключениями
// поэтому создание подключений и предоставление имеющихся подключений отправляющим потокам вынесено в отдельный сервис
type Service struct {
	Config

	Configs map[string]Config `yaml:"domains"`
}

// создает новый сервис соединений
func Inst() *Service {
	if service == nil {
		service = new(Service)
	}
	return service
}

// инициализирует сервис соединений
func (s *Service) OnInit(event *common.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		s.init(&s.Config, common.AllDomains)
		for name, config := range s.Configs {
			s.init(&config, name)
		}
		if s.ConnectorsCount == 0 {
			s.ConnectorsCount = common.DefaultWorkersCount
		}
	} else {
		logger.All().FailExit("connection service can't unmarshal config, error - %v", err)
	}
}

func (s *Service) init(conf *Config, hostname string) {
	// если указан путь до сертификата
	if len(conf.CertFilename) > 0 {
		// пытаемся прочитать сертификат
		pemBytes, err := ioutil.ReadFile(conf.CertFilename)
		if err == nil {
			// получаем сертификат
			pemBlock, _ := pem.Decode(pemBytes)
			cert, _ := x509.ParseCertificate(pemBlock.Bytes)
			conf.pool = x509.NewCertPool()
			conf.pool.AddCert(cert)
		} else {
			logger.By(hostname).FailExit("connection service can't read certificate %s, error - %v", conf.CertFilename, err)
		}
	} else {
		logger.By(hostname).Debug("certificate is not defined")
	}
	conf.addressesLen = len(conf.Addresses)
	if conf.addressesLen == 0 {
		logger.By(hostname).FailExit("ips should be defined")
	}
}

// запускает горутины
func (s *Service) OnRun() {
	for i := 0; i < s.ConnectorsCount; i++ {
		id := i + 1
		go newPreparer(id)
		go newSeeker(id)
		go newConnector(id)
	}
}

// канал для приема событий отправки писем
func (s *Service) Events() chan *common.SendEvent {
	return events
}

// завершает работу сервиса соединений
func (s *Service) OnFinish() {
	close(events)
}

func (s Service) getPool(hostname string) *x509.CertPool {
	if conf, ok := s.Configs[hostname]; ok {
		return conf.pool
	} else {
		return s.pool
	}
}

func (s Service) getAddresses(hostname string) []string {
	if conf, ok := s.Configs[hostname]; ok {
		return conf.Addresses
	} else {
		return s.Addresses
	}
}

func (s Service) getAddress(hostname string, id int) []string {
	if conf, ok := s.Configs[hostname]; ok {
		return conf.Addresses[id%conf.addressesLen]
	} else {
		return s.Addresses[id%s.addressesLen]
	}
}

// событие создания соединения
type ConnectionEvent struct {
	*common.SendEvent

	// канал для получения почтового сервиса после поиска информации о его серверах
	servers chan *MailServer

	// почтовый сервис, которому будет отправлено письмо
	server *MailServer

	// идентификатор заготовщика запросившего поиск информации о почтовом сервисе
	connectorId int

	// адрес, с которого будет отправлено письмо
	address string
}

type Config struct {
	// количество горутин устанавливающих соединения к почтовым сервисам
	ConnectorsCount int `yaml:"workers"`

	// путь до файла с закрытым ключом
	PrivateKeyFilename string `yaml:"privateKey"`

	// путь до файла с сертификатом
	CertFilename string `yaml:"certificate"`

	// ip с которых будем рассылать письма
	Addresses []string `yaml:"ips"`

	// количество ip
	addressesLen int

	// пул сертификатов
	pool *x509.CertPool
}
