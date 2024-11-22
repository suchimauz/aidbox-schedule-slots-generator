package logger

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[37m"
)

type ConsoleLogger struct {
	defaultFields out.LogFields
	module        string
	location      *time.Location
}

func NewConsoleLogger(timezone string) (*ConsoleLogger, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}

	return &ConsoleLogger{
		defaultFields: make(out.LogFields),
		location:      loc,
	}, nil
}

func (l *ConsoleLogger) WithFields(fields out.LogFields) out.LoggerPort {
	newLogger := &ConsoleLogger{
		defaultFields: make(out.LogFields),
		module:        l.module,
		location:      l.location,
	}

	// Копируем существующие поля
	for k, v := range l.defaultFields {
		newLogger.defaultFields[k] = v
	}

	// Добавляем новые поля
	for k, v := range fields {
		newLogger.defaultFields[k] = v
	}

	return newLogger
}

func (l *ConsoleLogger) WithModule(module string) out.LoggerPort {
	return &ConsoleLogger{
		defaultFields: l.defaultFields,
		module:        module,
		location:      l.location,
	}
}

func (l *ConsoleLogger) Debug(event string, fields out.LogFields) {
	l.log(out.LogLevelDebug, event, fields)
}

func (l *ConsoleLogger) Info(event string, fields out.LogFields) {
	l.log(out.LogLevelInfo, event, fields)
}

func (l *ConsoleLogger) Warn(event string, fields out.LogFields) {
	l.log(out.LogLevelWarn, event, fields)
}

func (l *ConsoleLogger) Error(event string, fields out.LogFields) {
	l.log(out.LogLevelError, event, fields)
}

func (l *ConsoleLogger) log(level out.LogLevel, event string, fields out.LogFields) {
	if l.module == "" {
		l.module = "unknown"
	}

	// Объединяем поля
	mergedFields := make(out.LogFields)
	for k, v := range l.defaultFields {
		mergedFields[k] = v
	}
	for k, v := range fields {
		mergedFields[k] = v
	}

	// Добавляем event в поля
	mergedFields["event"] = event

	// Используем таймзону для форматирования времени
	timestamp := time.Now().In(l.location).Format("2006-01-02 15:04:05.000")

	var levelColor, moduleColor string
	switch level {
	case out.LogLevelDebug:
		levelColor = colorGray
		moduleColor = colorCyan
	case out.LogLevelInfo:
		levelColor = colorGreen
		moduleColor = colorCyan
	case out.LogLevelWarn:
		levelColor = colorYellow
		moduleColor = colorCyan
	case out.LogLevelError:
		levelColor = colorRed
		moduleColor = colorCyan
	}

	// Форматируем поля
	fieldsBytes, _ := json.MarshalIndent(mergedFields, "", "  ")

	// Формируем и выводим лог
	logLine := fmt.Sprintf("%s[%s]%s %s[%s]%s %s[%s]%s\n%s",
		colorGray, timestamp, colorReset,
		levelColor, level, colorReset,
		moduleColor, l.module, colorReset,
		string(fieldsBytes),
	)

	fmt.Println(logLine)
}
