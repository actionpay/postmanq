package connector

import (
	"crypto/tls"
//	"crypto/x509"
	"errors"
	"fmt"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
	"net"
	"net/smtp"
	"time"
)

var (
	connectorEvents = make(chan *ConnectionEvent)
)

// Соединитель, устанавливает соединение к почтовому сервису
type Connector struct {
	// Идентификатор для логов
	id int
}

// Создает и запускает новый соединитель
func newConnector(id int) {
	connector := &Connector{id}
	connector.run()
}

// Запускает прослушивание событий создания соединений
func (c *Connector) run() {
	for event := range connectorEvents {
		c.connect(event)
	}
}

// Устанавливает соединение к почтовому сервису
func (c *Connector) connect(event *ConnectionEvent) {
	logger.Debug("connector#%d find connection for mail#%d", c.id, event.Message.Id)
	goto receiveConnect

receiveConnect:
	event.TryCount++
	var targetClient *common.SmtpClient

	// смотрим все mx сервера почтового сервиса
	for _, mxServer := range event.server.mxServers {
		logger.Debug("connector#%d try receive connection for %s", c.id, mxServer.hostname)

		// пробуем получить клиента
		event.Queue, _ = mxServer.queues[event.address]
		client := event.Queue.Pop()
		if client != nil {
			targetClient = client.(*common.SmtpClient)
			logger.Debug("connector%d found free smtp client#%d", c.id, targetClient.Id)
		}

		// создаем новое соединение к почтовому сервису
		// если не удалось найти клиента
		// или клиент разорвал соединение
		if (targetClient == nil && !event.Queue.HasLimit()) ||
			(targetClient != nil && targetClient.Status == common.DisconnectedSmtpClientStatus) {
			logger.Debug("connector#%d can't find free smtp client for %s", c.id, mxServer.hostname)
			c.createSmtpClient(mxServer, event, &targetClient)
		}

		if targetClient != nil {
			break
		}
	}

	// если клиент не создан, значит мы создали максимум соединений к почтовому сервису
	if targetClient == nil {
		// приостановим работу горутины
		goto waitConnect
	} else {
		targetClient.Wakeup()
		event.Client = targetClient
		// передаем событие отправителю
		event.Iterator.Next().(common.SendingService).Events() <- event.SendEvent
	}
	return

waitConnect:
	if event.TryCount >= common.MaxTryConnectionCount {
		common.ReturnMail(
			event.SendEvent,
			errors.New(fmt.Sprintf("connector#%d can't connect to %s", c.id, event.Message.HostnameTo)),
		)
	} else {
		logger.Debug("connector#%d can't find free connections, wait...", c.id)
		time.Sleep(common.SleepTimeout)
		goto receiveConnect
	}
	return
}

// Создает соединение к почтовому сервису
func (c *Connector) createSmtpClient(mxServer *MxServer, event *ConnectionEvent, ptrSmtpClient **common.SmtpClient) {
	// устанавливаем ip, с которого бцдем отсылать письмо
	tcpAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(event.address, "0"))
	if err == nil {
		logger.Debug("connector#%d resolve tcp address %s", c.id, tcpAddr.String())
		dialer := &net.Dialer{
			Timeout:   common.HelloTimeout,
			LocalAddr: tcpAddr,
		}
		hostname := net.JoinHostPort(mxServer.hostname, "25")
		// создаем соединение к почтовому сервису
		connection, err := dialer.Dial("tcp", hostname)
		if err == nil {
			logger.Debug("connector#%d connect to %s", c.id, hostname)
			connection.SetDeadline(time.Now().Add(common.HelloTimeout))
			client, err := smtp.NewClient(connection, mxServer.hostname)
			if err == nil {
				logger.Debug("connector#%d create client to %s", c.id, mxServer.hostname)
				err = client.Hello(event.Message.HostnameFrom)
				if err == nil {
					logger.Debug("connector#%d send command HELLO: %s", c.id, event.Message.HostnameFrom)
					// проверяем доступно ли TLS
					if mxServer.useTLS {
						mxServer.useTLS, _ = client.Extension("STARTTLS")
					}
					logger.Debug("connector#%d use TLS %v", c.id, mxServer.useTLS)
					// создаем TLS или обычное соединение
					if mxServer.useTLS {
						c.initTlsSmtpClient(mxServer, event, ptrSmtpClient, connection, client)
					} else {
						c.initSmtpClient(mxServer, event, ptrSmtpClient, connection, client)
					}
				} else {
					client.Quit()
					logger.Debug("connector#%d can't create client to %s, err - %v", c.id, mxServer.hostname, err)
				}
			} else {
				// если не удалось создать клиента,
				// возможно, на почтовом сервисе стоит ограничение на количество активных клиентов
				// ставим лимит очереди, чтобы не пытаться открывать новые соединения и не создавать новые клиенты
				event.Queue.HasLimitOn()
				connection.Close()
				logger.Warn("connector#%d can't create client to %s, err - %v", c.id, mxServer.hostname, err)
			}
		} else {
			// если не удалось установить соединение,
			// возможно, на почтовом сервисе стоит ограничение на количество соединений
			// ставим лимит очереди, чтобы не пытаться открывать новые соединения
			event.Queue.HasLimitOn()
			logger.Warn("connector#%d can't dial to %s, err - %v", c.id, hostname, err)
		}
	} else {
		logger.Warn("connector#%d can't resolve tcp address %s, err - %v", c.id, tcpAddr.String(), err)
	}
}

// Открывает защищенное соединение
func (c *Connector) initTlsSmtpClient(mxServer *MxServer, event *ConnectionEvent, ptrSmtpClient **common.SmtpClient, connection net.Conn, client *smtp.Client) {
	// если есть какие данные о сертификате и к серверу можно создать TLS соединение
	if service.pool != nil && mxServer.useTLS {
//		pool := x509.NewCertPool()
		// пытаем создать сертификат
//		cert, err := x509.ParseCertificate(event.CertBytes)
//		if err == nil {
			logger.Debug("connector#%d parse certificate for %s", c.id, event.Message.HostnameFrom)
			// задаем сертификату IP сервера
//			cert.IPAddresses = mxServer.ips
//			pool.AddCert(service.cert)
			// открываем TLS соединение
			err := client.StartTLS(&tls.Config{
				ClientCAs:  service.pool,
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
//		} else {
//			logger.Debug("connector#%d can't parse certificate for %s, err - %v", c.id, event.Message.HostnameFrom, err)
//			mxServer.dontUseTLS()
//			c.initSmtpClient(mxServer, event, ptrSmtpClient, connection, client)
//		}
	} else {
		c.initSmtpClient(mxServer, event, ptrSmtpClient, connection, client)
	}
}

// Создает или инициализирует клиента
func (c *Connector) initSmtpClient(mxServer *MxServer, event *ConnectionEvent, ptrSmtpClient **common.SmtpClient, connection net.Conn, client *smtp.Client) {
	isNil := *ptrSmtpClient == nil
	if isNil {
		var count int
		for _, queue := range mxServer.queues {
			count += queue.MaxLen()
		}
		*ptrSmtpClient = &common.SmtpClient{
			Id: count + 1,
		}
		// увеличиваем максимальную длину очереди
		event.Queue.AddMaxLen()
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
