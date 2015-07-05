package common

import (
	"net"
	"net/smtp"
	"time"
)

// Статус клиента почтового сервера
type SmtpClientStatus int

const (
	// Отсылает письмо
	WorkingSmtpClientStatus SmtpClientStatus = iota

	// Ожидает письма
	WaitingSmtpClientStatus

	// Отсоединен
	DisconnectedSmtpClientStatus
)

// Клиент почтового сервера
type SmtpClient struct {
	// Идертификатор клиента для удобства в логах
	Id int

	// Соединение к почтовому серверу
	Conn net.Conn

	// Реальный smtp клиент
	Worker *smtp.Client

	// Дата создания или изменения статуса клиента
	ModifyDate time.Time

	// Статус
	Status SmtpClientStatus

	// Таймер, по истечении которого, соединение к почтовому сервису будет разорвано
	timer *time.Timer
}

// Устанавливайт таймаут на чтение и запись соединения
func (s *SmtpClient) SetTimeout(timeout time.Duration) {
	s.Conn.SetDeadline(time.Now().Add(timeout))
}

// Переводит клиента в ожидание
// После окончания ожидания соединение разрывается, а статус меняется на отсоединенный
func (s *SmtpClient) Wait() {
	s.Status = WaitingSmtpClientStatus
	s.timer = time.AfterFunc(WaitingTimeout, func() {
		s.Status = DisconnectedSmtpClientStatus
		s.Worker.Close()
		s.timer = nil
	})
}

// Переводит клиента в рабочее состояние
// Если клиент был в ожидании, ожидание прерывается
func (s *SmtpClient) Wakeup() {
	s.Status = WorkingSmtpClientStatus
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}
