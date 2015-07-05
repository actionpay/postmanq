package connector

import (
	"encoding/pem"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"crypto/x509"
)

var (
	// Сервис создания соединения
	service *Service

	// Канал для приема событий отправки писем
	events = make(chan *common.SendEvent)

	// Почтовые сервисы будут хранится в карте по домену
	mailServers = make(map[string]*MailServer)
)

// Сервис, управляющий соединениями к почтовым сервисам
// Письма могут отсылаться в несколько потоков, почтовый сервис может разрешить несколько подключений с одного IP
// Количество подключений может быть не равно количеству отсылающих потоков
// Если доверить управление подключениями отправляющим потокам, тогда это затруднит общее управление подключениями
// Поэтому создание подключений и предоставление имеющихся подключений отправляющим потокам вынесено в отдельный сервис
type Service struct {
	// Количество горутин устанавливающих соединения к почтовым сервисам
	ConnectorsCount int `yaml:"workers"`

	// Путь до файла с закрытым ключом
	PrivateKeyFilename string `yaml:"privateKey"`

	// Путь до файла с сертификатом
	CertFilename string `yaml:"certificate"`

	// IP с которых будем рассылать письма
	Addresses []string `yaml:"ips"`

	// Количество IP
	addressesLen int

	// Сертификат в байтах
	certBytes []byte

	// Длина сертификата
	certBytesLen int

	cert *x509.Certificate

	pool *x509.CertPool
}

// Создает новый сервис соединений
func Inst() *Service {
	if service == nil {
		service = new(Service)
		service.certBytes = []byte{}
	}
	return service
}

// Инициализирует сервис соединений
func (s *Service) OnInit(event *common.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		// если указан путь до сертификата
		if len(s.CertFilename) > 0 {
			// пытаемся прочитать сертификат
			pemBytes, err := ioutil.ReadFile(s.CertFilename)
			if err == nil {
				// получаем сертификат
				pemBlock, _ := pem.Decode(pemBytes)
				s.certBytes = pemBlock.Bytes
				// и считаем его длину, чтобы не делать это при создании каждого сертификата
				s.certBytesLen = len(s.certBytes)
				s.cert, _ = x509.ParseCertificate(pemBlock.Bytes)
				s.pool = x509.NewCertPool()
				s.pool.AddCert(s.cert)
			} else {
				logger.FailExit("service can't read certificate, error - %v", err)
			}
		} else {
			logger.Debug("certificate is not defined")
		}
		s.addressesLen = len(s.Addresses)
		if s.addressesLen == 0 {
			logger.FailExit("ips should be defined")
		}
		if s.ConnectorsCount == 0 {
			s.ConnectorsCount = common.DefaultWorkersCount
		}
	} else {
		logger.FailExit("service can't unmarshal config, error - %v", err)
	}
}

// Запускает горутины
func (s *Service) OnRun() {
	for i := 0; i < s.ConnectorsCount; i++ {
		id := i + 1
		go newPreparer(id)
		go newSeeker(id)
		go newConnector(id)
	}
}

// Канал для приема событий отправки писем
func (s *Service) Events() chan *common.SendEvent {
	return events
}

// Завершает работу сервиса соединений
func (s *Service) OnFinish() {
	close(events)
}

// Событие создания соединения
type ConnectionEvent struct {
	*common.SendEvent

	// Канал для получения почтового сервиса после поиска информации о его серверах
	servers chan *MailServer

	// Почтовый сервис, которому будет отправлено письмо
	server *MailServer

	// Идентификатор заготовщика запросившего поиск информации о почтовом сервисе
	connectorId int

	// Адрес, с которого будет отправлено письмо
	address string
}
