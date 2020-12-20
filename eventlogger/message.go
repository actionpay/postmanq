package eventlogger

import (
	"github.com/Halfi/postmanq/common"
	"runtime/debug"
)

// запись логирования
type Message struct {
	Hostname string
	// сообщение для лога, может содержать параметры
	Message string

	// уровень логирования записи, необходим для отсечения лишних записей
	Level Level

	// аргументы для параметров сообщения
	Args []interface{}

	Stack []byte
}

// созадние новой записи логирования
func NewMessage(level Level, message string, args ...interface{}) *Message {
	logMessage := new(Message)
	logMessage.Level = level
	logMessage.Message = message
	logMessage.Args = args
	return logMessage
}

func All() *Message {
	return By(common.AllDomains)
}

func By(hostname string) *Message {
	return &Message{
		Hostname: hostname,
	}
}

func (m *Message) log(message string, necessaryLevel Level, args ...interface{}) {
	m.Message = message
	m.Level = necessaryLevel
	m.Args = args

	if necessaryLevel > InfoLevel || necessaryLevel == DebugLevel {
		m.Stack = debug.Stack()
	}

	messages <- m
}

// пишет ошибку в лог
func (m *Message) Err(message string, args ...interface{}) {
	go m.log(message, ErrorLevel, args...)
}

// пишет произвольную ошибку в лог и завершает программу
func (m *Message) FailExit(message string, args ...interface{}) {
	m.Err(message, args...)
	common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
}

// пишет системную ошибку в лог и завершает программу
func (m *Message) FailExitWithErr(err error) {
	m.FailExit("%v", err)
}

// пишет произвольное предупреждение
func (m *Message) Warn(message string, args ...interface{}) {
	go m.log(message, WarningLevel, args...)
}

// пишет системное предупреждение
func (m *Message) WarnWithErr(err error) {
	m.Warn("%v", err)
}

// пишет информационное сообщение
func (m *Message) Info(message string, args ...interface{}) {
	go m.log(message, InfoLevel, args...)
}

// пишет сообщение для отладки
func (m *Message) Debug(message string, args ...interface{}) {
	go m.log(message, DebugLevel, args...)
}
