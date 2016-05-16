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
	// идертификатор клиента для удобства в логах
	Id int

	// соединение к почтовому серверу
	Conn net.Conn

	// реальный smtp клиент
	Worker *smtp.Client

	// дата создания или изменения статуса клиента
	ModifyDate time.Time

	// статус
	Status SmtpClientStatus

	// таймер, по истечении которого, соединение к почтовому сервису будет разорвано
	timer *time.Timer
}

// сстанавливайт таймаут на чтение и запись соединения
func (s *SmtpClient) SetTimeout(timeout time.Duration) {
	s.Conn.SetDeadline(time.Now().Add(timeout))
}

// переводит клиента в ожидание
// после окончания ожидания соединение разрывается, а статус меняется на отсоединенный
func (s *SmtpClient) Wait() {
	s.Status = WaitingSmtpClientStatus
	s.timer = time.AfterFunc(App.Timeout().Waiting, func() {
		s.Status = DisconnectedSmtpClientStatus
		s.Worker.Close()
		s.timer = nil
	})
}

// переводит клиента в рабочее состояние
// если клиент был в ожидании, ожидание прерывается
func (s *SmtpClient) Wakeup() {
	s.Status = WorkingSmtpClientStatus
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}
