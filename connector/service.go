package connector

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/sergw3x/postmanq/common"
	"github.com/sergw3x/postmanq/logger"
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

	cipherSuites = []uint16{
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	}
)

// сервис, управляющий соединениями к почтовым сервисам
// письма могут отсылаться в несколько потоков, почтовый сервис может разрешить несколько подключений с одного IP
// количество подключений может быть не равно количеству отсылающих потоков
// если доверить управление подключениями отправляющим потокам, тогда это затруднит общее управление подключениями
// поэтому создание подключений и предоставление имеющихся подключений отправляющим потокам вынесено в отдельный сервис
type Service struct {
	// количество горутин устанавливающих соединения к почтовым сервисам
	ConnectorsCount int `yaml:"workers"`

	// путь до файла с закрытым ключом
	PrivateKeyFilename string `yaml:"privateKey"`

	// путь до файла с сертификатом
	CertFilename string `yaml:"certificate"`

	// ip с которых будем рассылать письма
	Addresses []string `yaml:"ips"`

	Domain string `yaml:"domain"`

	// количество ip
	addressesLen int

	pool *x509.CertPool

	certs []tls.Certificate

	config *tls.Config
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
		// Load client cert
		cert, err := tls.LoadX509KeyPair(s.CertFilename, s.PrivateKeyFilename)
		if err != nil {
			logger.FailExit("connection service can't load cert from %s and %s, error - %v", s.CertFilename, s.PrivateKeyFilename, err)
		}

		// Load CA cert
		caCert, err := ioutil.ReadFile(s.CertFilename)
		if err != nil {
			logger.FailExit("connection service can't read cert %s, error - %v", s.CertFilename, err)
		}
		s.pool = x509.NewCertPool()
		s.pool.AppendCertsFromPEM(caCert)
		s.certs = []tls.Certificate{cert}

		s.addressesLen = len(s.Addresses)
		if s.addressesLen == 0 {
			logger.FailExit("ips should be defined")
		}
		if s.Domain == common.InvalidInputString {
			logger.FailExit("domain should be defined")
		}
		if s.ConnectorsCount == 0 {
			s.ConnectorsCount = common.DefaultWorkersCount
		}
	} else {
		logger.FailExit("connection service can't unmarshal config, error - %v", err)
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

func (s *Service) getConf(hostname string) *tls.Config {
	conf := &tls.Config{
		ServerName:             hostname,
		ClientAuth:             tls.RequireAnyClientCert,
		CipherSuites:           cipherSuites,
		MinVersion:             tls.VersionTLS12,
		SessionTicketsDisabled: true,
		ClientCAs:              s.pool,
		Certificates:           s.certs,
	}
	return conf
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
