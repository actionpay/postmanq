package logger

import (
	"fmt"
	"github.com/AdOnWeb/postmanq/common"
	"runtime/debug"
)

// посылает сервису логирования запись для логирования произвольного уровня
func log(message string, necessaryLevel Level, args ...interface{}) {
	defer func() { recover() }()
	// если уровень записи не ниже уровня сервиса логирования
	// запись посылается сервису
	if level <= necessaryLevel {
		// если уровень выше "info", значит пишется ошибка
		// добавляем к сообщению стек, чтобы посмотреть в чем дело
		if necessaryLevel > InfoLevel && necessaryLevel == DebugLevel {
			message = fmt.Sprint(message, "\n", string(debug.Stack()))
		}
		messages <- NewMessage(necessaryLevel, message, args...)
	}
}

// пишет ошибку в лог
func Err(message string, args ...interface{}) {
	log(message, ErrorLevel, args...)
}

// пишет произвольную ошибку в лог и завершает программу
func FailExit(message string, args ...interface{}) {
	Err(message, args...)
	common.App.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
}

// пишет системную ошибку в лог и завершает программу
func FailExitWithErr(err error) {
	FailExit("%v", err)
}

// пишет произвольное предупреждение
func Warn(message string, args ...interface{}) {
	log(message, WarningLevel, args...)
}

// пишет системное предупреждение
func WarnWithErr(err error) {
	Warn("%v", err)
}

// пишет информационное сообщение
func Info(message string, args ...interface{}) {
	log(message, InfoLevel, args...)
}

// пишет сообщение для отладки
func Debug(message string, args ...interface{}) {
	log(message, DebugLevel, args...)
}
