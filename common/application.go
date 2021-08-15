package common

import (
	"flag"
	"fmt"
	"regexp"
	"runtime"
)

const (
	// ExampleConfigYaml используется в примерах использования
	ExampleConfigYaml = "/path/to/config/file.yaml"

	// InvalidInputString невалидная строка, введенная пользователем
	InvalidInputString = ""

	// InvalidInputInt невалидное число, введенное пользователем
	InvalidInputInt = 0
)

var (
	// App объект текущего приложения, иногда необходим сервисам, для отправки событий приложению
	App Application

	// Services сервисы, используются для создания итератора
	Services []interface{}

	// DefaultWorkersCount количество goroutine, может измениться для инициализации приложения
	DefaultWorkersCount = runtime.NumCPU()

	// FilenameRegex используется в нескольких пакетах, поэтому вынес сюда
	FilenameRegex = regexp.MustCompile(`[^\\/]+\.[^\\/]+`)

	// PrintUsage печает аргументы, используемые приложением
	PrintUsage = func(f *flag.Flag) {
		format := "  -%s %s\n"
		fmt.Printf(format, f.Name, f.Usage)
	}
)

// Application проект содержит несколько приложений: pmq-grep, pmq-publish, pmq-report, postmanq и т.д.
// чтобы упростить и стандартизировать приложения, разработан этот интерфейс
type Application interface {
	GetConfigFilename() string
	// SetConfigFilename устанавливает путь к файлу с настройками
	SetConfigFilename(string)

	// IsValidConfigFilename проверяет валидность пути к файлу с настройками
	IsValidConfigFilename(string) bool

	// InitChannels init channels
	InitChannels(cBufSize int)

	// OnEvent runs event callback
	OnEvent(f func(ev *ApplicationEvent))

	// CloseEvents close events channel
	CloseEvents()

	// SendEvents send event to the channel
	SendEvents(ev *ApplicationEvent) bool

	// Done возвращает канал завершения приложения
	Done() <-chan bool

	// Close main app
	Close()

	// Services возвращает сервисы, используемые приложением
	Services() []interface{}

	// FireInit инициализирует сервисы
	FireInit(*ApplicationEvent, interface{})

	// FireRun запускает сервисы приложения
	FireRun(*ApplicationEvent, interface{})

	// FireFinish останавливает сервисы приложения
	FireFinish(*ApplicationEvent, interface{})

	// Init инициализирует приложение
	Init(*ApplicationEvent)

	// Run запускает приложение
	Run()

	// RunWithArgs запускает приложение с аргументами
	RunWithArgs(...interface{})

	// Timeout возвращает таймауты приложения
	Timeout() Timeout
}
