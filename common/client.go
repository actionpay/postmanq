package common

import (
	"net"
	"net/smtp"
	"time"
)

// статус клиента почтового сервера
type SmtpClientStatus int

const (
	// отсылает письмо
	WorkingSmtpClientStatus SmtpClientStatus = iota
	// ожидает письма
	WaitingSmtpClientStatus
	// отсоединен
	DisconnectedSmtpClientStatus
)

// клиент почтового сервера
type SmtpClient struct {
	// номер клиента для удобства в логах
	Id int
	// соединение к почтовому серверу
	Conn net.Conn
	// реальный smtp клиент
	Worker *smtp.Client
	// дата создания или изменения статуса клиента
	ModifyDate time.Time
	// статус
	Status SmtpClientStatus

	timer *time.Timer
}

func (s *SmtpClient) SetTimeout(timeout time.Duration) {
	s.Conn.SetDeadline(time.Now().Add(timeout))
}

func (s *SmtpClient) Wait() {
	s.Status = WaitingSmtpClientStatus
	s.timer = time.AfterFunc(WaitingTimeout, func() {
		s.Status = DisconnectedSmtpClientStatus
		s.Worker.Close()
		s.timer = nil
	})
}

func (s *SmtpClient) Wakeup() {
	s.Status = WorkingSmtpClientStatus
	if s.timer != nil {
		s.timer.Stop()
	}
}
