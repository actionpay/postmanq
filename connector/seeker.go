package connector

import (
	"github.com/AdOnWeb/postmanq/logger"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var (
	seekerEvents = make(chan *ConnectionEvent)
	// семафор, необходим для поиска MX серверов
	seekerMutex = new(sync.Mutex)
)

type Seeker struct {
	id int
}

func newSeeker(id int) *Seeker {
	return &Seeker{id}.run()
}

func (s *Seeker) run() {
	for _, event := range seekerEvents {
		s.seek(event)
	}
}

func (s *Seeker) seek(event *ConnectionEvent) {
	hostnameTo := event.Message.HostnameTo
	seekerMutex.Lock()
	if _, ok := mailServers[hostnameTo]; !ok {
		logger.Debug("seeker#%d create mail server for %s", event.connectorId, hostnameTo)
		mailServers[hostnameTo] = &MailServer{
			status:      LookupMailServerStatus,
			connectorId: event.connectorId,
		}
	}
	seekerMutex.Unlock()
	mailServer := mailServers[hostnameTo]
	if event.connectorId == mailServer.connectorId && mailServer.status == LookupMailServerStatus {
		logger.Debug("seeker#%d look up mx domains for %s...", s.id, hostnameTo)
		mailServer := mailServers[hostnameTo]
		// ищем почтовые сервера для домена
		mxes, err := net.LookupMX(hostnameTo)
		if err == nil {
			mailServer.mxServers = make([]*MxServer, len(mxes))
			for i, mx := range mxes {
				mxHostname := strings.TrimRight(mx.Host, ".")
				logger.Debug("seeker#%d look up mx domain %s for %s", s.id, mxHostname, hostnameTo)
				mxServer := newMxServer(mxHostname)
				// собираем IP адреса для сертификата и проверок
				ips, err := net.LookupIP(mxHostname)
				if err == nil {
					for _, ip := range ips {
						// берем только IPv4
						ip = ip.To4()
						if ip != nil {
							logger.Debug("seeker#%d look up ip %s for %s", s.id, ip.String(), mxHostname)
							existsIpsLen := len(mxServer.ips)
							index := sort.Search(existsIpsLen, func(i int) bool {
								return mxServer.ips[i].Equal(ip)
							})
							// избавляемся от повторяющихся IP адресов
							if existsIpsLen == 0 || (index == -1 && existsIpsLen > 0) {
								mxServer.ips = append(mxServer.ips, ip)
							}
						}
					}
					// домен почтового ящика может отличаться от домена почтового сервера,
					// а домен почтового сервера может отличаться от реальной A записи сервера,
					// на котором размещен этот почтовый сервер
					// нам необходимо получить реальный домен, для того чтобы подписать на него сертификат
					for _, ip := range mxServer.ips {
						// пытаемся получить адреса сервера
						addrs, err := net.LookupAddr(ip.String())
						if err == nil {
							for _, addr := range addrs {
								// адрес получаем с точкой на конце, убираем ее
								addr = strings.TrimRight(addr, ".")
								// отсекаем адрес, если это IP
								if net.ParseIP(addr) == nil {
									logger.Debug("seeker#%d look up addr %s for ip %s", s.id, addr, ip.String())
									if len(mxServer.realServerName) == 0 {
										// пытаем найти домен почтового сервера в домене почты
										hostnameMatched, _ := regexp.MatchString(hostnameTo, mxServer.hostname)
										// пытаемся найти адрес в домене почтового сервиса
										addrMatched, _ := regexp.MatchString(mxServer.hostname, addr)
										// если найден домен почтового сервера в домене почты
										// тогда в адресе будет PTR запись
										if hostnameMatched && !addrMatched {
											mxServer.realServerName = addr
										} else if !hostnameMatched && addrMatched || !hostnameMatched && !addrMatched { // если найден адрес в домене почтового сервиса или нет совпадений
											mxServer.realServerName = mxServer.hostname
										}
									}
								}
							}
						} else {
							logger.Warn("seeker#%d can't look up addr for ip %s", s.id, ip.String())
						}
					}
				} else {
					logger.Warn("seeker#%d can't look up ips for mx %s", s.id, mxHostname)
				}
				if len(mxServer.realServerName) == 0 { // если безвыходная ситуация
					mxServer.realServerName = mxServer.hostname
				}
				logger.Debug("seeker#%d look up detect real server name %s", s.id, mxServer.realServerName)
				mailServer.mxServers[i] = mxServer
			}
			mailServer.lastIndex = len(mailServer.mxServers) - 1
			mailServer.status = SuccessMailServerStatus
			logger.Debug("seeker#%d look up %s success", s.id, hostnameTo)
		} else {
			mailServer.status = ErrorMailServerStatus
			logger.Warn("seeker#%d can't look up mx domains for %s", s.id, hostnameTo)
		}
	}
	event.servers <- mailServer
}
