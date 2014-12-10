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
	"strings"
	"regexp"
)

const (
	UNLIMITED_CONNECTION_COUNT = -1
	CONNECTION_TIMEOUT         = 30 * time.Second
	RECEIVE_CONNECTION_TIMEOUT = 5 * time.Second
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
	certBytesLen       int
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
			pemBytes, err := ioutil.ReadFile(this.CertFilename)
			if err == nil {
				pemBlock, _ := pem.Decode(pemBytes)
				this.certBytes = pemBlock.Bytes
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
	event.CertBytesLen = this.certBytesLen
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
	Debug("lookup mx domains for %s...", hostname)
	mxs, err := net.LookupMX(hostname)
	if err == nil {
		mailServer := new(MailServer)
		mailServer.mxServers = make([]*MxServer, len(mxs))
		for i, mx := range mxs {
			mxHostname := mx.Host
			mxHostnameLen := len(mxHostname)
			if mxHostname[mxHostnameLen - 1:mxHostnameLen] == "." {
				mxHostname = mxHostname[:mxHostnameLen - 1]
			}
			Debug("lookup mx domain %s for %s", mxHostname, hostname)
			mxHostnameParts := strings.Split(mxHostname, ".")
			mxARecord := strings.Join(mxHostnameParts[len(mxHostnameParts) - 2:], ".")
			mxServer := new(MxServer)
			mxServer.hostname = mxHostname
			mxServer.maxConnections = UNLIMITED_CONNECTION_COUNT
			mxServer.ips = make([]net.IP, 0)
			mxServer.clients = make([]*SmtpClient, 0)
			mxServer.useTLS = true
			ips, err := net.LookupIP(mxHostname)
			if err == nil {
				for _, ip := range ips {
					ip = ip.To4()
					if ip != nil {
						Debug("lookup mx ip %s for %s", ip.String(), hostname)
						existsIpsLen := len(mxServer.ips)
						index := sort.Search(existsIpsLen, func(i int) bool {
							return mxServer.ips[i].Equal(ip)
						})
						if existsIpsLen == 0 || (index == -1 && existsIpsLen > 0) {
							mxServer.ips = append(mxServer.ips, ip)
						}
					}
				}
				for _, ip := range mxServer.ips {
					addrs, err := net.LookupAddr(ip.String())
					if err == nil {
						for _, addr := range addrs {
							addrLen := len(addr)
							if addr[addrLen - 1:addrLen] == "." {
								addr = addr[:addrLen - 1]
							}
							if net.ParseIP(addr) == nil {
								Debug("lookup addr %s for ip %s", addr, ip.String())
								if len(mxServer.realServerName) == 0 {
									hostnameMatched, _ := regexp.MatchString(hostname, mxServer.hostname)
									addrMatched, _ := regexp.MatchString(mxARecord, addr)
									if hostnameMatched && !addrMatched {
										mxServer.realServerName = addr
									} else if !hostnameMatched && addrMatched || !hostnameMatched && !addrMatched {
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
			if len(mxServer.realServerName) == 0 {
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

type MailServer struct {
	mxServers []*MxServer
	lastIndex int
}

func (this *MailServer) findSmtpClient(event *SendEvent) {
	var targetSmtpClient *SmtpClient
	mxServersIndex := 0
	for targetSmtpClient == nil && time.Now().Sub(event.CreateDate) <= RECEIVE_CONNECTION_TIMEOUT {
		mxServer := this.mxServers[mxServersIndex]
		mxServer.findFreeSmtpServers(&targetSmtpClient)
		if targetSmtpClient == nil && mxServer.maxConnections == UNLIMITED_CONNECTION_COUNT {
			Debug("create smtp client for %s", mxServer.hostname)
			mxServer.createNewSmtpClient(event, &targetSmtpClient, mxServer.createTLSSmtpClient)
			if targetSmtpClient != nil {
				break
			}
		}
		if mxServersIndex == this.lastIndex {
			mxServersIndex = 0
		} else {
			mxServersIndex++
		}
	}
	if targetSmtpClient == nil {
		event.DefaultPrevented = true
		ReturnMail(event.Message, errors.New(fmt.Sprintf("can't find free connection for mail#%d", event.Message.Id)))
	} else {
		event.Client = targetSmtpClient
	}
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
	realServerName string
	useTLS         bool
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

func (this *MxServer) createNewSmtpClient(event *SendEvent, ptrSmtpClient **SmtpClient, callback func(event *SendEvent, ptrSmtpClient **SmtpClient, connection net.Conn, client *smtp.Client)) {
	connection, err := net.Dial("tcp", net.JoinHostPort(this.hostname, "25"))
	if err == nil {
		client, err := smtp.NewClient(connection, event.Message.HostnameFrom)
		if err == nil {
			err = client.Hello(event.Message.HostnameFrom)
			if err == nil {
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

func (this *MxServer) createTLSSmtpClient(event *SendEvent, ptrSmtpClient **SmtpClient, connection net.Conn, client *smtp.Client) {
	if event.CertBytesLen > 0 && this.useTLS {
		pool := x509.NewCertPool()
		cert, err := x509.ParseCertificate(event.CertBytes)
		if err == nil {
			cert.IPAddresses = this.ips
			pool.AddCert(cert)
			err = client.StartTLS(&tls.Config {
				ClientCAs : pool,
				ServerName: this.realServerName,
			})
			if err == nil {
				this.createSmtpClient(ptrSmtpClient, connection, client)
			} else {
				this.dontUseTLS(err)
				client.Quit()
				this.createNewSmtpClient(event, ptrSmtpClient, this.createPlainSmtpClient)
			}
		} else {
			this.dontUseTLS(err)
		}
	}
}

func (this *MxServer) createPlainSmtpClient(event *SendEvent, ptrSmtpClient **SmtpClient, connection net.Conn, client *smtp.Client) {
	this.createSmtpClient(ptrSmtpClient, connection, client)
}

func (this *MxServer) createSmtpClient(ptrSmtpClient **SmtpClient, connection net.Conn, client *smtp.Client) {
	(*ptrSmtpClient) = new(SmtpClient)
	(*ptrSmtpClient).Id = len(this.clients) + 1
	(*ptrSmtpClient).connection = connection
	(*ptrSmtpClient).Worker = client
	(*ptrSmtpClient).createDate = time.Now()
	(*ptrSmtpClient).Status = SMTP_CLIENT_STATUS_WORKING
	this.clients = append(this.clients, (*ptrSmtpClient))
	Debug("smtp client#%d created for %s", (*ptrSmtpClient).Id, this.hostname)
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

func (this *MxServer) dontUseTLS(err error) {
	this.useTLS = false
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
	Worker     *smtp.Client
	createDate time.Time
	Status     SmtpClientStatus
}
