package connector

import (
	"errors"
	"fmt"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/log"
	"time"
)

type ConnectionEvent struct {
	sendEvent   *common.SendEvent
	servers     chan *MailServer
	server      *MailServer
	connectorId int
}

type Preparer struct {
	id int
}

func newPreparer(id int) *Preparer {
	return &Preparer{id}
}

func (p *Preparer) run() {
	for _, event := range events {
		p.prepare(event)
	}
}

func (p *Preparer) prepare(event *common.SendEvent) {
	log.Info("preparer#%d try create connection for mail#%d", p.id, event.Message.Id)
	// передаем событию сертификат и его длину
	event.CertBytes = p.certBytes
	event.CertBytesLen = p.certBytesLen

	connectionEvent := &ConnectionEvent{
		sendEvent:   event,
		servers:     make(chan *MailServer, 1),
		connectorId: p.id,
	}
	goto connectToMailServer

connectToMailServer:
	seekerEvents <- connectionEvent
	server := <-connectionEvent.servers
	switch server.status {
	case LookupMailServerStatus:
		goto waitLookup
		break
	case SuccessMailServerStatus:
		connectionEvent.server = server
		connectorEvents <- connectionEvent
		break
	case ErrorMailServerStatus:
		common.ReturnMail(
			event,
			errors.New(fmt.Sprintf("511 preparer#%d can't lookup %s", p.id, event.Message.HostnameTo)),
		)
		break
	}

waitLookup:
	log.Debug("preparer#%d wait ending look up mail server %s...", p.id, event.Message.HostnameTo)
	time.Sleep(common.SleepTimeout)
	goto connectToMailServer
}
