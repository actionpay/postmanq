package postmanq

import (
	"net"
	"net/smtp"
	"sync"
	"time"
)

const (
	UNLIMITED_CONNECTION_COUNT = -1
	CONNECTION_TIMEOUT = time.Second * 30
)

var (
	connector *Connector
)

type Connector struct {
	mailServers map[string]*MailServer
	mutex       *sync.Mutex
	tick        <-chan time.Time
}

func NewConnector() *Connector {
	if connector == nil {
		connector = new(Connector)
		connector.mailServers = make(map[string]*MailServer)
		connector.mutex = new(sync.Mutex)
//		connector.tick = time.Tick(time.Second)
//		go connector.checkConnections()
	}
	return connector
}

//func (this *Connector) checkConnections() {
//	for now := range this.tick {
//		for _, mailServer := range this.mailServers {
//			for _, mxServer := range mailServer.mxServers {
//				for _, smtpClient := range mxServer.clients {
//					if smtpClient.Status == SMTP_CLIENT_STATUS_WAITING && time.Now().Sub(smtpClient.createDate) >= CONNECTION_TIMEOUT {
//						smtpClient.Status = SMTP_CLIENT_STATUS_DISCONNECTED
//						err := smtpClient.client.Close()
//						if err != nil {
//							WarnWithErr(err)
//						}
//					}
//				}
//			}
//		}
//	}
//}

func (this *Connector) OnSend(event *SendEvent) {
	this.mutex.Lock()
	hostname := event.Message.HostnameTo
	if _, ok := this.mailServers[hostname]; !ok {
		Debug("look up mx domains for %s...", hostname)
		mxs, err := net.LookupMX(hostname)
		if err == nil {
			mailServer := new(MailServer)
			mailServer.mxServers = make([]*MxServer, len(mxs))
			for i, mx := range mxs {
				Debug("receive mx domain %s for %s", mx.Host, hostname)
				mxServer := new(MxServer)
				mxServer.hostname = mx.Host
				mxServer.maxConnections = UNLIMITED_CONNECTION_COUNT
				mxServer.clients = make([]*SmtpClient, 0)
				mailServer.mxServers[i] = mxServer
			}
			this.mailServers[hostname] = mailServer
		} else {
			WarnWithErr(err)
		}
	}
	if mailServer, ok := this.mailServers[hostname]; ok {
		var targetSmtpClient *SmtpClient
		mxServersIndex := 0
		mxServersLastIndex := len(mailServer.mxServers) - 1
		for targetSmtpClient == nil {
			mxServer := mailServer.mxServers[mxServersIndex]
			Debug("search free smtp clients for %s...", mxServer.hostname)
			for _, smtpClient := range mxServer.clients {
				if smtpClient.Status == SMTP_CLIENT_STATUS_WAITING {
					Debug("found smtp client#%d", smtpClient.Id)
					smtpClient.Status = SMTP_CLIENT_STATUS_WORKING
					targetSmtpClient = smtpClient
					break
				}
			}
			Debug("free smtp clients not found for %s", mxServer.hostname)
			if targetSmtpClient == nil && mxServer.maxConnections == UNLIMITED_CONNECTION_COUNT {
				Debug("create smtp client for %s...", mxServer.hostname)
				connection, err := net.Dial("tcp", net.JoinHostPort(mxServer.hostname, "25"))
				if err == nil {
					client, err := smtp.NewClient(connection, event.Message.HostnameFrom)
					if err == nil {
						targetSmtpClient = new(SmtpClient)
						targetSmtpClient.Id = len(mxServer.clients) + 1
						targetSmtpClient.connection = connection
						targetSmtpClient.client = client
						targetSmtpClient.createDate = time.Now()
						targetSmtpClient.Status = SMTP_CLIENT_STATUS_WORKING
						mxServer.clients = append(mxServer.clients, targetSmtpClient)
						Debug("smtp client#%d created for %s", targetSmtpClient.Id, mxServer.hostname)
						break
					} else {
						connection.Close()
						mxServer.updateMaxConnections(err)
					}
				} else {
					mxServer.updateMaxConnections(err)
				}
			}
			if mxServersIndex == mxServersLastIndex {
				mxServersIndex = 0
			} else {
				mxServersIndex++
			}
		}
		if targetSmtpClient != nil {
			event.Client = targetSmtpClient
		}
	}
	this.mutex.Unlock()
}

type MailServer struct {
	mxServers []*MxServer
}

type MxServer struct {
	hostname       string
	maxConnections int
	clients        []*SmtpClient
}

func (this *MxServer) updateMaxConnections(err error) {
	this.maxConnections = len(this.clients)
	Debug("max %d smtp clients for %s, wait...", this.maxConnections, this.hostname)
	WarnWithErr(err)
}

type SmtpClientStatus int

const (
	SMTP_CLIENT_STATUS_WORKING SmtpClientStatus = iota
	SMTP_CLIENT_STATUS_WAITING
	SMTP_CLIENT_STATUS_DISCONNECTED
)

type SmtpClient struct {
	Id         int
	connection net.Conn
	client     *smtp.Client
	createDate time.Time
	Status     SmtpClientStatus
}
