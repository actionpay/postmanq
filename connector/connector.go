package connector

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/log"
	"math/rand"
	"net"
	"net/smtp"
	"time"
	"fmt"
	"errors"
)

var (
	connectorEvents = make(chan *ConnectionEvent)
)

type Connector struct {
	id int
}

func newConnector(id int) *Connector {
	return &Connector{id}
}

func (c *Connector) run() {
	for _, event := range connectorEvents {
		c.connect(event)
	}
}

func (c *Connector) connect(event *ConnectionEvent) {
	sendEvent := event.sendEvent
	log.Debug("service#%d find connection for mail#%d", c.id, sendEvent.Message.Id)
	goto receiveConnect

receiveConnect:
	sendEvent.TryCount++
	var targetClient *common.SmtpClient

	for _, mxServer := range event.server.mxServers {
		log.Debug("connector#%d try receive connection for %s", c.id, mxServer.hostname)

		if !mxServer.waitingQueue.Empty() {
			client := mxServer.waitingQueue.Pop()
			if client != nil {
				targetClient = client.(*common.SmtpClient)
				log.Debug("connector%d found free smtp client#%d", c.id, targetClient.Id)
				break
			}
		}

		if targetClient == nil && !mxServer.hasMaxConnections {
			log.Debug("connector#%d can't find free smtp client for %s", c.id, mxServer.hostname)
			c.createSmtpClient(mxServer, sendEvent, &targetClient)
		}
	}

	if targetClient == nil {
		goto waitConnect
		return
	} else {
		targetClient.SetTimeout(common.MailTimeout)
		sendEvent.Client = targetClient
		sendEvent.Iterator.Next().(common.SendingService).Events() <- sendEvent
		return
	}

waitConnect:
	if sendEvent.TryCount >= common.TryConnectCount {
		common.ReturnMail(
			sendEvent,
			errors.New(fmt.Sprintf("service#%d can't connect to %s", c.id, sendEvent.Message.HostnameTo)),
		)
		return
	} else {
		log.Debug("service#%d can't find free connections, wait...", c.id)
		time.Sleep(SleepTimeout)
		goto receiveConnect
		return
	}
}

func (c *Connector) createSmtpClient(mxServer *MxServer, event *common.SendEvent, ptrSmtpClient **common.SmtpClient) {
	// создаем соединение
	rand.Seed(time.Now().UnixNano())
	addr := service.Addresses[rand.Intn(service.addressesLen)]
	tcpAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(addr, "0"))
	if err == nil {
		log.Debug("service#%d resolve tcp address %s", c.id, tcpAddr.String())
		dialer := &net.Dialer{
			Timeout:   common.HelloTimeout,
			LocalAddr: tcpAddr,
		}
		hostname := net.JoinHostPort(mxServer.hostname, "25")
		connection, err := dialer.Dial("tcp", hostname)
		if err == nil {
			log.Debug("service#%d dial to %s", c.id, hostname)
			connection.SetDeadline(time.Now().Add(common.HelloTimeout))
			client, err := smtp.NewClient(connection, mxServer.hostname)
			if err == nil {
				log.Debug("service#%d create client to %s", c.id, mxServer.hostname)
				err = client.Hello(event.Message.HostnameFrom)
				if err == nil {
					log.Debug("service#%d send command HELLO: %s", c.id, event.Message.HostnameFrom)
					// создаем TLS или обычное соединение
					if mxServer.useTLS {
						mxServer.useTLS, _ = client.Extension("STARTTLS")
					}
					log.Debug("service#%d use TLS %v", c.id, mxServer.useTLS)
					if mxServer.useTLS {
						c.createTlsSmtpClient(mxServer, event, ptrSmtpClient, connection, client)
					} else {

					}
				} else {
					mxServer.hasMaxConnectionsOn()
					client.Quit()
					log.Debug("service#%d can't create client to %s, err - %v", c.id, mxServer.hostname, err)
				}
			} else {
				mxServer.hasMaxConnectionsOn()
				connection.Close()
				log.Warn("service#%d can't create client to %s, err - %v", c.id, mxServer.hostname, err)
			}
		} else {
			mxServer.hasMaxConnectionsOn()
			log.Warn("service#%d can't dial to %s, err - %v", c.id, hostname, err)
		}
	} else {
		log.Warn("connector#%d can't resolve tcp address %s, err - %v", c.id, tcpAddr.String(), err)
	}
}

func (c *Connector) createTlsSmtpClient(mxServer *MxServer, event *common.SendEvent, ptrSmtpClient **common.SmtpClient, connection net.Conn, client *smtp.Client) {
	// если есть какие данные о сертификате и к серверу можно создать TLS соединение
	if event.CertBytesLen > 0 && mxServer.useTLS {
		pool := x509.NewCertPool()
		// пытаем создать сертификат
		cert, err := x509.ParseCertificate(event.CertBytes)
		if err == nil {
			log.Debug("service#%d parse certificate for %s", c.id, event.Message.HostnameFrom)
			// задаем сертификату IP сервера
			cert.IPAddresses = mxServer.ips
			pool.AddCert(cert)
			// открываем TLS соединение
			err = client.StartTLS(&tls.Config{
				ClientCAs:  pool,
				ServerName: mxServer.realServerName,
			})
			// если все нормально, создаем клиента
			if err == nil {
				c.initSmtpClient(mxServer, event, ptrSmtpClient, connection, client)
			} else {
				// если не удалось создать TLS соединение
				// говорим, что не надо больше создавать TLS соединение
				mxServer.dontUseTLS()
				// разрываем созданое соединение
				// это необходимо, т.к. не все почтовые сервисы позволяют продолжить отправку письма
				// после неудачной попытке создать TLS соединение
				client.Quit()
				// создаем обычное соединие
				c.createSmtpClient(mxServer, event, ptrSmtpClient)
			}
		} else {
			log.Debug("service#%d can't parse certificate for %s, err - %v", c.id, event.Message.HostnameFrom, err)
			mxServer.dontUseTLS()
			c.initSmtpClient(mxServer, event, ptrSmtpClient, connection, client)
		}
	} else {
		c.initSmtpClient(mxServer, event, ptrSmtpClient, connection, client)
	}
}

func (c *Connector) initSmtpClient(mxServer *MxServer, event *common.SendEvent, ptrSmtpClient **common.SmtpClient, connection net.Conn, client *smtp.Client) {
	(*ptrSmtpClient) = new(common.SmtpClient)
	(*ptrSmtpClient).Id = len(mxServer.clients) + 1
	(*ptrSmtpClient).connection = connection
	(*ptrSmtpClient).Worker = client
	(*ptrSmtpClient).createDate = time.Now()
	(*ptrSmtpClient).Status = common.WorkingSmtpClientStatus
	mxServer.clients = append(mxServer.clients, (*ptrSmtpClient))
	log.Debug("service#%d create smtp client#%d for %s", c.id, (*ptrSmtpClient).Id, mxServer.hostname)
}
