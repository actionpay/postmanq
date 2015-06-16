package connector

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/log"
	"net"
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

type MxQueue struct {
	common.Queue
	hasMax bool
}

func (m *MxQueue) hasMaxOn() {
	m.hasMax = true
}

// почтовый сервер
type MxServer struct {
	// доменное имя почтового сервера
	hostname       string

	// IP сервера
	ips            []net.IP

	// клиенты сервера
	clients        []*common.SmtpClient

	// А запись сервера
	realServerName string

	// использоватение TLS
	useTLS         bool

	queues map[string]*MxQueue
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
				log.Debug("close connection smtp client#%d mx server %s", client.Id, this.hostname)
			}
		}
	}
}

// запрещает использовать TLS соединения
func (this *MxServer) dontUseTLS() {
	this.useTLS = false
}
