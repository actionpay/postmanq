package connector

import (
	"github.com/AdOnWeb/postmanq/common"
	"net"
)

type MailServerStatus int

const (
	LookupMailServerStatus MailServerStatus = iota
	SuccessMailServerStatus
	ErrorMailServerStatus
)

// почтовый сервис
type MailServer struct {
	// серверы почтового сервиса
	mxServers []*MxServer

	// индекс последнего почтового сервиса
	lastIndex int

	// номер потока, собирающего информацию о почтовом сервисе
	connectorId int

	// статус, говорящий о том, собранали ли информация о почтовом сервисе
	status MailServerStatus
}

// почтовый сервер
type MxServer struct {
	// доменное имя почтового сервера
	hostname       string

	// IP сервера
	ips            []net.IP

	// клиенты сервера
	clients        []*common.SmtpClient

	// А запись сервера
	realServerName string

	// использоватение TLS
	useTLS         bool

	queues map[string]*common.LimitedQueue
}

func newMxServer(hostname string) *MxServer {
	queues := make(map[string]*common.LimitedQueue)
	for _, address := range service.Addresses {
		queues[address] = common.NewLimitQueue()
	}

	return &MxServer{
		hostname: hostname,
		ips: make([]net.IP, 0),
		useTLS: true,
		queues: queues,
	}
}

// запрещает использовать TLS соединения
func (this *MxServer) dontUseTLS() {
	this.useTLS = false
}
