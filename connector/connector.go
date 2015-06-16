package connector

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
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
	return &Connector{id}.run()
}

func (c *Connector) run() {
	for _, event := range connectorEvents {
		c.connect(event)
	}
}

func (c *Connector) connect(event *ConnectionEvent) {
	logger.Debug("connector#%d find connection for mail#%d", c.id, event.Message.Id)
	goto receiveConnect

receiveConnect:
	event.TryCount++
	var targetClient *common.SmtpClient

	for _, mxServer := range event.server.mxServers {
		logger.Debug("connector#%d try receive connection for %s", c.id, mxServer.hostname)

		var queue *common.LimitedQueue
		var ok bool
		if queue, ok = mxServer.queues[event.address]; ok {
			if !queue.Empty() {
				client := queue.Pop()
				if client != nil {
					targetClient = client.(*common.SmtpClient)
					logger.Debug("connector%d found free smtp client#%d", c.id, targetClient.Id)
				}
			}
		} else {
			queue = new(common.LimitedQueue)
			mxServer.queues[event.address] = queue
		}

		if (targetClient == nil && !queue.HasLimit) ||
			(targetClient != nil && targetClient.Status == common.DisconnectedSmtpClientStatus) {
			logger.Debug("connector#%d can't find free smtp client for %s", c.id, mxServer.hostname)
			event.Queue = queue
			c.createSmtpClient(mxServer, event, &targetClient)
		}

		if targetClient != nil {
			break
		}
	}

	if targetClient == nil {
		goto waitConnect
		return
	} else {
		targetClient.Wakeup()
		targetClient.SetTimeout(common.MailTimeout)
		event.Client = targetClient
		event.Iterator.Next().(common.SendingService).Events() <- event.SendEvent
		return
	}

waitConnect:
	if event.TryCount >= common.TryConnectCount {
		common.ReturnMail(
			event.SendEvent,
			errors.New(fmt.Sprintf("connector#%d can't connect to %s", c.id, event.Message.HostnameTo)),
		)
		return
	} else {
		logger.Debug("connector#%d can't find free connections, wait...", c.id)
		time.Sleep(SleepTimeout)
		goto receiveConnect
		return
	}
}

func (c *Connector) createSmtpClient(mxServer *MxServer, event *ConnectionEvent, ptrSmtpClient **common.SmtpClient) {
	// создаем соединение
	tcpAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(event.address, "0"))
	if err == nil {
		logger.Debug("connector#%d resolve tcp address %s", c.id, tcpAddr.String())
		queue := event.Queue
		dialer := &net.Dialer{
			Timeout:   common.HelloTimeout,
			LocalAddr: tcpAddr,
		}
		hostname := net.JoinHostPort(mxServer.hostname, "25")
		connection, err := dialer.Dial("tcp", hostname)
		if err == nil {
			logger.Debug("connector#%d dial to %s", c.id, hostname)
			connection.SetDeadline(time.Now().Add(common.HelloTimeout))
			client, err := smtp.NewClient(connection, mxServer.hostname)
			if err == nil {
				logger.Debug("connector#%d create client to %s", c.id, mxServer.hostname)
				err = client.Hello(event.Message.HostnameFrom)
				if err == nil {
					logger.Debug("connector#%d send command HELLO: %s", c.id, event.Message.HostnameFrom)
					// создаем TLS или обычное соединение
					if mxServer.useTLS {
						mxServer.useTLS, _ = client.Extension("STARTTLS")
					}
					logger.Debug("connector#%d use TLS %v", c.id, mxServer.useTLS)
					if mxServer.useTLS {
						c.createTlsSmtpClient(mxServer, event, ptrSmtpClient, connection, client)
					} else {
						c.initSmtpClient(mxServer, ptrSmtpClient, connection, client)
					}
				} else {
					queue.HasLimitOn()
					client.Quit()
					logger.Debug("connector#%d can't create client to %s, err - %v", c.id, mxServer.hostname, err)
				}
			} else {
				queue.HasLimitOn()
				connection.Close()
				logger.Warn("connector#%d can't create client to %s, err - %v", c.id, mxServer.hostname, err)
			}
		} else {
			queue.HasLimitOn()
			logger.Warn("connector#%d can't dial to %s, err - %v", c.id, hostname, err)
		}
	} else {
		logger.Warn("connector#%d can't resolve tcp address %s, err - %v", c.id, tcpAddr.String(), err)
	}
}

func (c *Connector) createTlsSmtpClient(mxServer *MxServer, event *ConnectionEvent, ptrSmtpClient **common.SmtpClient, connection net.Conn, client *smtp.Client) {
	// если есть какие данные о сертификате и к серверу можно создать TLS соединение
	if event.CertBytesLen > 0 && mxServer.useTLS {
		pool := x509.NewCertPool()
		// пытаем создать сертификат
		cert, err := x509.ParseCertificate(event.CertBytes)
		if err == nil {
			logger.Debug("connector#%d parse certificate for %s", c.id, event.Message.HostnameFrom)
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
				c.initSmtpClient(mxServer, ptrSmtpClient, connection, client)
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
			logger.Debug("connector#%d can't parse certificate for %s, err - %v", c.id, event.Message.HostnameFrom, err)
			mxServer.dontUseTLS()
			c.initSmtpClient(mxServer, ptrSmtpClient, connection, client)
		}
	} else {
		c.initSmtpClient(mxServer, ptrSmtpClient, connection, client)
	}
}

func (c *Connector) initSmtpClient(mxServer *MxServer, ptrSmtpClient **common.SmtpClient, connection net.Conn, client *smtp.Client) {
	isNil := *ptrSmtpClient == nil
	if isNil {
		var count int
		for _, queue := range mxServer.queues {
			count += queue.Len()
		}
		*ptrSmtpClient = &common.SmtpClient{
			Id: count + 1,
		}
	}
	smtpClient := *ptrSmtpClient
	smtpClient.Conn = connection
	smtpClient.Worker = client
	smtpClient.ModifyDate = time.Now()
	if isNil {
		logger.Debug("connector#%d create smtp client#%d for %s", c.id, smtpClient.Id, mxServer.hostname)
	} else {
		logger.Debug("connector#%d reopen smtp client#%d for %s", c.id, smtpClient.Id, mxServer.hostname)
	}
}
