package connector

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"math/rand"
	"net"
	"net/smtp"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"github.com/AdOnWeb/postmanq/log"
	"github.com/AdOnWeb/postmanq/common"
)

var (
	service *Service
	events = make(chan *common.SendEvent)
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
	// почтовые сервисы
	mailServers map[string]*MailServer
	// семафор, необходим для создания и поиска соединений
	mutex *sync.Mutex
	// таймер, необходим для проверки открытых соединений
	ticker *time.Ticker
	// сертификат в байтах
	certBytes []byte
	// длина сертификата
	certBytesLen  int
	events        chan *SendEvent
	lookupEvents  chan *SendEvent
	connectEvents chan *SendEvent
}

// создает новый сервис соединений
func Inst() *Service {
	if service == nil {
		service = new(Service)
		// почтовые сервисы будут хранится в карте по домену
		service.mailServers = make(map[string]*MailServer)
		service.mutex = new(sync.Mutex)
		// создаем таймер
		service.ticker = time.NewTicker(6 * time.Second)
		service.certBytes = []byte{}

//		service.lookupEvents = make(chan *SendEvent)
//		service.connectEvents = make(chan *SendEvent)
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
	go service.checkConnections()
	for i := 0; i < c.ConnectorsCount; i++ {
		id := i + 1
		go c.receiveConnections(id)
		go c.lookupServers(id)
		go c.createConnections(id)
	}
}

func (c *Service) createConnections(id int) {
	for event := range c.events {
		c.doConnection(id, event)
	}
}

func (c *Service) doConnection(id int, event *SendEvent) {
	log.Info("service#%d try create connection for mail#%d", id, event.Message.Id)
	// передаем событию сертификат и его длину
	event.CertBytes = c.certBytes
	event.CertBytesLen = c.certBytesLen
	goto connectToMailServer

connectToMailServer:
	c.lookupEvents <- event
	mailServer := <-event.MailServers
	switch mailServer.status {
	case LookupMailServerStatus:
		goto waitLookup
		return
	case SuccessMailServerStatus:
		event.MailServer = mailServer
		c.connectEvents <- event
		return
	case ErrorMailServerStatus:
		ReturnMail(
			event,
			errors.New(fmt.Sprintf("511 service#%d can't lookup %s", id, event.Message.HostnameTo)),
		)
		return
	}

waitLookup:
	log.Debug("service#%d wait ending look up mail server %s...", id, event.Message.HostnameTo)
	time.Sleep(SleepTimeout)
	goto connectToMailServer
}

func (c *Service) lookupServers(id int) {
	for event := range c.lookupEvents {
		c.lookupServer(id, event)
	}
}

func (c *Service) lookupServer(id int, event *common.SendEvent) {
	c.mutex.Lock()
	if _, ok := c.mailServers[event.Message.HostnameTo]; !ok {
		log.Debug("service#%d create mail server for %s", id, event.Message.HostnameTo)
		c.mailServers[event.Message.HostnameTo] = &MailServer{
			status:      LookupMailServerStatus,
			connectorId: id,
		}
	}
	c.mutex.Unlock()
	mailServer := c.mailServers[event.Message.HostnameTo]
	if id == mailServer.connectorId && mailServer.status == LookupMailServerStatus {
		log.Debug("service#%d look up mx domains for %s...", id, event.Message.HostnameTo)
		mailServer := c.mailServers[event.Message.HostnameTo]
		// ищем почтовые сервера для домена
		mxs, err := net.LookupMX(event.Message.HostnameTo)
		if err == nil {
			mailServer.mxServers = make([]*MxServer, len(mxs))
			for i, mx := range mxs {
				mxHostname := strings.TrimRight(mx.Host, ".")
				log.Debug("service#%d look up mx domain %s for %s", id, mxHostname, event.Message.HostnameTo)
				mxServer := new(MxServer)
				mxServer.hostname = mxHostname
				// по умолчанию создаем с безлимитным количеством соединений, т.к. мы не знаем заранее об ограничениях почтовых сервисов
				mxServer.maxConnections = common.UnlimitedConnectionCount
				mxServer.ips = make([]net.IP, 0)
				mxServer.clients = make([]*common.SmtpClient, 0)
				// по умолчанию будем создавать TLS соединение
				mxServer.useTLS = true
				// собираем IP адреса для сертификата и проверок
				ips, err := net.LookupIP(mxHostname)
				if err == nil {
					for _, ip := range ips {
						// берем только IPv4
						ip = ip.To4()
						if ip != nil {
							log.Debug("service#%d look up ip %s for %s", id, ip.String(), mxHostname)
							existsIpsLen := len(mxServer.ips)
							index := sort.Search(existsIpsLen, func(i int) bool {
								return mxServer.ips[i].Equal(ip)
							})
							// избавляемся от повторяющихся IP адресов
							if existsIpsLen == 0 || (index == -1 && existsIpsLen > 0) {
								mxServer.ips = append(mxServer.ips, ip)
							}
						}
					}
					// домен почтового ящика может отличаться от домена почтового сервера,
					// а домен почтового сервера может отличаться от реальной A записи сервера,
					// на котором размещен этот почтовый сервер
					// нам необходимо получить реальный домен, для того чтобы подписать на него сертификат
					for _, ip := range mxServer.ips {
						// пытаемся получить адреса сервера
						addrs, err := net.LookupAddr(ip.String())
						if err == nil {
							for _, addr := range addrs {
								// адрес получаем с точкой на конце, убираем ее
								addr = strings.TrimRight(addr, ".")
								// отсекаем адрес, если это IP
								if net.ParseIP(addr) == nil {
									log.Debug("service#%d look up addr %s for ip %s", id, addr, ip.String())
									if len(mxServer.realServerName) == 0 {
										// пытаем найти домен почтового сервера в домене почты
										hostnameMatched, _ := regexp.MatchString(event.Message.HostnameTo, mxServer.hostname)
										// пытаемся найти адрес в домене почтового сервиса
										addrMatched, _ := regexp.MatchString(mxServer.hostname, addr)
										// если найден домен почтового сервера в домене почты
										// тогда в адресе будет PTR запись
										if hostnameMatched && !addrMatched {
											mxServer.realServerName = addr
										} else if !hostnameMatched && addrMatched || !hostnameMatched && !addrMatched { // если найден адрес в домене почтового сервиса или нет совпадений
											mxServer.realServerName = mxServer.hostname
										}
									}
								}
							}
						} else {
							log.Warn("service#%d can't look up addr for ip %s", id, ip.String())
						}
					}
				} else {
					log.Warn("service#%d can't look up ips for mx %s", id, mxHostname)
				}
				if len(mxServer.realServerName) == 0 { // если безвыходная ситуация
					mxServer.realServerName = mxServer.hostname
				}
				log.Debug("service#%d look up detect real server name %s", id, mxServer.realServerName)
				mailServer.mxServers[i] = mxServer
			}
			mailServer.lastIndex = len(mailServer.mxServers) - 1
			mailServer.status = SuccessMailServerStatus
			log.Debug("service#%d look up %s success", id, event.Message.HostnameTo)
		} else {
			mailServer.status = ErrorMailServerStatus
			log.Warn("service#%d can't look up mx domains for %s", id, event.Message.HostnameTo)
		}
	}
	event.MailServers <- mailServer
}

func (c *Service) receiveConnections(id int) {
	for event := range c.connectEvents {
		c.receiveConnection(id, event)
	}
}

func (c *Service) receiveConnection(id int, event *common.SendEvent) {
	log.Debug("service#%d find connection for mail#%d", id, event.Message.Id)
	goto receiveConnect

receiveConnect:
	event.TryCount++
	var targetClient *common.SmtpClient
	for _, mxServer := range event.MailServer.mxServers {
		log.Debug("service#%d check connections for %s", id, mxServer.hostname)
		for _, client := range mxServer.clients {
			if atomic.LoadInt32(&(client.Status)) == WaitingSmtpClientStatus {
				atomic.StoreInt32(&(client.Status), WorkingSmtpClientStatus)
				client.SetTimeout(MailTimeout)
				targetClient = client
				log.Debug("service%d found smtp client#%d", id, client.Id)
				break
			}
		}
		if targetClient == nil && mxServer.maxConnections == common.UnlimitedConnectionCount {
			log.Debug("service#%d can't find free connections for %s, create", id, mxServer.hostname)
			mxServer.createNewSmtpClient(id, event, &targetClient, mxServer.createTLSSmtpClient)
		}
	}
	if targetClient == nil {
		goto waitConnect
		return
	} else {
		event.Client = targetClient
		mailer.events <- event
		return
	}

waitConnect:
	if event.TryCount >= common.TryConnectCount {
		ReturnMail(
			event,
			errors.New(fmt.Sprintf("service#%d can't connect to %s", id, event.Message.HostnameTo)),
		)
		return
	} else {
		log.Debug("service#%d can't find free connections, wait...", id)
		time.Sleep(SleepTimeout)
		goto receiveConnect
		return
	}
}

// завершает работу сервиса соединений
func (c *Service) OnFinish() {
	close(c.events)
	// останавливаем таймер
	c.ticker.Stop()
	// закрываем все соединения
	c.closeConnections(time.Now().Add(time.Minute))
}
