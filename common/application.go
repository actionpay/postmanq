package common

import (
	"flag"
	"fmt"
	"runtime"
)

const (
	ExampleConfigYaml  = "/path/to/config/file.yaml"
	InvalidInputString = ""
	InvalidInputInt    = 0
)

var (
	App                 Application
	Services     []interface{}
	DefaultWorkersCount = runtime.NumCPU()
	PrintUsage          = func(f *flag.Flag) {
		format := "  -%s %s\n"
		fmt.Printf(format, f.Name, f.Usage)
	}
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
