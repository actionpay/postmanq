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
	"runtime/debug"
)

type LogLevel int

const(
	LOG_LEVEL_DEBUG   LogLevel = iota
	LOG_LEVEL_INFO
	LOG_LEVEL_WARNING
	LOG_LEVEL_ERROR
)

const (
	LOG_LEVEL_DEBUG_NAME   = "debug"
	LOG_LEVEL_INFO_NAME    = "info"
	LOG_LEVEL_WARNING_NAME = "warning"
	LOG_LEVEL_ERROR_NAME   = "error"
)

var (
	filenameRegex = regexp.MustCompile(`[^\\/]+\.[^\\/]+`)
	logLevelById = map[LogLevel]string{
		LOG_LEVEL_DEBUG:   LOG_LEVEL_DEBUG_NAME,
		LOG_LEVEL_INFO:    LOG_LEVEL_INFO_NAME,
		LOG_LEVEL_WARNING: LOG_LEVEL_WARNING_NAME,
		LOG_LEVEL_ERROR:   LOG_LEVEL_ERROR_NAME,
	}
	logLevelByName = map[string]LogLevel{
		LOG_LEVEL_DEBUG_NAME   : LOG_LEVEL_DEBUG,
		LOG_LEVEL_INFO_NAME   : LOG_LEVEL_INFO,
		LOG_LEVEL_WARNING_NAME: LOG_LEVEL_WARNING,
		LOG_LEVEL_ERROR_NAME  : LOG_LEVEL_ERROR,
	}
	logger *Logger
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
	Output       string           `yaml:"logOutput"`
	level        LogLevel
	writer       *bufio.Writer
	messages     chan *LogMessage
}

func NewLogger() *Logger {
	if logger == nil {
		logger = new(Logger)
	}
	return logger
}

func (this *Logger) OnRegister() {
	this.level = LOG_LEVEL_WARNING
	this.messages = make(chan *LogMessage)
	this.initWriter()
	go this.start()
}

func (this *Logger) OnInit(event *InitEvent) {
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		if level, ok := logLevelByName[this.LogLevelName]; ok {
			this.level = level
		}
		this.writer = nil
		this.initWriter()
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
	for message := range this.messages {
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

func log(message string, level LogLevel, args ...interface{}) {
	defer func(){recover()}()
	if logger.level <= level {
		if level > LOG_LEVEL_INFO {
			message = fmt.Sprint(message, "\n", string(debug.Stack()))
		}
		logger.messages <- NewLogMessage(level, message, args...)
	}
}

func Err(message string, args ...interface{}) {
	log(message, LOG_LEVEL_ERROR, args...)
}

func FailExit(message string, args ...interface{}) {
	Err(message, args...)
	app.events <- NewApplicationEvent(APPLICATION_EVENT_KIND_FINISH)
}

func FailExitWithErr(err error) {
	FailExit("%v", err)
}

func Warn(message string, args ...interface{}) {
	log(message, LOG_LEVEL_WARNING, args...)
}

func WarnWithErr(err error) {
	Warn("%v", err)
}

func Info(message string, args ...interface{}) {
	log(message, LOG_LEVEL_INFO, args...)
}

func Debug(message string, args ...interface{}) {
	log(message, LOG_LEVEL_DEBUG, args...)
}
