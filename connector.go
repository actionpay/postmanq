package postmanq

import (
	"net"
	"net/smtp"
	"sync"
	"time"
)

const (
	UNLIMITED_CONNECTION_COUNT = -1
	CONNECTION_TIMEOUT = 30 * time.Second
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
		connector.tick = time.Tick(5 * time.Second)
		go connector.checkConnections()
	}
	return connector
}

func (this *Connector) checkConnections() {
	for now := range this.tick {
		go this.closeConnections(now)
	}
}

func (this *Connector) closeConnections(now time.Time) {
	Debug("check opened smtp connections...")
	for _, mailServer := range this.mailServers {
		for _, mxServer := range mailServer.mxServers {
			for i, smtpClient := range mxServer.clients {
				if smtpClient.Status == SMTP_CLIENT_STATUS_WAITING && now.Sub(smtpClient.createDate) >= CONNECTION_TIMEOUT {
					smtpClient.Status = SMTP_CLIENT_STATUS_DISCONNECTED
					err := smtpClient.client.Close()
					if err != nil {
						WarnWithErr(err)
					}
					mxServer.clients = mxServer.clients[:i]
					if i < len(mxServer.clients) - 1 {
						mxServer.clients = append(mxServer.clients, mxServer.clients[i+1:]...)
					}
					if mxServer.maxConnections != UNLIMITED_CONNECTION_COUNT {
						mxServer.maxConnections = UNLIMITED_CONNECTION_COUNT
					}
					Debug("close connection smtp client#%d mx server %s", smtpClient.Id, mxServer.hostname)
				}
			}
		}
	}
}

func (this *Connector) OnSend(event *SendEvent) {
	this.mutex.Lock()
	hostname := event.Message.HostnameTo
	if _, ok := this.mailServers[hostname]; !ok {
		this.lookupMxServers(hostname)
	}
	if mailServer, ok := this.mailServers[hostname]; ok {
		mailServer.findSmtpClient(event)
	}
	this.mutex.Unlock()
}

func (this *Connector) lookupMxServers(hostname string) {
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
		mailServer.lastIndex = len(mailServer.mxServers) - 1
		this.mailServers[hostname] = mailServer
	} else {
		WarnWithErr(err)
	}
}

type MailServer struct {
	mxServers []*MxServer
	lastIndex int
}

func (this *MailServer) findSmtpClient(event *SendEvent) {
	var targetSmtpClient *SmtpClient
	mxServersIndex := 0
	for targetSmtpClient == nil {
		mxServer := this.mxServers[mxServersIndex]
		targetSmtpClient = mxServer.findFreeSmtpServers()
		if targetSmtpClient == nil && mxServer.maxConnections == UNLIMITED_CONNECTION_COUNT {
			targetSmtpClient = mxServer.createNewSmtpClient(event.Message.HostnameFrom)
		}
		if mxServersIndex == this.lastIndex {
			mxServersIndex = 0
		} else {
			mxServersIndex++
		}
	}
	event.Client = targetSmtpClient
}

type MxServer struct {
	hostname       string
	maxConnections int
	clients        []*SmtpClient
}

func (this *MxServer) findFreeSmtpServers() *SmtpClient {
	Debug("search free smtp clients for %s...", this.hostname)
	for _, smtpClient := range this.clients {
		if smtpClient.Status == SMTP_CLIENT_STATUS_WAITING {
			smtpClient.createDate = time.Now()
			smtpClient.Status = SMTP_CLIENT_STATUS_WORKING
			Debug("found smtp client#%d", smtpClient.Id)
			return smtpClient
		}
	}
	Debug("free smtp clients not found for %s", this.hostname)
	return nil
}

func (this *MxServer) createNewSmtpClient(hostname string) *SmtpClient {
	Debug("create smtp client for %s...", this.hostname)
	var smtpClient *SmtpClient
	connection, err := net.Dial("tcp", net.JoinHostPort(this.hostname, "25"))
	if err == nil {
		client, err := smtp.NewClient(connection, hostname)
		if err == nil {
			err = client.Hello(hostname)
			if err == nil {
				smtpClient = new(SmtpClient)
				smtpClient.Id = len(this.clients) + 1
				smtpClient.connection = connection
				smtpClient.client = client
				smtpClient.createDate = time.Now()
				smtpClient.Status = SMTP_CLIENT_STATUS_WORKING
				this.clients = append(this.clients, smtpClient)
				Debug("smtp client#%d created for %s", smtpClient.Id, this.hostname)
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
	return smtpClient
}

func (this *MxServer) updateMaxConnections(err error) {
	this.maxConnections = len(this.clients)
	Debug("max %d smtp clients for %s, wait...", this.maxConnections, this.hostname)
	WarnWithErr(err)
}

type SmtpClientStatus int

const (
	SMTP_CLIENT_STATUS_WORKING      SmtpClientStatus = iota
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
