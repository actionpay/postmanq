package connector

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/actionpay/postmanq/common"
	"github.com/actionpay/postmanq/logger"
	"net"
	"net/smtp"
	"time"
)

var (
	connectorEvents = make(chan *ConnectionEvent)
)

// соединитель, устанавливает соединение к почтовому сервису
type Connector struct {
	// Идентификатор для логов
	id int
}

// создает и запускает новый соединитель
func newConnector(id int) {
	connector := &Connector{id}
	connector.run()
}

// запускает прослушивание событий создания соединений
func (c *Connector) run() {
	for event := range connectorEvents {
		c.connect(event)
	}
}

// устанавливает соединение к почтовому сервису
func (c *Connector) connect(event *ConnectionEvent) {
	logger.By(event.Message.HostnameFrom).Debug("connector#%d-%d try find connection", c.id, event.Message.Id)
	goto receiveConnect

receiveConnect:
	event.TryCount++
	var targetClient *common.SmtpClient

	// смотрим все mx сервера почтового сервиса
	for _, mxServer := range event.server.mxServers {
		logger.By(event.Message.HostnameFrom).Debug("connector#%d-%d try receive connection for %s", c.id, event.Message.Id, mxServer.hostname)

		// пробуем получить клиента
		event.Queue, _ = mxServer.queues[event.address]
		client := event.Queue.Pop()
		if client != nil {
			targetClient = client.(*common.SmtpClient)
			logger.By(event.Message.HostnameFrom).Debug("connector%d-%d found free smtp client#%d", c.id, event.Message.Id, targetClient.Id)
		}

		// создаем новое соединение к почтовому сервису
		// если не удалось найти клиента
		// или клиент разорвал соединение
		if (targetClient == nil && !event.Queue.HasLimit()) ||
			(targetClient != nil && targetClient.Status == common.DisconnectedSmtpClientStatus) {
			logger.By(event.Message.HostnameFrom).Debug("connector#%d-%d can't find free smtp client for %s", c.id, event.Message.Id, mxServer.hostname)
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
		logger.By(event.Message.HostnameFrom).Debug("connector#%d-%d can't find free connections, wait...", c.id, event.Message.Id)
		time.Sleep(common.App.Timeout().Sleep)
		goto receiveConnect
	}
	return
}

// создает соединение к почтовому сервису
func (c *Connector) createSmtpClient(mxServer *MxServer, event *ConnectionEvent, ptrSmtpClient **common.SmtpClient) {
	// устанавливаем ip, с которого бцдем отсылать письмо
	tcpAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(event.address, "0"))
	if err == nil {
		logger.By(event.Message.HostnameFrom).Debug("connector#%d-%d resolve tcp address %s", c.id, event.Message.Id, tcpAddr.String())
		dialer := &net.Dialer{
			Timeout:   common.App.Timeout().Connection,
			LocalAddr: tcpAddr,
		}
		hostname := net.JoinHostPort(mxServer.hostname, "25")
		// создаем соединение к почтовому сервису
		connection, err := dialer.Dial("tcp", hostname)
		if err == nil {
			logger.By(event.Message.HostnameFrom).Debug("connector#%d-%d connect to %s", c.id, event.Message.Id, hostname)
			connection.SetDeadline(time.Now().Add(common.App.Timeout().Hello))
			client, err := smtp.NewClient(connection, mxServer.hostname)
			if err == nil {
				logger.By(event.Message.HostnameFrom).Debug("connector#%d-%d create client to %s", c.id, event.Message.Id, mxServer.hostname)
				err = client.Hello(event.Message.HostnameFrom)
				if err == nil {
					logger.By(event.Message.HostnameFrom).Debug("connector#%d-%d send command HELLO: %s", c.id, event.Message.Id, event.Message.HostnameFrom)
					// проверяем доступно ли TLS
					if mxServer.useTLS {
						mxServer.useTLS, _ = client.Extension("STARTTLS")
					}
					logger.By(event.Message.HostnameFrom).Debug("connector#%d-%d use TLS %v", c.id, event.Message.Id, mxServer.useTLS)
					// создаем TLS или обычное соединение
					if mxServer.useTLS {
						c.initTlsSmtpClient(mxServer, event, ptrSmtpClient, connection, client)
					} else {
						c.initSmtpClient(mxServer, event, ptrSmtpClient, connection, client)
					}
				} else {
					client.Quit()
					logger.By(event.Message.HostnameFrom).Debug("connector#%d-%d can't create client to %s, err - %v", c.id, event.Message.Id, mxServer.hostname, err)
				}
			} else {
				// если не удалось создать клиента,
				// возможно, на почтовом сервисе стоит ограничение на количество активных клиентов
				// ставим лимит очереди, чтобы не пытаться открывать новые соединения и не создавать новые клиенты
				event.Queue.HasLimitOn()
				connection.Close()
				logger.By(event.Message.HostnameFrom).Warn("connector#%d-%d can't create client to %s, err - %v", c.id, event.Message.Id, mxServer.hostname, err)
			}
		} else {
			// если не удалось установить соединение,
			// возможно, на почтовом сервисе стоит ограничение на количество соединений
			// ставим лимит очереди, чтобы не пытаться открывать новые соединения
			event.Queue.HasLimitOn()
			logger.By(event.Message.HostnameFrom).Warn("connector#%d-%d can't dial to %s, err - %v", c.id, event.Message.Id, hostname, err)
		}
	} else {
		logger.By(event.Message.HostnameFrom).Warn("connector#%d-%d can't resolve tcp address %s, err - %v", c.id, event.Message.Id, tcpAddr.String(), err)
	}
}

// открывает защищенное соединение
func (c *Connector) initTlsSmtpClient(mxServer *MxServer, event *ConnectionEvent, ptrSmtpClient **common.SmtpClient, connection net.Conn, client *smtp.Client) {
	// если есть какие данные о сертификате и к серверу можно создать TLS соединение
	pool := service.getPool(event.Message.HostnameFrom)
	if pool != nil && mxServer.useTLS {
		// открываем TLS соединение
		err := client.StartTLS(&tls.Config{
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
		c.initSmtpClient(mxServer, event, ptrSmtpClient, connection, client)
	}
}

// создает или инициализирует клиента
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
		logger.By(event.Message.HostnameFrom).Debug("connector#%d-%d create smtp client#%d for %s", c.id, event.Message.Id, smtpClient.Id, mxServer.hostname)
	} else {
		logger.By(event.Message.HostnameFrom).Debug("connector#%d-%d reopen smtp client#%d for %s", c.id, event.Message.Id, smtpClient.Id, mxServer.hostname)
	}
}
