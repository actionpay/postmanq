package guardian

import (
	"github.com/actionpay/postmanq/common"
	"github.com/actionpay/postmanq/logger"
	"sort"
)

// защитник, блокирует отправку на указанные почтовые сервисы
type Guardian struct {
	// идентификатор для логов
	id int
}

// создает нового защитника
func newGuardian(id int) {
	guardian := &Guardian{id}
	guardian.run()
}

// запускает прослушивание событий отправки писем
func (g *Guardian) run() {
	for event := range events {
		g.guard(event)
	}
}

// блокирует отправку на указанные почтовые сервисы
func (g *Guardian) guard(event *common.SendEvent) {
	logger.Info("guardian#%d-%d check mail", g.id, event.Message.Id)
	hostnameTo := event.Message.HostnameTo
	i := sort.Search(service.hostnameLen, func(i int) bool {
		return service.Hostnames[i] == hostnameTo
	})
	if i < service.hostnameLen && service.Hostnames[i] == hostnameTo {
		logger.Debug("guardian#%d-%d detect postal worker - %s, revoke sending mail", g.id, event.Message.Id, hostnameTo)
		event.Result <- common.RevokeSendEventResult
	} else {
		logger.Debug("guardian#%d-%d continue sending mail", g.id, event.Message.Id)
		event.Iterator.Next().(common.SendingService).Events() <- event
	}
}
