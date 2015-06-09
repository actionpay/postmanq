package connector

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/log"
	"math/rand"
	"net"
	"net/smtp"
	"sync/atomic"
	"time"
)

type MailServerStatus int

const (
	LookupMailServerStatus MailServerStatus = iota
	SuccessMailServerStatus
	ErrorMailServerStatus
)

// почтовый сервис
type MailServer struct {
	// серверы почтового сервиса
	mxServers []*MxServer

	// индекс последнего почтового сервиса
	lastIndex int

	// номер потока, собирающего информацию о почтовом сервисе
	connectorId int

	// статус, говорящий о том, собранали ли информация о почтовом сервисе
	status MailServerStatus
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
	// доменное имя почтового сервера
	hostname       string

	// количество подключений для одного IP
	maxConnections int

	// IP сервера
	ips            []net.IP

	// клиенты сервера
	clients        []*common.SmtpClient

	// А запись сервера
	realServerName string

	// использоватение TLS
	useTLS         bool
}

// создает новое TLS или обычное соединение
func (this *MxServer) createNewSmtpClient(id int, event *common.SendEvent, ptrSmtpClient **common.SmtpClient, callback func(id int, event *common.SendEvent, ptrSmtpClient **common.SmtpClient, connection net.Conn, client *smtp.Client)) {
	// создаем соединение
	rand.Seed(time.Now().UnixNano())
	addr := service.Addresses[rand.Intn(service.addressesLen)]
	tcpAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(addr, "0"))
	if err == nil {
		log.Debug("service#%d resolve tcp address %s", id, tcpAddr.String())
		dialer := &net.Dialer{
			Timeout:   common.HelloTimeout,
			LocalAddr: tcpAddr,
		}
		hostname := net.JoinHostPort(this.hostname, "25")
		log.Debug("service#%d dial to %s", id, hostname)
		connection, err := dialer.Dial("tcp", hostname)
		if err == nil {
			log.Debug("service#%d dialed to %s", id, hostname)
			connection.SetDeadline(time.Now().Add(common.HelloTimeout))
			// создаем клиента
			log.Debug("service#%d create client to %s", id, this.hostname)
			client, err := smtp.NewClient(connection, this.hostname)
			if err == nil {
				log.Debug("service#%d created client to %s", id, this.hostname)
				// здороваемся
				err = client.Hello(event.Message.HostnameFrom)
				if err == nil {
					log.Debug("service#%d send command HELLO: %s", id, event.Message.HostnameFrom)
					// создаем TLS или обычное соединение
					if this.useTLS {
						this.useTLS, _ = client.Extension("STARTTLS")
					}
					log.Debug("service#%d use TLS %v", id, this.useTLS)
					callback(id, event, ptrSmtpClient, connection, client)
				} else {
					client.Quit()
					this.updateMaxConnections(id, err)
				}
			} else {
				connection.Close()
				this.updateMaxConnections(id, err)
			}
		} else {
			this.updateMaxConnections(id, err)
		}
	} else {
		this.updateMaxConnections(id, err)
	}
}

// создает новое TLS соединение к почтовому серверу
func (this *MxServer) createTLSSmtpClient(id int, event *common.SendEvent, ptrSmtpClient **common.SmtpClient, connection net.Conn, client *smtp.Client) {
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
			err = client.StartTLS(&tls.Config{
				ClientCAs:  pool,
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
func (this *MxServer) createPlainSmtpClient(id int, event *common.SendEvent, ptrSmtpClient **common.SmtpClient, connection net.Conn, client *smtp.Client) {
	this.createSmtpClient(id, ptrSmtpClient, connection, client)
}

// создает нового клиента почтового сервера
func (this *MxServer) createSmtpClient(id int, ptrSmtpClient **common.SmtpClient, connection net.Conn, client *smtp.Client) {
	(*ptrSmtpClient) = new(common.SmtpClient)
	(*ptrSmtpClient).Id = len(this.clients) + 1
	(*ptrSmtpClient).connection = connection
	(*ptrSmtpClient).Worker = client
	(*ptrSmtpClient).createDate = time.Now()
	(*ptrSmtpClient).Status = common.WorkingSmtpClientStatus
	this.clients = append(this.clients, (*ptrSmtpClient))
	log.Debug("service#%d create smtp client#%d for %s", id, (*ptrSmtpClient).Id, this.hostname)
}

// обновляет количество максимальных соединений
// пишет в лог количество максимальных соединений и ошибку, возникшую при попытке открыть новое соединение
func (this *MxServer) updateMaxConnections(id int, err error) {
	clientsCount := len(this.clients)
	if clientsCount > 0 {
		this.maxConnections = clientsCount
	}
	log.Warn("service#%d detect max %d open connections for %s, error - %v", id, this.maxConnections, this.hostname, err)
}

// закрывает свои собственные соединения
func (this *MxServer) closeConnections(now time.Time) {
	if this.clients != nil && len(this.clients) > 0 {
		for i, client := range this.clients {
			// если соединение свободно и висит в таком статусе дольше 30 секунд, закрываем соединение
			status := atomic.LoadInt32(&(client.Status))
			if status == common.WaitingSmtpClientStatus && client.IsExpire(now) || status == common.ExpireSmtpClientStatus {
				client.Status = common.DisconnectedSmtpClientStatus
				err := client.Worker.Close()
				if err != nil {
					log.WarnWithErr(err)
				}
				this.clients = this.clients[:i]
				if i < len(this.clients)-1 {
					this.clients = append(this.clients, this.clients[i+1:]...)
				}
				if this.maxConnections != common.UnlimitedConnectionCount {
					this.maxConnections = common.UnlimitedConnectionCount
				}
				log.Debug("close connection smtp client#%d mx server %s", client.Id, this.hostname)
			}
		}
	}
}

// запрещает использовать TLS соединения
// и пишет в лог и ошибку, возникшую при попытке открыть TLS соединение
func (this *MxServer) dontUseTLS(err error) {
	this.useTLS = false
	log.WarnWithErr(err)
}
