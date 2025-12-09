package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel определяет уровень логирования
type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
)

// Logger предоставляет структурированное логирование
type Logger struct {
	level      LogLevel
	enableJSON bool
}

// LogEntry представляет структурированную запись лога
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

var defaultLogger *Logger

func init() {
	defaultLogger = New(INFO, os.Getenv("LOG_FORMAT") == "json")
}

// New создает новый логгер
func New(level LogLevel, enableJSON bool) *Logger {
	return &Logger{
		level:      level,
		enableJSON: enableJSON,
	}
}

// Debug логирует сообщение уровня DEBUG
func Debug(message string, fields map[string]interface{}) {
	defaultLogger.log(DEBUG, message, fields)
}

// Info логирует сообщение уровня INFO
func Info(message string, fields map[string]interface{}) {
	defaultLogger.log(INFO, message, fields)
}

// Warn логирует сообщение уровня WARN
func Warn(message string, fields map[string]interface{}) {
	defaultLogger.log(WARN, message, fields)
}

// Error логирует сообщение уровня ERROR
func Error(message string, fields map[string]interface{}) {
	defaultLogger.log(ERROR, message, fields)
}

// Infof логирует форматированное сообщение уровня INFO
func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...), nil)
}

// Warnf логирует форматированное сообщение уровня WARN
func Warnf(format string, args ...interface{}) {
	Warn(fmt.Sprintf(format, args...), nil)
}

// Errorf логирует форматированное сообщение уровня ERROR
func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...), nil)
}

func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     string(level),
		Message:   message,
		Fields:    fields,
	}

	if l.enableJSON {
		l.logJSON(entry)
	} else {
		l.logText(entry)
	}
}

func (l *Logger) logJSON(entry LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Ошибка сериализации лога: %v", err)
		return
	}
	log.Println(string(data))
}

func (l *Logger) logText(entry LogEntry) {
	fieldsStr := ""
	if len(entry.Fields) > 0 {
		fieldsStr = " "
		for k, v := range entry.Fields {
			fieldsStr += fmt.Sprintf("%s=%v ", k, v)
		}
	}
	log.Printf("[%s] %s%s", entry.Level, entry.Message, fieldsStr)
}

func (l *Logger) shouldLog(level LogLevel) bool {
	levels := map[LogLevel]int{
		DEBUG: 0,
		INFO:  1,
		WARN:  2,
		ERROR: 3,
	}
	return levels[level] >= levels[l.level]
}
