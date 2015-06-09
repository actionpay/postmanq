package connector

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/log"
	"fmt"
	"time"
	"errors"
)

type ConnectionEvent struct {
	sendEvent *common.SendEvent
	servers   chan *MailServer
	server *MailServer
}

type Connector struct {
	id int
}

func newConnector(id int) *Connector {
	return &Connector{id}
}

func (c *Connector) run() {
	for _, event := range events {
		c.connect(event)
	}
}

func (c *Connector) connect(event *common.SendEvent) {
	log.Info("connector#%d try create connection for mail#%d", c.id, event.Message.Id)
	// передаем событию сертификат и его длину
	event.CertBytes = c.certBytes
	event.CertBytesLen = c.certBytesLen

	connectionEvent := &ConnectionEvent{
		sendEvent: event,
		servers: make(chan *MailServer, 1),
	}
	goto connectToMailServer

connectToMailServer:
	seekerEvents <- connectionEvent
	server := <-connectionEvent.servers
	switch server.status {
	case LookupMailServerStatus:
		goto waitLookup
		return
	case SuccessMailServerStatus:
		connectionEvent.server = server
		connectionEvents <- connectionEvent
		return
	case ErrorMailServerStatus:
		common.ReturnMail(
			event,
			errors.New(fmt.Sprintf("511 connector#%d can't lookup %s", c.id, event.Message.HostnameTo)),
		)
		return
	}

waitLookup:
	log.Debug("connector#%d wait ending look up mail server %s...", c.id, event.Message.HostnameTo)
	time.Sleep(common.SleepTimeout)
	goto connectToMailServer
}
