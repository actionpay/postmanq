package connector

import (
	"github.com/AdOnWeb/postmanq/common"
	"net"
)

// Статус почтового сервис
type MailServerStatus int

const (
	// По сервису ведется поиск информации
	LookupMailServerStatus MailServerStatus = iota

	// По сервису успешно собрана информация
	SuccessMailServerStatus

	// По сервису не удалось собрать информацию
	ErrorMailServerStatus
)

// Почтовый сервис
type MailServer struct {
	// Серверы почтового сервиса
	mxServers []*MxServer

	// Номер потока, собирающего информацию о почтовом сервисе
	connectorId int

	// Статус, говорящий о том, собранали ли информация о почтовом сервисе
	status MailServerStatus
}

// Почтовый сервер
type MxServer struct {
	// Доменное имя почтового сервера
	hostname string

	// IP сервера
	ips []net.IP

	// Клиенты сервера
	clients []*common.SmtpClient

	// А запись сервера
	realServerName string

	// Использоватение TLS
	useTLS bool

	// Очередь клиентов
	queues map[string]*common.LimitedQueue
}

// Создает новый почтовый сервер
func newMxServer(hostname string) *MxServer {
	queues := make(map[string]*common.LimitedQueue)
	for _, address := range service.Addresses {
		queues[address] = common.NewLimitQueue()
	}

	return &MxServer{
		hostname: hostname,
		ips:      make([]net.IP, 0),
		useTLS:   true,
		queues:   queues,
	}
}

// Запрещает использовать TLS соединения
func (m *MxServer) dontUseTLS() {
	m.useTLS = false
}
