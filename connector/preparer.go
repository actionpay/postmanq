package connector

import (
	"errors"
	"fmt"
	"github.com/sergw3x/postmanq/common"
	"github.com/sergw3x/postmanq/logger"
	"time"
)

// заготовщик, подготавливает событие соединения
type Preparer struct {
	// Идентификатор для логов
	id int
}

// создает и запускает нового заготовщика
func newPreparer(id int) {
	preparer := &Preparer{id}
	preparer.run()
}

// запускает прослушивание событий отправки писем
func (p *Preparer) run() {
	for event := range events {
		p.prepare(event)
	}
}

// подготавливает и запускает событие создание соединения
func (p *Preparer) prepare(event *common.SendEvent) {
	logger.Info("preparer#%d-%d try create connection", p.id, event.Message.Id)

	connectionEvent := &ConnectionEvent{
		SendEvent:   event,
		servers:     make(chan *MailServer, 1),
		connectorId: p.id,
		address:     service.Addresses[p.id%service.addressesLen],
	}
	goto connectToMailServer

connectToMailServer:
	// отправляем событие сбора информации о сервере
	seekerEvents <- connectionEvent
	server := <-connectionEvent.servers
	switch server.status {
	case LookupMailServerStatus:
		goto waitLookup
	case SuccessMailServerStatus:
		connectionEvent.server = server
		connectorEvents <- connectionEvent
	case ErrorMailServerStatus:
		common.ReturnMail(
			event,
			errors.New(fmt.Sprintf("511 preparer#%d-%d can't lookup %s", p.id, event.Message.Id, event.Message.HostnameTo)),
		)
	}
	return

waitLookup:
	logger.Debug("preparer#%d-%d wait ending look up mail server %s...", p.id, event.Message.Id, event.Message.HostnameTo)
	time.Sleep(common.App.Timeout().Sleep)
	goto connectToMailServer
	return
}
