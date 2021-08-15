package logger

import (
	"runtime/debug"

	"github.com/rs/zerolog"

	"github.com/Halfi/postmanq/common"
)

var logger zerolog.Logger

// Message запись логирования
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

// Err пишет ошибку в лог
func (m *Message) Err(message string, args ...interface{}) {
	go func() {
		logger.Error().Str("hostname", m.Hostname).Str("stack", string(debug.Stack())).Msgf(message, args...)
	}()
}

func (m *Message) ErrErr(err error) {
	go func() {
		logger.Error().Str("hostname", m.Hostname).Str("stack", string(debug.Stack())).Interface("error", err).Err(err).Send()
	}()
}

// FailExit пишет произвольную ошибку в лог и завершает программу
func (m *Message) FailExit(message string, args ...interface{}) {
	m.Err(message, args...)
	common.App.SendEvents(common.NewApplicationEvent(common.FinishApplicationEventKind))
}

// FailExitWithErr пишет ошибку с сообщением в лог и завершает программу
func (m *Message) FailExitWithErr(err error, message string, args ...interface{}) {
	go func() {
		l := logger.Error().Str("hostname", m.Hostname).Str("stack", string(debug.Stack()))
		if err != nil {
			l = l.Interface("error", err)
		}

		l.Msgf(message, args...)
	}()
	common.App.SendEvents(common.NewApplicationEvent(common.FinishApplicationEventKind))
}

// FailExitErr пишет системную ошибку в лог и завершает программу
func (m *Message) FailExitErr(err error) {
	go func() {
		l := logger.Error().Str("hostname", m.Hostname).Str("stack", string(debug.Stack()))
		if err != nil {
			l = l.Interface("error", err)
		}

		l.Send()
	}()

	common.App.SendEvents(common.NewApplicationEvent(common.FinishApplicationEventKind))
}

// Warn пишет произвольное предупреждение
func (m *Message) Warn(message string, args ...interface{}) {
	go func() {
		logger.Warn().Str("hostname", m.Hostname).Msgf(message, args...)
	}()
}

// WarnErr пишет системное предупреждение
func (m *Message) WarnErr(err error) {
	go func() {
		l := logger.Warn().Str("hostname", m.Hostname)
		if err != nil {
			l = l.Interface("error", err)
		}
		l.Send()
	}()
}

// WarnWithErr пишет ошибку с сообщением
func (m *Message) WarnWithErr(err error, message string, args ...interface{}) {
	go func() {
		l := logger.Warn().Str("hostname", m.Hostname)
		if err != nil {
			l = l.Interface("error", err).Err(err)
		}
		l.Msgf(message, args...)
	}()
}

// Info пишет информационное сообщение
func (m *Message) Info(message string, args ...interface{}) {
	go func() { logger.Info().Str("hostname", m.Hostname).Msgf(message, args...) }()
}

// Debug пишет сообщение для отладки
func (m *Message) Debug(message string, args ...interface{}) {
	go func() {
		logger.Debug().Str("hostname", m.Hostname).Msgf(message, args...)
	}()
}
