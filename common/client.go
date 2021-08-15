package common

import (
	"fmt"
	"net"
	"net/smtp"
	"time"
)

// SmtpClientStatus статус клиента почтового сервера
type SmtpClientStatus int

const (
	// WorkingSmtpClientStatus отсылает письмо
	WorkingSmtpClientStatus SmtpClientStatus = iota

	// WaitingSmtpClientStatus ожидает письма
	WaitingSmtpClientStatus

	// DisconnectedSmtpClientStatus отсоединен
	DisconnectedSmtpClientStatus
)

// SmtpClient клиент почтового сервера
type SmtpClient struct {
	// идертификатор клиента для удобства в логах
	Id int

	// Conn соединение к почтовому серверу
	Conn net.Conn

	// Worker реальный smtp клиент
	Worker *smtp.Client

	// ModifyDate дата создания или изменения статуса клиента
	ModifyDate time.Time

	// Status статус
	Status SmtpClientStatus

	// таймер, по истечении которого, соединение к почтовому сервису будет разорвано
	timer *time.Timer
}

// SetTimeout останавливает таймаут на чтение и запись соединения
func (s *SmtpClient) SetTimeout(timeout time.Duration) error {
	err := s.Conn.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return fmt.Errorf("can't set smtp connection timeout: %w", err)
	}
	return nil
}

// Wait переводит клиента в ожидание
// после окончания ожидания соединение разрывается, а статус меняется на отсоединенный
func (s *SmtpClient) Wait() {
	s.Status = WaitingSmtpClientStatus
	s.timer = time.AfterFunc(App.Timeout().Waiting, func() {
		s.Status = DisconnectedSmtpClientStatus
		_ = s.Worker.Quit()
		s.timer = nil
	})
}

// Wakeup переводит клиента в рабочее состояние
// если клиент был в ожидании, ожидание прерывается
func (s *SmtpClient) Wakeup() {
	s.Status = WorkingSmtpClientStatus
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}
