package common

import (
	"flag"
	"fmt"
	"regexp"
	"runtime"
)

const (
	// используется в примерах использования
	ExampleConfigYaml = "/path/to/config/file.yaml"

	// невалидная строка, введенная пользователем
	InvalidInputString = ""

	// невалидное число, введенное пользователем
	InvalidInputInt = 0
)

var (
	// объект текущего приложения, иногда необходим сервисам, для отправки событий приложению
	App Application

	// сервисы, используются для создания итератора
	Services []interface{}

	// количество goroutine, может измениться для инициализации приложения
	DefaultWorkersCount = runtime.NumCPU()

	// используется в нескольких пакетах, поэтому вынес сюда
	FilenameRegex = regexp.MustCompile(`[^\\/]+\.[^\\/]+`)

	// печает аргументы, используемые приложением
	PrintUsage = func(f *flag.Flag) {
		format := "  -%s %s\n"
		fmt.Printf(format, f.Name, f.Usage)
	}
)

// проект содержит несколько приложений: pmq-grep, pmq-publish, pmq-report, postmanq и т.д.
// чтобы упростить и стандартизировать приложения, разработан этот интерфейс
type Application interface {
	GetConfigFilename() string
	// устанавливает путь к файлу с настройками
	SetConfigFilename(string)

	// проверяет валидность пути к файлу с настройками
	IsValidConfigFilename(string) bool

	// InitChannels init channels
	InitChannels(cBufSize int)

	// OnEvent runs event callback
	OnEvent(f func(ev *ApplicationEvent))

	// CloseEvents close events channel
	CloseEvents()

	// SendEvents send event to the channel
	SendEvents(ev *ApplicationEvent) bool

	// возвращает канал завершения приложения
	Done() <-chan bool

	// Close main app
	Close()

	// возвращает сервисы, используемые приложением
	Services() []interface{}

	// инициализирует сервисы
	FireInit(*ApplicationEvent, interface{})

	// запускает сервисы приложения
	FireRun(*ApplicationEvent, interface{})

	// останавливает сервисы приложения
	FireFinish(*ApplicationEvent, interface{})

	// инициализирует приложение
	Init(*ApplicationEvent)

	// запускает приложение
	Run()

	// запускает приложение с аргументами
	RunWithArgs(...interface{})

	// возвращает таймауты приложения
	Timeout() Timeout
}
