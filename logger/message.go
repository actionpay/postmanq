package logger

import (
	"runtime/debug"

	"github.com/Halfi/postmanq/common"
)

// запись логирования
type Message struct {
	Hostname string

	Stack []byte
}

func All() *Message {
	return By(common.AllDomains)
}

func By(hostname string) *Message {
	return &Message{
		Hostname: hostname,
	}
}

// пишет ошибку в лог
func (m *Message) Err(message string, args ...interface{}) {
	go func() {
		logger.Error().Str("hostname", m.Hostname).Str("stack", string(debug.Stack())).Msgf(message, args)
	}()
}

// пишет произвольную ошибку в лог и завершает программу
func (m *Message) FailExit(message string, args ...interface{}) {
	m.Err(message, args...)
	common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
}

// пишет ошибку с сообщением в лог и завершает программу
func (m *Message) FailExitWithErr(err error, message string, args ...interface{}) {
	go func() {
		l := logger.Error().Str("hostname", m.Hostname).Str("stack", string(debug.Stack()))
		if err != nil {
			l = l.Interface("error", err)
		}

		l.Msgf(message, args)
	}()
	common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
}

// пишет системную ошибку в лог и завершает программу
func (m *Message) FailExitErr(err error) {
	go func() {
		l := logger.Error().Str("hostname", m.Hostname).Str("stack", string(debug.Stack()))
		if err != nil {
			l = l.Interface("error", err)
		}

		l.Send()
	}()

	common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
}

// пишет произвольное предупреждение
func (m *Message) Warn(message string, args ...interface{}) {
	go func() {
		logger.Warn().Str("hostname", m.Hostname).Str("stack", string(debug.Stack())).Msgf(message, args)
	}()
}

// пишет системное предупреждение
func (m *Message) WarnErr(err error) {
	go func() {
		l := logger.Warn().Str("hostname", m.Hostname).Str("stack", string(debug.Stack()))
		if err != nil {
			l = l.Interface("error", err)
		}
		l.Send()
	}()
}

// пишет ошибку с сообщением
func (m *Message) WarnWithErr(err error, message string, args ...interface{}) {
	go func() {
		l := logger.Warn().Str("hostname", m.Hostname).Str("stack", string(debug.Stack()))
		if err != nil {
			l = l.Interface("error", err).Err(err)
		}
		l.Msgf(message, args)
	}()
}

// пишет информационное сообщение
func (m *Message) Info(message string, args ...interface{}) {
	go func() { logger.Info().Str("hostname", m.Hostname).Msgf(message, args) }()
}

// пишет сообщение для отладки
func (m *Message) Debug(message string, args ...interface{}) {
	go func() {
		logger.Debug().Str("hostname", m.Hostname).Str("stack", string(debug.Stack())).Msgf(message, args)
	}()
}
