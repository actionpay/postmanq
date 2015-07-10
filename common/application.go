package common

import (
	"flag"
	"fmt"
	"regexp"
	"runtime"
)

const (
	// Используется в примерах использования
	ExampleConfigYaml = "/path/to/config/file.yaml"

	InvalidInputString = ""
	InvalidInputInt    = 0
)

var (
	// Объект текущего приложения, иногда необходим сервисам, для отправки событий приложению
	App Application

	// Сервисы, используются для создания итератора
	Services []interface{}

	// Количество goroutine, может измениться для инициализации приложения
	DefaultWorkersCount = runtime.NumCPU()

	// Используется в нескольких пакетах, поэтому вынес
	FilenameRegex = regexp.MustCompile(`[^\\/]+\.[^\\/]+`)

	PrintUsage = func(f *flag.Flag) {
		format := "  -%s %s\n"
		fmt.Printf(format, f.Name, f.Usage)
	}
)

// Проект содержит несколько приложений: pmq-grep, pmq-publish, pmq-report, postmanq
// Чтобы упростить и стандартизировать приложения, разработан этот интерфейс
type Application interface {
	SetConfigFilename(string)
	IsValidConfigFilename(string) bool
	SetEvents(chan *ApplicationEvent)
	Events() chan *ApplicationEvent
	SetDone(chan bool)
	Done() chan bool
	Services() []interface{}
	FireInit(*ApplicationEvent, interface{})
	FireRun(*ApplicationEvent, interface{})
	FireFinish(*ApplicationEvent, interface{})
	Init(*ApplicationEvent)
	Run()
	RunWithArgs(...interface{})
	Timeout() Timeout
}
