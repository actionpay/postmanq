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
		logger.Debug("seeker#%d-%d create mail server for %s", event.connectorId, event.Message.Id, hostnameTo)
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
		logger.Debug("seeker#%d-%d look up mx domains for %s...", s.id, event.Message.Id, hostnameTo)
		// ищем почтовые сервера для домена
		mxes, err := net.LookupMX(hostnameTo)
		if err == nil {
			mailServer.mxServers = make([]*MxServer, len(mxes))
			for i, mx := range mxes {
				mxHostname := strings.TrimRight(mx.Host, ".")
				logger.Debug("seeker#%d-%d look up mx domain %s for %s", s.id, event.Message.Id, mxHostname, hostnameTo)
				mxServer := newMxServer(mxHostname)
				mxServer.realServerName = s.seekRealServerName(mx.Host, event)
				// собираем IP адреса для сертификата и проверок
				//ips, err := net.LookupIP(mxHostname)
				//if err == nil {
				//	for _, ip := range ips {
				//		// берем только IPv4
				//		ip = ip.To4()
				//		if ip != nil {
				//			logger.Debug("seeker#%d-%d look up ip %s for %s", s.id, event.Message.Id, ip.String(), mxHostname)
				//			existsIpsLen := len(mxServer.ips)
				//			index := sort.Search(existsIpsLen, func(i int) bool {
				//				return mxServer.ips[i].Equal(ip)
				//			})
				//			// избавляемся от повторяющихся IP адресов
				//			if existsIpsLen == 0 || (index == -1 && existsIpsLen > 0) {
				//				mxServer.ips = append(mxServer.ips, ip)
				//			}
				//		}
				//	}
				//	// домен почтового ящика может отличаться от домена почтового сервера,
				//	// а домен почтового сервера может отличаться от реальной A записи сервера,
				//	// на котором размещен этот почтовый сервер
				//	// нам необходимо получить реальный домен, для того чтобы подписать на него сертификат
				//	for _, ip := range mxServer.ips {
				//		// пытаемся получить адреса сервера
				//		addrs, err := net.LookupAddr(ip.String())
				//		if err == nil {
				//			for _, addr := range addrs {
				//				// адрес получаем с точкой на конце, убираем ее
				//				addr = strings.TrimRight(addr, ".")
				//				// отсекаем адрес, если это IP
				//				if net.ParseIP(addr) == nil {
				//					logger.Debug("seeker#%d-%d look up addr %s for ip %s", s.id, event.Message.Id, addr, ip.String())
				//					if len(mxServer.realServerName) == 0 {
				//						// пытаем найти домен почтового сервера в домене почты
				//						hostnameMatched, _ := regexp.MatchString(hostnameTo, mxServer.hostname)
				//						// пытаемся найти адрес в домене почтового сервиса
				//						addrMatched, _ := regexp.MatchString(mxServer.hostname, addr)
				//						// если найден домен почтового сервера в домене почты
				//						// тогда в адресе будет PTR запись
				//						if hostnameMatched && !addrMatched {
				//							mxServer.realServerName = addr
				//						} else if !hostnameMatched && addrMatched || !hostnameMatched && !addrMatched { // если найден адрес в домене почтового сервиса или нет совпадений
				//							mxServer.realServerName = mxServer.hostname
				//						}
				//					}
				//				}
				//			}
				//		} else {
				//			logger.Warn("seeker#%d-%d can't look up addr for ip %s", s.id, event.Message.Id, ip.String())
				//		}
				//	}
				//} else {
				//	logger.Warn("seeker#%d-%d can't look up ips for mx %s", s.id, event.Message.Id, mxHostname)
				//}
				//if len(mxServer.realServerName) == 0 { // если безвыходная ситуация
				//	mxServer.realServerName = mxServer.hostname
				//}
				logger.Debug("seeker#%d-%d look up detect real server name %s", s.id, event.Message.Id, mxServer.realServerName)
				mailServer.mxServers[i] = mxServer
			}
			mailServer.status = SuccessMailServerStatus
			logger.Debug("seeker#%d-%d look up %s success", s.id, event.Message.Id, hostnameTo)
		} else {
			mailServer.status = ErrorMailServerStatus
			logger.Warn("seeker#%d-%d can't look up mx domains for %s, err: %v", s.id, event.Message.Id, hostnameTo, err)
		}
	}
	event.servers <- mailServer
}

func (s *Seeker) seekRealServerName(hostname string, event *ConnectionEvent) string {
	parts := strings.Split(hostname, ".")
	partsLen := len(parts)
	var lookupHostname string
	if partsLen > 2 {
		lookupHostname = strings.Join(parts[partsLen-3:partsLen-1], ".")
	} else {
		lookupHostname = strings.Join(parts, ".")
	}
	mxes, err := net.LookupMX(lookupHostname)
	if err == nil && len(mxes) > 0 {
		if strings.Contains(mxes[0].Host, lookupHostname) {
			return hostname
		} else {
			return s.seekRealServerName(mxes[0].Host, event)
		}
	} else {
		logger.Warn("seeker#%d-%d can't look up real mx domains for %s, err: %v", s.id, event.Message.Id, lookupHostname, err)
		return hostname
	}
}
