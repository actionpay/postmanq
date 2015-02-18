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
)

const (
	UNLIMITED_CONNECTION_COUNT = -1               // безлимитное количество соединений к почтовому сервису
	CONNECTION_TIMEOUT         = 30 * time.Second // время ожидания неиспользуемого соединения
	RECEIVE_CONNECTION_TIMEOUT = 5 * time.Second  // время ожидания получения соединения к почтовому сервису
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
	// путь до файла с закрытым ключом
	PrivateKeyFilename string                 `yaml:"privateKey"`
	// путь до файла с сертификатом
	CertFilename       string                 `yaml:"certificate"`
	// почтовые сервисы
	mailServers        map[string]*MailServer
	// семафор, необходим для создания и поиска соединений
	mutex              *sync.Mutex
	// таймер, необходим для проверки открытых соединений
	ticker             *time.Ticker
	// пул сертификатов
	certPool           *x509.CertPool
	// сертификат в байтах
	certBytes          []byte
	// длина сертификата
	certBytesLen       int
}

// создает новый сервис соединений
func NewConnector() *Connector {
	if connector == nil {
		connector = new(Connector)
		// почтовые сервисы будут хранится в карте по домену
		connector.mailServers = make(map[string]*MailServer)
		connector.mutex = new(sync.Mutex)
		// создаем таймер
		connector.ticker = time.NewTicker(11 * time.Second)
		connector.certBytes = []byte{}
		// запускаем проверку открытых соединений
		go connector.checkConnections()
	}
	return connector
}

// по срабатыванию таймера, просматривает все соединения к почтовым сервисам
// и закрывает те, которые висят дольше 30 секунд
func (this *Connector) checkConnections() {
	Debug("check opened smtp connections...")
	for now := range this.ticker.C {
		go this.closeConnections(now)
	}
}

func (this *Connector) closeConnections(now time.Time) {
	Debug("check opened smtp connections...")
	for _, mailServer := range this.mailServers {
		// закрываем соединения к каждого почтового сервиса
		go mailServer.closeConnections(now)
	}
}

func (this *Connector) OnInit(event *InitEvent) {
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		// если указан путь до сертификата
		if len(this.CertFilename) > 0 {
			// пытаемся прочитать сертификат
			pemBytes, err := ioutil.ReadFile(this.CertFilename)
			if err == nil {
				// получаем сертификат
				pemBlock, _ := pem.Decode(pemBytes)
				this.certBytes = pemBlock.Bytes
				// и считаем его длину, чтобы не делать это при создании каждого сертификата
				this.certBytesLen = len(this.certBytes)
			} else {
				FailExitWithErr(err)
			}
		} else {
			Debug("certificate is not defined")
		}
	} else {
		FailExitWithErr(err)
	}
}

func (this *Connector) OnRun() {}

// завершает работу сервиса соединений
func (this *Connector) OnFinish() {
	// останавливаем таймер
	this.ticker.Stop()
	// закрываем все соединения
	this.closeConnections(time.Now().Add(time.Minute))
}

// при отправке письма ищет созданное подключение для отправке письма или создает новое
func (this *Connector) OnSend(event *SendEvent) {
	this.mutex.Lock()
	if this.certPool != nil {
		event.CertPool = this.certPool
	}
	// передаем событию сертификат и его длину
	event.CertBytes = this.certBytes
	event.CertBytesLen = this.certBytesLen
	hostname := event.Message.HostnameTo
	// если это новый почтовый сервис, собираем информацию о его серверах
	if _, ok := this.mailServers[hostname]; !ok {
		this.lookupMxServers(hostname)
	}
	// пытаемся найти свободное подключения для отправки письма
	if mailServer, ok := this.mailServers[hostname]; ok {
		mailServer.findSmtpClient(event)
	}
	if event.Client == nil || (event.Client != nil && event.Client.Worker == nil) {
		ReturnMail(event.Message, errors.New(fmt.Sprintf("can't find connection for %s", event.Message.Recipient)))
		event.DefaultPrevented = true
	}
	this.mutex.Unlock()
}

// собирает информацию о серверах почтового сервиса
func (this *Connector) lookupMxServers(hostname string) {
	Debug("lookup mx domains for %s...", hostname)
	// ищем почтовые сервера для домена
	mxs, err := net.LookupMX(hostname)
	if err == nil {
		mailServer := new(MailServer)
		mailServer.mxServers = make([]*MxServer, len(mxs))
		for i, mx := range mxs {
			mxHostname := mx.Host
			mxHostnameLen := len(mxHostname)
			// домен получаем с точкой на конце, убираем ее
			if mxHostname[mxHostnameLen - 1:mxHostnameLen] == "." {
				mxHostname = mxHostname[:mxHostnameLen - 1]
			}
			Debug("lookup mx domain %s for %s", mxHostname, hostname)
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
						Debug("lookup mx ip %s for %s", ip.String(), hostname)
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
							addrLen := len(addr)
							// адрес получаем с точкой на конце, убираем ее
							if addr[addrLen - 1:addrLen] == "." {
								addr = addr[:addrLen - 1]
							}
							// отсекаем адрес, если это IP
							if net.ParseIP(addr) == nil {
								Debug("lookup addr %s for ip %s", addr, ip.String())
								if len(mxServer.realServerName) == 0 {
									// пытаем найти домен почтового сервера в домене почты
									hostnameMatched, _ := regexp.MatchString(hostname, mxServer.hostname)
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
						WarnWithErr(err)
					}
				}
			} else {
				WarnWithErr(err)
			}
			if len(mxServer.realServerName) == 0 { // если безвыходная ситуация
				mxServer.realServerName = mxServer.hostname
			}
			Debug("real server name %s", mxServer.realServerName)
			mailServer.mxServers[i] = mxServer
		}
		mailServer.lastIndex = len(mailServer.mxServers) - 1
		this.mailServers[hostname] = mailServer
	} else {
		WarnWithErr(err)
	}
}

// почтовый сервис
type MailServer struct {
	mxServers []*MxServer // серверы почтового сервиса
	lastIndex int         //
}

// ищет свободное для отправки соединение
func (this *MailServer) findSmtpClient(event *SendEvent) {
	var targetSmtpClient *SmtpClient
	mxServersIndex := 0
	// ищем соединение 5 секунд, пока не найдем первое свободное
	for targetSmtpClient == nil && time.Now().Sub(event.CreateDate) <= RECEIVE_CONNECTION_TIMEOUT {
		mxServer := this.mxServers[mxServersIndex]
		// сначала пытаемся найти уже открытое свободное соединение
		mxServer.findFreeSmtpServers(&targetSmtpClient)
		// если нет свободных соединений и мы не знаем, есть ли ограничение на количество соединений
		// пытаемся открыть новое соединение к почтовому сервису
		if targetSmtpClient == nil && mxServer.maxConnections == UNLIMITED_CONNECTION_COUNT {
			Debug("create smtp client for %s", mxServer.hostname)
			// пытаемся создать новое TLS соединение
			mxServer.createNewSmtpClient(event, &targetSmtpClient, mxServer.createTLSSmtpClient)
			if targetSmtpClient != nil {
				break
			}
		}
		// если соединение не найдено и не создано,
		// пытаемся найти соединение на другом почтовом сервере
		if mxServersIndex == this.lastIndex {
			mxServersIndex = 0
		} else {
			mxServersIndex++
		}
	}
	// если соединение не найдено, ругаемся, отменяем отправку письма
	if targetSmtpClient == nil {
		event.DefaultPrevented = true
		ReturnMail(event.Message, errors.New(fmt.Sprintf("can't find free connection for mail#%d", event.Message.Id)))
	} else {
		event.Client = targetSmtpClient
	}
}

// закрывает соединения почтового сервиса
func (this *MailServer) closeConnections(now time.Time) {
	for _, mxServer := range this.mxServers {
		go mxServer.closeConnections(now)
	}
}

// почтовый сервер
type MxServer struct {
	hostname       string        // доменное имя почтового сервера
	maxConnections int           // количество подключений для одного IP
	ips            []net.IP      // IP сервера
	clients        []*SmtpClient // клиенты сервера
	realServerName string        // А запись сервера
	useTLS         bool          // использоватение TLS
}

// ищет свободное соединение
func (this *MxServer) findFreeSmtpServers(targetSmtpClient **SmtpClient) {
	Debug("search free smtp clients for %s...", this.hostname)
	for _, smtpClient := range this.clients {
		if smtpClient.Status == SMTP_CLIENT_STATUS_WAITING {
			smtpClient.modifyDate = time.Now()
			smtpClient.Status = SMTP_CLIENT_STATUS_WORKING
			(*targetSmtpClient) = smtpClient
			Debug("found smtp client#%d", smtpClient.Id)
			break
		}
	}
	Debug("free smtp clients not found for %s", this.hostname)
}

// создает новое TLS или обычное соединение
func (this *MxServer) createNewSmtpClient(event *SendEvent, ptrSmtpClient **SmtpClient, callback func(event *SendEvent, ptrSmtpClient **SmtpClient, connection net.Conn, client *smtp.Client)) {
	// создаем соединение
	connection, err := net.Dial("tcp", net.JoinHostPort(this.hostname, "25"))
	if err == nil {
		// создаем клиента
		client, err := smtp.NewClient(connection, event.Message.HostnameFrom)
		if err == nil {
			// здороваемся
			err = client.Hello(event.Message.HostnameFrom)
			if err == nil {
				// создаем TLS или обычное соединение
				callback(event, ptrSmtpClient, connection, client)
			} else {
				client.Quit()
				this.updateMaxConnections(err)
			}
		} else {
			connection.Close()
			this.updateMaxConnections(err)
		}
	} else {
		this.updateMaxConnections(err)
	}
}

// создает новое TLS соединение к почтовому серверу
func (this *MxServer) createTLSSmtpClient(event *SendEvent, ptrSmtpClient **SmtpClient, connection net.Conn, client *smtp.Client) {
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
				this.createSmtpClient(ptrSmtpClient, connection, client)
			} else { // если не удалось создать TLS соединение
				// говорим, что не надо больше создавать TLS соединение
				this.dontUseTLS(err)
				// разрываем созданое соединение
				// это необходимо, т.к. не все почтовые сервисы позволяют продолжить отправку письма
				// после неудачной попытке создать TLS соединение
				client.Quit()
				// создаем обычное соединие
				this.createNewSmtpClient(event, ptrSmtpClient, this.createPlainSmtpClient)
			}
		} else {
			this.dontUseTLS(err)
			this.createSmtpClient(ptrSmtpClient, connection, client)
		}
	} else {
		this.createSmtpClient(ptrSmtpClient, connection, client)
	}
}

// создает новое соединие к почтовому серверу
func (this *MxServer) createPlainSmtpClient(event *SendEvent, ptrSmtpClient **SmtpClient, connection net.Conn, client *smtp.Client) {
	this.createSmtpClient(ptrSmtpClient, connection, client)
}

// создает нового клиента почтового сервера
func (this *MxServer) createSmtpClient(ptrSmtpClient **SmtpClient, connection net.Conn, client *smtp.Client) {
	(*ptrSmtpClient) = new(SmtpClient)
	(*ptrSmtpClient).Id = len(this.clients) + 1
	(*ptrSmtpClient).connection = connection
	(*ptrSmtpClient).Worker = client
	(*ptrSmtpClient).modifyDate = time.Now()
	(*ptrSmtpClient).Status = SMTP_CLIENT_STATUS_WORKING
	this.clients = append(this.clients, (*ptrSmtpClient))
	Debug("smtp client#%d created for %s", (*ptrSmtpClient).Id, this.hostname)
}

// обновляет количество максимальных соединений
// пишет в лог количество максимальных соединений и ошибку, возникшую при попытке открыть новое соединение
func (this *MxServer) updateMaxConnections(err error) {
	this.maxConnections = len(this.clients)
	Debug("max %d smtp clients for %s, wait...", this.maxConnections, this.hostname)
	WarnWithErr(err)
}

// закрывает свои собственные соединения
func (this *MxServer) closeConnections(now time.Time) {
	for i, smtpClient := range this.clients {
		// если соединение свободно и висит в таком статусе дольше 30 секунд, закрываем соединение
		if smtpClient.Status == SMTP_CLIENT_STATUS_WAITING && now.Sub(smtpClient.modifyDate) >= CONNECTION_TIMEOUT {
			smtpClient.Status = SMTP_CLIENT_STATUS_DISCONNECTED
			err := smtpClient.Worker.Close()
			if err != nil {
				WarnWithErr(err)
			}
			this.clients = this.clients[:i]
			if i < len(this.clients) - 1 {
				this.clients = append(this.clients, this.clients[i+1:]...)
			}
			if this.maxConnections != UNLIMITED_CONNECTION_COUNT {
				this.maxConnections = UNLIMITED_CONNECTION_COUNT
			}
			Debug("close connection smtp client#%d mx server %s", smtpClient.Id, this.hostname)
		}
	}
}

// запрещает использовать TLS соединения
// и пишет в лог и ошибку, возникшую при попытке открыть TLS соединение
func (this *MxServer) dontUseTLS(err error) {
	this.useTLS = false
	WarnWithErr(err)
}

// статус клиента почтового сервера
type SmtpClientStatus int

const (
	// отсылает письмо
	SMTP_CLIENT_STATUS_WORKING      SmtpClientStatus = iota
	// ожидает письма
	SMTP_CLIENT_STATUS_WAITING
	// отсоединен
	SMTP_CLIENT_STATUS_DISCONNECTED
)

// клиент почтового сервера
type SmtpClient struct {
	Id         int              // номер клиента для удобства в логах
	connection net.Conn         // соединение к почтовому серверу
	Worker     *smtp.Client     // реальный smtp клиент
	modifyDate time.Time        // дата создания или изменения статуса клиента
	Status     SmtpClientStatus // статус
}
