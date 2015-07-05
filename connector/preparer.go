package connector

import (
	"errors"
	"fmt"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
	"time"
)

// Заготовщик, подготавливает событие соединения
type Preparer struct {
	// Идентификатор для логов
	id int
}

// Создает и запускает нового заготовщика
func newPreparer(id int) {
	preparer := &Preparer{id}
	preparer.run()
}

// Запускает прослушивание событий отправки писем
func (p *Preparer) run() {
	for event := range events {
		p.prepare(event)
	}
}

// Подготавливает и запускает событие создание соединения
func (p *Preparer) prepare(event *common.SendEvent) {
	logger.Info("preparer#%d try create connection for mail#%d", p.id, event.Message.Id)

	connectionEvent := &ConnectionEvent{
		SendEvent:   event,
		servers:     make(chan *MailServer, 1),
		connectorId: p.id,
		address:     service.Addresses[p.id % service.addressesLen],
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
			errors.New(fmt.Sprintf("511 preparer#%d can't lookup %s", p.id, event.Message.HostnameTo)),
		)
	}
	return

waitLookup:
	logger.Debug("preparer#%d wait ending look up mail server %s...", p.id, event.Message.HostnameTo)
	time.Sleep(common.SleepTimeout)
	goto connectToMailServer
	return
}
