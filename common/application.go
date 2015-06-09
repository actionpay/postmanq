package common

import "runtime"

const (
	ExampleConfigYaml  = "/path/to/config/file.yaml"
	InvalidInputString = ""
	InvalidInputInt    = 0
)

var (
	App                 Application
	SendindServices     []interface{}
	DefaultWorkersCount = runtime.NumCPU()
)

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
	Run()
	RunWithArgs(...interface{})
}
