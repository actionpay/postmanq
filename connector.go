package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"net"
	"net/smtp"
	"sync"
	"time"
	"crypto/x509"
	"io/ioutil"
//	"crypto/tls"
	"encoding/pem"
)

const (
	UNLIMITED_CONNECTION_COUNT = -1
	CONNECTION_TIMEOUT = 30 * time.Second
)

var (
	connector *Connector
)

type Connector struct {
	PrivateKeyFilename string                 `yaml:"privateKey"`
	CertFilename       string                 `yaml:"certificate"`
	mailServers        map[string]*MailServer
	mutex              *sync.Mutex
	ticker             *time.Ticker
	certPool           *x509.CertPool
	certBytes          []byte
}

func NewConnector() *Connector {
	if connector == nil {
		connector = new(Connector)
		connector.mailServers = make(map[string]*MailServer)
		connector.mutex = new(sync.Mutex)
		connector.ticker = time.NewTicker(11 * time.Second)
		connector.certBytes = []byte{}
		go connector.checkConnections()
	}
	return connector
}

func (this *Connector) checkConnections() {
	for now := range this.ticker.C {
		go this.closeConnections(now)
	}
}

func (this *Connector) closeConnections(now time.Time) {
	Debug("check opened smtp connections...")
	for _, mailServer := range this.mailServers {
		go mailServer.closeConnections(now)
	}
}

func (this *Connector) OnRegister() {}

func (this *Connector) OnInit(event *InitEvent) {
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		if len(this.CertFilename) > 0 {
//			certs := x509.NewCertPool()
			pemBytes, err := ioutil.ReadFile(this.CertFilename)
			if err == nil {
				pemBlock, _ := pem.Decode(pemBytes)
				this.certBytes = pemBlock.Bytes
//				if err == nil {
//					certBytes, err := x509.ParseCertificate(pemBlock.Bytes)
//					if err == nil {
//						this.certBytes = certBytes
//					} else {
//						WarnWithErr(err)
//					}
//				} else {
//					WarnWithErr(err)
//				}

//				if certs.AppendCertsFromPEM(pemBytes) {
//					Debug("certificate is loaded")
//					this.certPool = certs
//				} else {
//					Debug("certificate is not loaded")
//				}
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

func (this *Connector) OnFinish(event *FinishEvent) {
	this.ticker.Stop()
	this.closeConnections(time.Now().Add(time.Minute))
	event.Group.Done()
}

func (this *Connector) OnSend(event *SendEvent) {
	this.mutex.Lock()
	if this.certPool != nil {
		event.CertPool = this.certPool
	}
	event.CertBytes = this.certBytes
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
			mxServer.ips = make([]net.IP, 0)
			mxServer.clients = make([]*SmtpClient, 0)
			ips, err := net.LookupIP(mx.Host)
			if err == nil {
				for _, ip := range ips {
					if len(mxServer.ips) == 0 {
						mxServer.ips = append(mxServer.ips, ip)
					} else {
						for _ ,existsIp := range mxServer.ips {
							if !ip.Equal(existsIp) {
								mxServer.ips = append(mxServer.ips, ip)
							}
						}
					}
				}
			} else {
				WarnWithErr(err)
			}
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
		mxServer.findFreeSmtpServers(&targetSmtpClient)
		if targetSmtpClient == nil && mxServer.maxConnections == UNLIMITED_CONNECTION_COUNT {
			targetSmtpClient = mxServer.createNewSmtpClient(event)
		}
		if mxServersIndex == this.lastIndex {
			mxServersIndex = 0
		} else {
			mxServersIndex++
		}
	}
	event.Client = targetSmtpClient
}

func (this *MailServer) closeConnections(now time.Time) {
	for _, mxServer := range this.mxServers {
		go mxServer.closeConnections(now)
	}
}

type MxServer struct {
	hostname       string
	maxConnections int
	ips            []net.IP
	clients        []*SmtpClient
}

func (this *MxServer) findFreeSmtpServers(targetSmtpClient **SmtpClient) {
	Debug("search free smtp clients for %s...", this.hostname)
	for _, smtpClient := range this.clients {
		if smtpClient.Status == SMTP_CLIENT_STATUS_WAITING {
			smtpClient.createDate = time.Now()
			smtpClient.Status = SMTP_CLIENT_STATUS_WORKING
			(*targetSmtpClient) = smtpClient
			Debug("found smtp client#%d", smtpClient.Id)
			break
		}
	}
	Debug("free smtp clients not found for %s", this.hostname)
}

func (this *MxServer) createNewSmtpClient(event *SendEvent) *SmtpClient {
	Debug("create smtp client for %s...", this.hostname)
	var smtpClient *SmtpClient
	connection, err := net.Dial("tcp", net.JoinHostPort(this.hostname, "25"))
	if err == nil {
		client, err := smtp.NewClient(connection, event.Message.HostnameFrom)
		if err == nil {
			err = client.Hello(event.Message.HostnameFrom)
//			if len(event.CertBytes) > 0 {
//				pool := x509.NewCertPool()
//				cert, err := x509.ParseCertificate(event.CertBytes)
//				if err == nil {
//					cert.IPAddresses = this.ips
//					cert.DNSNames = []string{this.hostname}
//					pool.AddCert(cert)
//					err = client.StartTLS(&tls.Config {
//						RootCAs           : pool,
//						ServerName        : this.hostname,
//					})
//					Debug("server name is %s", this.hostname)
//					if err == nil {
//						Debug("create tls connection...")
//					} else {
//						WarnWithErr(err)
//					}
//				} else {
//					WarnWithErr(err)
//				}
//			}
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

func (this *MxServer) closeConnections(now time.Time) {
	for i, smtpClient := range this.clients {
		if smtpClient.Status == SMTP_CLIENT_STATUS_WAITING && now.Sub(smtpClient.createDate) >= CONNECTION_TIMEOUT {
			smtpClient.Status = SMTP_CLIENT_STATUS_DISCONNECTED
			err := smtpClient.client.Close()
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
