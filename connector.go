package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"net"
	"net/smtp"
	"sync"
	"time"
	"crypto/x509"
	"io/ioutil"
	"encoding/pem"
	"crypto/tls"
	"sort"
	"errors"
	"fmt"
	"regexp"
	"math/rand"
	"strings"
	"sync/atomic"
)

const (
	UNLIMITED_CONNECTION_COUNT = -1                     // безлимитное количество соединений к почтовому сервису
	RECEIVE_CONNECTION_TIMEOUT = 5 * time.Minute        // время ожидания для получения соединения к почтовому сервису
	SLEEP_TIMEOUT              = 1000 * time.Millisecond
	HELLO_TIMEOUT              = 5 * time.Minute
	MAIL_TIMEOUT               = 5 * time.Minute
	RCPT_TIMEOUT               = 5 * time.Minute
	DATA_TIMEOUT               = 10 * time.Minute
	WAITING_TIMEOUT            = 30 * time.Second
	TRY_CONNECT_COUNT          = 10
)

var (
	connector *Connector
)

// Сервис, управляющий соединениями к почтовым сервисам.
// Письма могут отсылаться в несколько потоков, почтовый сервис может разрешить несколько подключений с одного IP.
// Количество подключений может быть не равно количеству отсылающих потоков.
// Если доверить управление подключениями отправляющим потокам, тогда это затруднит общее управление подключениями.
// Поэтому создание подключений и предоставление имеющихся подключений отправляющим потокам вынесено в отдельный сервис.
type Connector struct {
	ConnectorsCount    int                    `yaml:"workers"`
	// путь до файла с закрытым ключом
	PrivateKeyFilename string                 `yaml:"privateKey"`
	// путь до файла с сертификатом
	CertFilename       string                 `yaml:"certificate"`
	// ip с которых будем рассылать письма
	Addresses          []string               `yaml:"ips"`
	addressesLen       int
	// почтовые сервисы
	mailServers        map[string]*MailServer
	// семафор, необходим для создания и поиска соединений
	mutex              *sync.Mutex
	// таймер, необходим для проверки открытых соединений
	ticker             *time.Ticker
	// сертификат в байтах
	certBytes          []byte
	// длина сертификата
	certBytesLen       int
	events             chan *SendEvent
	lookupEvents       chan *SendEvent
	connectEvents      chan *SendEvent
}

// создает новый сервис соединений
func ConnectorOnce() *Connector {
	if connector == nil {
		connector = new(Connector)
		// почтовые сервисы будут хранится в карте по домену
		connector.mailServers = make(map[string]*MailServer)
		connector.mutex = new(sync.Mutex)
		// создаем таймер
		connector.ticker = time.NewTicker(6 * time.Second)
		connector.certBytes = []byte{}
		connector.events = make(chan *SendEvent)
		connector.lookupEvents = make(chan *SendEvent)
		connector.connectEvents = make(chan *SendEvent)
	}
	return connector
}

// по срабатыванию таймера, просматривает все соединения к почтовым сервисам
// и закрывает те, которые висят дольше 30 секунд
func (c *Connector) checkConnections() {
	for now := range c.ticker.C {
		go c.closeConnections(now)
	}
}

func (c *Connector) closeConnections(now time.Time) {
	for _, mailServer := range c.mailServers {
		// закрываем соединения к каждого почтового сервиса
		go mailServer.closeConnections(now)
	}
}

func (c *Connector) OnInit(event *ApplicationEvent) {
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
				FailExit("connector can't read certificate, error - %v", err)
			}
		} else {
			Debug("certificate is not defined")
		}
		c.addressesLen = len(c.Addresses)
		if c.addressesLen == 0 {
			FailExit("ips should be defined")
		}
		if c.ConnectorsCount == 0 {
			c.ConnectorsCount = defaultWorkersCount
		}
	} else {
		FailExit("connector can't unmarshal config, error - %v", err)
	}
}

func (c *Connector) OnRun() {
	// запускаем проверку открытых соединений
	go connector.checkConnections()
	for i := 0;i < c.ConnectorsCount; i++ {
		id := i + 1
		go c.receiveConnections(id)
		go c.lookupServers(id)
		go c.createConnections(id)
	}
}

func (c *Connector) createConnections(id int) {
	for event := range c.events {
		c.doConnection(id, event)
	}
}

func (c *Connector) doConnection(id int, event *SendEvent) {
	Info("connector#%d try create connection for mail#%d", id, event.Message.Id)
	// передаем событию сертификат и его длину
	event.CertBytes = c.certBytes
	event.CertBytesLen = c.certBytesLen
	goto connectToMailServer

connectToMailServer:
	c.lookupEvents <- event
	mailServer := <- event.MailServers
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
			errors.New(fmt.Sprintf("511 connector#%d can't lookup %s", id, event.Message.HostnameTo)),
		)
		return
	}

waitLookup:
	Debug("connector#%d wait ending look up mail server %s...", id, event.Message.HostnameTo)
	time.Sleep(SLEEP_TIMEOUT)
	goto connectToMailServer
}

func (c *Connector) lookupServers(id int) {
	for event := range c.lookupEvents {
		c.lookupServer(id, event)
	}
}

func (c *Connector) lookupServer(id int, event *SendEvent) {
	c.mutex.Lock()
	if _, ok := c.mailServers[event.Message.HostnameTo]; !ok {
		Debug("connector#%d create mail server for %s", id, event.Message.HostnameTo)
		c.mailServers[event.Message.HostnameTo] = &MailServer{
			status: LookupMailServerStatus,
			connectorId: id,
		}
	}
	c.mutex.Unlock()
	mailServer := c.mailServers[event.Message.HostnameTo]
	if id == mailServer.connectorId && mailServer.status == LookupMailServerStatus {
		Debug("connector#%d look up mx domains for %s...", id, event.Message.HostnameTo)
		mailServer := c.mailServers[event.Message.HostnameTo]
		// ищем почтовые сервера для домена
		mxs, err := net.LookupMX(event.Message.HostnameTo)
		if err == nil {
			mailServer.mxServers = make([]*MxServer, len(mxs))
			for i, mx := range mxs {
				mxHostname := strings.TrimRight(mx.Host, ".")
				Debug("connector#%d look up mx domain %s for %s", id, mxHostname, event.Message.HostnameTo)
				mxServer := new(MxServer)
				mxServer.hostname = mxHostname
				// по умолчанию создаем с безлимитным количеством соединений, т.к. мы не знаем заранее об ограничениях почтовых сервисов
				mxServer.maxConnections = UNLIMITED_CONNECTION_COUNT
				mxServer.ips = make([]net.IP, 0)
				mxServer.clients = make([]*SmtpClient, 0)
				// по умолчанию будем создавать TLS соединение
				mxServer.useTLS = true
				// собираем IP адреса для сертификата и проверок
				ips, err := net.LookupIP(mxHostname)
				if err == nil {
					for _, ip := range ips {
						// берем только IPv4
						ip = ip.To4()
						if ip != nil {
							Debug("connector#%d look up ip %s for %s", id, ip.String(), mxHostname)
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
									Debug("connector#%d look up addr %s for ip %s", id, addr, ip.String())
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
							Warn("connector#%d can't look up addr for ip %s", id, ip.String())
						}
					}
				} else {
					Warn("connector#%d can't look up ips for mx %s", id, mxHostname)
				}
				if len(mxServer.realServerName) == 0 { // если безвыходная ситуация
					mxServer.realServerName = mxServer.hostname
				}
				Debug("connector#%d look up detect real server name %s", id, mxServer.realServerName)
				mailServer.mxServers[i] = mxServer
			}
			mailServer.lastIndex = len(mailServer.mxServers) - 1
			mailServer.status = SuccessMailServerStatus
			Debug("connector#%d look up %s success", id, event.Message.HostnameTo)
		} else {
			mailServer.status = ErrorMailServerStatus
			Warn("connector#%d can't look up mx domains for %s", id, event.Message.HostnameTo)
		}
	}
	event.MailServers <- mailServer
}

func (c *Connector) receiveConnections(id int) {
	for event := range c.connectEvents {
		c.receiveConnection(id, event)
	}
}

func (c *Connector) receiveConnection(id int, event *SendEvent) {
	Debug("connector#%d find connection for mail#%d", id, event.Message.Id)
	goto receiveConnect

receiveConnect:
	event.TryCount++
	var targetClient *SmtpClient
	for _, mxServer := range event.MailServer.mxServers {
		Debug("connector#%d check connections for %s", id, mxServer.hostname)
		Debug("connector#%d - %s has %d clients", id, mxServer.hostname, len(mxServer.clients))
		for _, client := range mxServer.clients {
			status := atomic.LoadInt32(&(client.Status))
			Debug("connector#%d - client#%d has status %v", id, client.Id, status)
			if status == SMTP_CLIENT_STATUS_WAITING {
				atomic.StoreInt32(&(client.Status), SMTP_CLIENT_STATUS_WORKING)
				client.SetTimeout(MAIL_TIMEOUT)
				targetClient = client
				Debug("connector%d found smtp client#%d", id, client.Id)
				break
			}
		}
		if targetClient == nil {
			Debug("connector#%d can't find free connections for %s", id, mxServer.hostname)
			maxConnections := mxServer.getMaxConnections()
			if maxConnections == UNLIMITED_CONNECTION_COUNT {
				Debug("connector#%d create new connection to %s", id, mxServer.hostname)
				mxServer.createNewSmtpClient(id, event, &targetClient, mxServer.createTLSSmtpClient)
			} else {
				Debug("connector#%d can't create more %d connections to %s", id, maxConnections, mxServer.hostname)
			}
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
	if event.TryCount >= TRY_CONNECT_COUNT {
		ReturnMail(
			event,
			errors.New(fmt.Sprintf("connector#%d can't connect to %s", id, event.Message.HostnameTo)),
		)
		return
	} else {
		Debug("connector#%d can't find free connections, wait...", id)
		time.Sleep(SLEEP_TIMEOUT)
		goto receiveConnect
		return
	}
}

// завершает работу сервиса соединений
func (c *Connector) OnFinish() {
	close(c.events)
	// останавливаем таймер
	c.ticker.Stop()
	// закрываем все соединения
	c.closeConnections(time.Now().Add(time.Minute))
}

type MailServerStatus int

const (
	LookupMailServerStatus MailServerStatus = iota
	SuccessMailServerStatus
	ErrorMailServerStatus
)

// почтовый сервис
type MailServer struct {
	mxServers   []*MxServer      // серверы почтового сервиса
	lastIndex   int              // индекс последнего почтового сервиса
	connectorId int              // номер потока, собирающего информацию о почтовом сервисе
	status      MailServerStatus // статус, говорящий о том, собранали ли информация о почтовом сервисе
}

// закрывает соединения почтового сервиса
func (this *MailServer) closeConnections(now time.Time) {
	if this.mxServers != nil && len(this.mxServers) > 0 {
		for _, mxServer := range this.mxServers {
			if mxServer != nil {
				go mxServer.closeConnections(now)
			}
		}
	}
}

// почтовый сервер
type MxServer struct {
	hostname       string        // доменное имя почтового сервера
	maxConnections int32         // количество подключений для одного IP
	ips            []net.IP      // IP сервера
	clients        []*SmtpClient // клиенты сервера
	realServerName string        // А запись сервера
	useTLS         bool          // использоватение TLS
}

// создает новое TLS или обычное соединение
func (this *MxServer) createNewSmtpClient(id int, event *SendEvent, ptrSmtpClient **SmtpClient, callback func(id int, event *SendEvent, ptrSmtpClient **SmtpClient, connection net.Conn, client *smtp.Client)) {
	// создаем соединение
	rand.Seed(time.Now().UnixNano())
	addr := connector.Addresses[rand.Intn(connector.addressesLen)]
	tcpAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(addr, "0"))
	if err == nil {
		Debug("connector#%d resolve tcp address %s", id, tcpAddr.String())
		dialer := &net.Dialer{
			Timeout: HELLO_TIMEOUT,
			LocalAddr: tcpAddr,
		}
		hostname := net.JoinHostPort(this.hostname, "25")
		connection, err := dialer.Dial("tcp", hostname)
		if err == nil {
			Debug("connector#%d connect to %s", id, hostname)
			connection.SetDeadline(time.Now().Add(HELLO_TIMEOUT))
			// создаем клиента
			client, err := smtp.NewClient(connection, this.hostname)
			if err == nil {
				Debug("connector#%d create client to %s", id, this.hostname)
				// здороваемся
				err = client.Hello(event.Message.HostnameFrom)
				if err == nil {
					Debug("connector#%d send command HELLO %s", id, event.Message.HostnameFrom)
					// создаем TLS или обычное соединение
					if this.useTLS {
						this.useTLS, _ = client.Extension("STARTTLS")
					}
					Debug("connector#%d use TLS %v", id, this.useTLS)
					callback(id, event, ptrSmtpClient, connection, client)
				} else {
					client.Quit()
					Debug("connector#%d can't send command HELLO %s to %s", id, event.Message.HostnameFrom, this.hostname)
				}
			} else {
				connection.Close()
				this.updateMaxConnections(id, err)
				Debug("connector#%d can't create client to %s", id, this.hostname)
			}
		} else {
			this.updateMaxConnections(id, err)
			Debug("connector#%d can't connect to %s", id, hostname)
		}
	} else {
		Debug("connector#%d can't resolve tcp address %s:0", id, addr)
	}
}

// создает новое TLS соединение к почтовому серверу
func (this *MxServer) createTLSSmtpClient(id int, event *SendEvent, ptrSmtpClient **SmtpClient, connection net.Conn, client *smtp.Client) {
	// если есть какие данные о сертификате и к серверу можно создать TLS соединение
	if event.CertBytesLen > 0 && this.useTLS {
		pool := x509.NewCertPool()
		// пытаем создать сертификат
		cert, err := x509.ParseCertificate(event.CertBytes)
		if err == nil {
			// задаем сертификату IP сервера
			cert.IPAddresses = this.ips
			pool.AddCert(cert)
			// открываем TLS соединение
			err = client.StartTLS(&tls.Config {
				ClientCAs : pool,
				ServerName: this.realServerName,
			})
			// если все нормально, создаем клиента
			if err == nil {
				this.createSmtpClient(id, ptrSmtpClient, connection, client)
			} else { // если не удалось создать TLS соединение
				// говорим, что не надо больше создавать TLS соединение
				this.dontUseTLS(err)
				// разрываем созданое соединение
				// это необходимо, т.к. не все почтовые сервисы позволяют продолжить отправку письма
				// после неудачной попытке создать TLS соединение
				client.Quit()
				// создаем обычное соединие
				this.createNewSmtpClient(id, event, ptrSmtpClient, this.createPlainSmtpClient)
			}
		} else {
			this.dontUseTLS(err)
			this.createPlainSmtpClient(id, event, ptrSmtpClient, connection, client)
		}
	} else {
		this.createPlainSmtpClient(id, event, ptrSmtpClient, connection, client)
	}
}

// создает новое соединие к почтовому серверу
func (this *MxServer) createPlainSmtpClient(id int, event *SendEvent, ptrSmtpClient **SmtpClient, connection net.Conn, client *smtp.Client) {
	this.createSmtpClient(id, ptrSmtpClient, connection, client)
}

// создает нового клиента почтового сервера
func (this *MxServer) createSmtpClient(id int, ptrSmtpClient **SmtpClient, connection net.Conn, client *smtp.Client) {
	(*ptrSmtpClient) = new(SmtpClient)
	(*ptrSmtpClient).Id = len(this.clients) + 1
	(*ptrSmtpClient).connection = connection
	(*ptrSmtpClient).Worker = client
	(*ptrSmtpClient).createDate = time.Now()
	(*ptrSmtpClient).Status = SMTP_CLIENT_STATUS_WORKING
	this.clients = append(this.clients, (*ptrSmtpClient))
	Debug("connector#%d create smtp client#%d for %s", id, (*ptrSmtpClient).Id, this.hostname)
}

// обновляет количество максимальных соединений
// пишет в лог количество максимальных соединений и ошибку, возникшую при попытке открыть новое соединение
func (this *MxServer) updateMaxConnections(id int, err error) {
	clientsLen := int32(len(this.clients))
	this.setMaxConnections(clientsLen)
	Warn("connector#%d detect max %d open connections for %s, error - %v", id, clientsLen, this.hostname, err)
}

func (this *MxServer) getMaxConnections() int32 {
	return atomic.LoadInt32(&(this.maxConnections))
}

func (this *MxServer) setMaxConnections(maxConnections int32) {
	atomic.StoreInt32(&(this.maxConnections), maxConnections)
}

func (this *MxServer) dropMaxConnections() {
	if this.getMaxConnections() != UNLIMITED_CONNECTION_COUNT {
		this.setMaxConnections(UNLIMITED_CONNECTION_COUNT)
	}
}

// закрывает свои собственные соединения
func (this *MxServer) closeConnections(now time.Time) {
	if this.clients != nil && len(this.clients) > 0 {
		Debug("mx server %s has %d clients before checking", this.hostname, len(this.clients))
		clients := make([]*SmtpClient, 0)
		for _, client := range this.clients {
			// если соединение свободно и висит в таком статусе дольше 30 секунд, закрываем соединение
			status := atomic.LoadInt32(&(client.Status))
			isExpire := client.IsExpire(now)
			Debug("mx server %s smtp client#%d has status %d and expire %v", this.hostname, client.Id, status, isExpire)
			if (status == SMTP_CLIENT_STATUS_WAITING && isExpire) || status == SMTP_CLIENT_STATUS_EXPIRE {
				atomic.StoreInt32(&(client.Status), SMTP_CLIENT_STATUS_DISCONNECTED)
				err := client.Worker.Close()
				if err != nil {
					WarnWithErr(err)
				}
				this.dropMaxConnections()
				Debug("close connection smtp client#%d mx server %s", client.Id, this.hostname)
			} else {
				clients = append(clients, client)
			}
		}
		this.clients = append([]*SmtpClient{}, clients...)
		Debug("mx server %s has %d clients after checking", this.hostname, len(this.clients))
	} else {
		this.dropMaxConnections()
	}
}

// запрещает использовать TLS соединения
// и пишет в лог и ошибку, возникшую при попытке открыть TLS соединение
func (this *MxServer) dontUseTLS(err error) {
	this.useTLS = false
	WarnWithErr(err)
}

// статус клиента почтового сервера
const (
	// отсылает письмо
	SMTP_CLIENT_STATUS_WORKING      int32 = iota
	// ожидает письма
	SMTP_CLIENT_STATUS_WAITING
	SMTP_CLIENT_STATUS_EXPIRE
	// отсоединен
	SMTP_CLIENT_STATUS_DISCONNECTED
)

// клиент почтового сервера
type SmtpClient struct {
	Id          int          // номер клиента для удобства в логах
	connection  net.Conn     // соединение к почтовому серверу
	Worker      *smtp.Client // реальный smtp клиент
	createDate  time.Time    // дата создания или изменения статуса клиента
	Status      int32        // статус
}

func (this *SmtpClient) SetTimeout(timeout time.Duration) {
	this.connection.SetDeadline(time.Now().Add(timeout))
}

func (this *SmtpClient) IsExpireByNow() bool {
	return this.IsExpire(time.Now())
}

func (this *SmtpClient) IsExpire(now time.Time) bool {
	return now.Sub(this.createDate) >= WAITING_TIMEOUT
}
