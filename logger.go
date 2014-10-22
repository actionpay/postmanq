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
	LOG_LEVEL_INFO    LogLevel = iota
	LOG_LEVEL_WARNING
	LOG_LEVEL_ERROR
)

const (
	LOG_LEVEL_INFO_NAME    = "info"
	LOG_LEVEL_WARNING_NAME = "warning"
	LOG_LEVEL_ERROR_NAME   = "error"
)

var (
	filenameRegex = regexp.MustCompile(`[^\\/]+\.[^\\/]+`)
	logLevelById = map[LogLevel]string{
		LOG_LEVEL_INFO:    LOG_LEVEL_INFO_NAME,
		LOG_LEVEL_WARNING: LOG_LEVEL_WARNING_NAME,
		LOG_LEVEL_ERROR:   LOG_LEVEL_ERROR_NAME,
	}
	logLevelByName = map[string]LogLevel{
		LOG_LEVEL_INFO_NAME   : LOG_LEVEL_INFO,
		LOG_LEVEL_WARNING_NAME: LOG_LEVEL_WARNING,
		LOG_LEVEL_ERROR_NAME  : LOG_LEVEL_ERROR,
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
	LogLevelName string           `yaml:"logLevel"`
	level        LogLevel
	Output       string           `yaml:"output"`
	writer       *bufio.Writer
	messages     chan *LogMessage
}

func NewLogger() *Logger {
	logger := new(Logger)
	return logger
}

func (this *Logger) OnRegister(event *RegisterEvent) {
	this.level = LOG_LEVEL_ERROR
	this.messages = make(chan *LogMessage)
	this.initWriter()
	go this.start()
	event.LogChan = this.messages
	event.Group.Done()
}

func (this *Logger) OnInit(event *InitEvent) {
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		if level, ok := logLevelByName[this.LogLevelName]; ok {
			this.level = level
		}
		this.writer = nil
		this.initWriter()
		event.Group.Done()
	} else {
		FailExitWithErr(err)
	}
}

func (this *Logger) OnRun() {}

func (this *Logger) OnFinish(event *FinishEvent) {
	close(this.messages)
	event.Group.Done()
}

func (this *Logger) start() {
	for {
		select {
		case message, ok := <- this.messages:
			if ok && this.level <= message.Level {
				this.writer.WriteString(
					fmt.Sprintf(
						"PostmanQ | %v | %s: %s\n",
						time.Now().Format("2006-01-02 15:04:05"),
						logLevelById[message.Level],
						fmt.Sprintf(message.Message, message.Args...),
					),
				)
				this.writer.Flush()
			}
		}
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
					FailExitWithErr(err)
				}
			}
		}
		this.writer = bufio.NewWriter(output)
	}
}
