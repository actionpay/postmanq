package postmanq

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"bufio"
	"fmt"
	"time"
	yaml "gopkg.in/yaml.v2"
)

type LogLevel int

const(
	LOG_LEVEL_INFO = iota
	LOG_LEVEL_WARNING
	LOG_LEVEL_ERROR
	LOG_LEVEL_CRITICAL
)

var (
	filenameRegex = regexp.MustCompile(`[^\\/]+\.[^\\/]+`)
	logLevelNames = map[LogLevel]string{
		LOG_LEVEL_INFO:     "Info",
		LOG_LEVEL_WARNING:  "Warning",
		LOG_LEVEL_ERROR:    "Error",
		LOG_LEVEL_CRITICAL: "Critical",
	}
)

type LogMessage struct {
	Message string
	Level   LogLevel
	Args    []interface{}
}

func NewLogMessage(level LogLevel, message string, args ...interface{}) *LogMessage {
	logMessage := new(LogMessage)
	logMessage.Level = level
	logMessage.Message = message
	logMessage.Args = args
	return logMessage
}

type Logger struct {
	Debug   bool              `yaml:"debug"`
	Output  string            `yaml:"output"`
	writer  *bufio.Writer
	messages chan *LogMessage
}

func NewLogger() *Logger {
	logger := new(Logger)
	return logger
}

func (this *Logger) OnRegister(event *RegisterEvent) {
	this.Debug = true
	this.messages = make(chan *LogMessage)
	this.initWriter()
	go this.start()
	event.LogChan = this.messages
	event.Group.Done()
}

func (this *Logger) OnInit(event *InitEvent) {
	err := yaml.Unmarshal(event.Data, this)
	if err != nil {
		FailExit("%v", err)
	}
	this.writer = nil
	this.initWriter()
}

func (this *Logger) OnFinish(event *FinishEvent) {
	close(this.messages)
	event.Group.Done()
}

func (this *Logger) start() {
	for message := range this.messages {
		str := fmt.Sprintf(message.Message, message.Args...)
		this.writer.WriteString(fmt.Sprintf("PostmanQ | %v | %s: %s\n", time.Now(), logLevelNames[message.Level], str))
		this.writer.Flush()
	}
}

func (this *Logger) initWriter() {
	if this.writer == nil {
		var output io.Writer = os.Stdout
		if filenameRegex.MatchString(this.Output) {
			dir := filepath.Dir(this.Output)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				FailExit("directory %s is not exists", dir)
			} else {
				var logFile *os.File
				_, err := os.Stat(this.Output);
				if os.IsNotExist(err) {
					logFile, err = os.Create(this.Output)
				} else {
					logFile, err = os.OpenFile(this.Output, os.O_APPEND|os.O_WRONLY, os.ModePerm)
				}
				if logFile != nil {
					output = logFile
				}
				if err != nil {
					FailExit("%v", err)
				}
			}
		}
		this.writer = bufio.NewWriter(output)
	}
}
