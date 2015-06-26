package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"github.com/AdOnWeb/postmanq/common"
)

type Writer interface {
	writeString(string)
}

type StdoutWriter struct{}

func (this *StdoutWriter) writeString(str string) {
	os.Stdout.WriteString(str)
}

type FileWriter struct {
	filename string
}

func (this *FileWriter) writeString(str string) {
	f, err := os.OpenFile(this.filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModePerm)
	if err == nil {
		_, err = f.WriteString(str)
		f.Close()
	}
}

type Writers []Writer

func (w Writers) len() int {
	return len(w)
}

func (w *Writers) init(service *Service) {
	for i := 0; i < w.len(); i++ {
		if common.FilenameRegex.MatchString(service.Output) { // проверяем получили ли из настроек имя файла
			// получаем директорию, в которой лежит файл
			dir := filepath.Dir(service.Output)
			// смотрим, что она реально существует
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				FailExit("directory %s is not exists", dir)
			} else {
				w.add(i, &FileWriter{service.Output})
			}
		} else if len(service.Output) == 0 || service.Output == "stdout" {
			w.add(i, &StdoutWriter{})
		}
	}
}

func (w *Writers) add(i int, writer Writer) {
	(*w)[i] = writer
}

func (w Writers) write() {
	for _, writer := range w {
		go w.listenMessages(writer)
	}
}

func (w *Writers) listenMessages(writer Writer) {
	for message := range messages {
		w.writeMessage(writer, message)
	}
}

func (w *Writers) writeMessage(writer Writer, message *Message) {
	writer.writeString(
		fmt.Sprintf(
			"PostmanQ | %v | %s: %s\n",
			time.Now().Format("2006-01-02 15:04:05"),
			logLevelById[message.Level],
			fmt.Sprintf(message.Message, message.Args...),
		),
	)
}
