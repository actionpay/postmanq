package connector

import (
	"github.com/actionpay/postmanq/logger"
	"net"
	"strings"
	"sync"
)

var (
	seekerEvents = make(chan *ConnectionEvent)
	// семафор, необходим для поиска MX серверов
	seekerMutex = new(sync.Mutex)
)

// искатель, ищет информацию о сервере
type Seeker struct {
	// Идентификатор для логов
	id int
}

// создает и запускает нового искателя
func newSeeker(id int) {
	seeker := &Seeker{id}
	seeker.run()
}

// запускает прослушивание событий поиска информации о сервере
func (s *Seeker) run() {
	for event := range seekerEvents {
		s.seek(event)
	}
}

// ищет информацию о сервере
func (s *Seeker) seek(event *ConnectionEvent) {
	hostnameTo := event.Message.HostnameTo
	// добавляем новый почтовый домен
	seekerMutex.Lock()
	if _, ok := mailServers[hostnameTo]; !ok {
		logger.By(event.Message.HostnameFrom).Debug("seeker#%d-%d create mail server for %s", event.connectorId, event.Message.Id, hostnameTo)
		mailServers[hostnameTo] = &MailServer{
			status:      LookupMailServerStatus,
			connectorId: event.connectorId,
		}
	}
	seekerMutex.Unlock()
	mailServer := mailServers[hostnameTo]
	// если пришло несколько несколько писем на один почтовый сервис,
	// и информация о сервисе еще не собрана,
	// то таким образом блокируем повторную попытку собрать инфомацию о почтовом сервисе
	if event.connectorId == mailServer.connectorId && mailServer.status == LookupMailServerStatus {
		logger.By(event.Message.HostnameFrom).Debug("seeker#%d-%d look up mx domains for %s...", s.id, event.Message.Id, hostnameTo)
		mailServer := mailServers[hostnameTo]
		// ищем почтовые сервера для домена
		mxes, err := net.LookupMX(hostnameTo)
		if err == nil {
			mailServer.mxServers = make([]*MxServer, len(mxes))
			for i, mx := range mxes {
				mxHostname := strings.TrimRight(mx.Host, ".")
				logger.By(event.Message.HostnameFrom).Debug("seeker#%d-%d look up mx domain %s for %s", s.id, event.Message.Id, mxHostname, hostnameTo)
				mxServer := newMxServer(mxHostname, event.Message.HostnameFrom)
				mxServer.realServerName = s.seekRealServerName(mx.Host)
				logger.By(event.Message.HostnameFrom).Debug("seeker#%d-%d look up detect real server name %s", s.id, event.Message.Id, mxServer.realServerName)
				mailServer.mxServers[i] = mxServer
			}
			mailServer.status = SuccessMailServerStatus
			logger.By(event.Message.HostnameFrom).Debug("seeker#%d-%d look up %s success", s.id, event.Message.Id, hostnameTo)
		} else {
			mailServer.status = ErrorMailServerStatus
			logger.By(event.Message.HostnameFrom).Warn("seeker#%d-%d can't look up mx domains for %s", s.id, event.Message.Id, hostnameTo)
		}
	}
	event.servers <- mailServer
}

func (s *Seeker) seekRealServerName(hostname string) string {
	parts := strings.Split(hostname, ".")
	partsLen := len(parts)
	hostname = strings.Join(parts[partsLen-3:partsLen-1], ".")
	mxes, err := net.LookupMX(hostname)
	if err == nil {
		if strings.Contains(mxes[0].Host, hostname) {
			return hostname
		} else {
			return s.seekRealServerName(mxes[0].Host)
		}
	} else {
		return hostname
	}
}
