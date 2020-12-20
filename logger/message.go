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
	logger.Error().Str("hostname", m.Hostname).Str("stack", string(debug.Stack())).Msgf(message, args)
}

// пишет произвольную ошибку в лог и завершает программу
func (m *Message) FailExit(message string, args ...interface{}) {
	m.Err(message, args...)
	common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
}

// пишет системную ошибку в лог и завершает программу
func (m *Message) FailExitWithErr(err error) {
	logger.Error().Str("hostname", m.Hostname).Str("stack", string(debug.Stack())).Interface("error", err).Send()
	common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
}

// пишет произвольное предупреждение
func (m *Message) Warn(message string, args ...interface{}) {
	logger.Warn().Str("hostname", m.Hostname).Str("stack", string(debug.Stack())).Msgf(message, args)
}

// пишет системное предупреждение
func (m *Message) WarnWithErr(err error) {
	logger.Warn().Str("hostname", m.Hostname).Str("stack", string(debug.Stack())).Interface("error", err).Send()
}

// пишет информационное сообщение
func (m *Message) Info(message string, args ...interface{}) {
	logger.Info().Str("hostname", m.Hostname).Msgf(message, args)
}

// пишет сообщение для отладки
func (m *Message) Debug(message string, args ...interface{}) {
	logger.Debug().Str("hostname", m.Hostname).Str("stack", string(debug.Stack())).Msgf(message, args)
}
