package logger

import (
	"os"
)

// автор логов
type Writer interface {
	writeString(string)
	getLevel() Level
}

// автор логов пишущий в стандартный вывод
type StdoutWriter struct{
	// уровень логов, ниже этого уровня логи писаться не будут
	level Level
}

// пишет логи в стандартный вывод
func (s *StdoutWriter) writeString(str string) {
	os.Stdout.WriteString(str)
}

func (s *StdoutWriter) getLevel() Level {
	return s.level
}

// автор логов пишущий в файл
type FileWriter struct {
	filename string
	// уровень логов, ниже этого уровня логи писаться не будут
	level Level
}

// пишет логи в файл
func (f *FileWriter) writeString(str string) {
	file, err := os.OpenFile(f.filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModePerm)
	if err == nil {
		_, err = file.WriteString(str)
		file.Close()
	}
}

func (f *FileWriter) getLevel() Level {
	return f.level
}
