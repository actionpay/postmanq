package connector

import (
	"encoding/pem"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
)

var (
	service *Service
	events  = make(chan *common.SendEvent)
	// почтовые сервисы будут хранится в карте по домену
	mailServers = make(map[string]*MailServer)
)

// Сервис, управляющий соединениями к почтовым сервисам.
// Письма могут отсылаться в несколько потоков, почтовый сервис может разрешить несколько подключений с одного IP.
// Количество подключений может быть не равно количеству отсылающих потоков.
// Если доверить управление подключениями отправляющим потокам, тогда это затруднит общее управление подключениями.
// Поэтому создание подключений и предоставление имеющихся подключений отправляющим потокам вынесено в отдельный сервис.
type Service struct {
	ConnectorsCount int `yaml:"workers"`
	// путь до файла с закрытым ключом
	PrivateKeyFilename string `yaml:"privateKey"`
	// путь до файла с сертификатом
	CertFilename string `yaml:"certificate"`
	// ip с которых будем рассылать письма
	Addresses    []string `yaml:"ips"`
	addressesLen int
	// сертификат в байтах
	certBytes []byte
	// длина сертификата
	certBytesLen int
}

// создает новый сервис соединений
func Inst() *Service {
	if service == nil {
		service = new(Service)
		service.certBytes = []byte{}
	}
	return service
}

func (s *Service) OnInit(event *ApplicationEvent) {
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

func (s *Service) OnRun() {
	for i := 0; i < s.ConnectorsCount; i++ {
		id := i + 1
		go newPreparer(id)
		go newSeeker(id)
		go newConnector(id)
	}
}

func (s *Service) Events() chan *common.SendEvent {
	return events
}

// завершает работу сервиса соединений
func (s *Service) OnFinish() {
	close(events)
}

type ConnectionEvent struct {
	*common.SendEvent
	servers     chan *MailServer
	server      *MailServer
	connectorId int
	address     string
}
