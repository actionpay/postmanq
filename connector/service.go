package connector

import (
	"encoding/pem"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"time"
	"github.com/AdOnWeb/postmanq/log"
	"github.com/AdOnWeb/postmanq/common"
)

var (
	service *Service
	events = make(chan *common.SendEvent)
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
	certBytesLen  int
}

// создает новый сервис соединений
func Inst() *Service {
	if service == nil {
		service = new(Service)
		service.certBytes = []byte{}
	}
	return service
}

// по срабатыванию таймера, просматривает все соединения к почтовым сервисам
// и закрывает те, которые висят дольше 30 секунд
func (c *Service) checkConnections() {
	for now := range c.ticker.C {
		go c.closeConnections(now)
	}
}

func (c *Service) closeConnections(now time.Time) {
	for _, mailServer := range c.mailServers {
		// закрываем соединения к каждого почтового сервиса
		go mailServer.closeConnections(now)
	}
}

func (c *Service) OnInit(event *ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, c)
	if err == nil {
		// если указан путь до сертификата
		if len(c.CertFilename) > 0 {
			// пытаемся прочитать сертификат
			pemBytes, err := ioutil.ReadFile(c.CertFilename)
			if err == nil {
				// получаем сертификат
				pemBlock, _ := pem.Decode(pemBytes)
				c.certBytes = pemBlock.Bytes
				// и считаем его длину, чтобы не делать это при создании каждого сертификата
				c.certBytesLen = len(c.certBytes)
			} else {
				log.FailExit("service can't read certificate, error - %v", err)
			}
		} else {
			log.Debug("certificate is not defined")
		}
		c.addressesLen = len(c.Addresses)
		if c.addressesLen == 0 {
			log.FailExit("ips should be defined")
		}
		if c.ConnectorsCount == 0 {
			c.ConnectorsCount = common.DefaultWorkersCount
		}
	} else {
		log.FailExit("service can't unmarshal config, error - %v", err)
	}
}

func (c *Service) OnRun() {
	// запускаем проверку открытых соединений
//	go service.checkConnections()
	for i := 0; i < c.ConnectorsCount; i++ {
		id := i + 1
		go newPreparer(id).run()
		go newSeeker(id).run()
		go newConnector(id).run()
	}
}

// завершает работу сервиса соединений
func (c *Service) OnFinish() {
	close(c.events)
	// закрываем все соединения
	c.closeConnections(time.Now().Add(time.Minute))
}
