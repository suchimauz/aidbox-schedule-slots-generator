package out

type LogLevel string

const (
    LogLevelDebug LogLevel = "DEBUG"
    LogLevelInfo  LogLevel = "INFO"
    LogLevelWarn  LogLevel = "WARN"
    LogLevelError LogLevel = "ERROR"
)

type LogFields map[string]interface{}

type LoggerPort interface {
    Debug(event string, fields LogFields)
    Info(event string, fields LogFields)
    Warn(event string, fields LogFields)
    Error(event string, fields LogFields)
    WithFields(fields LogFields) LoggerPort
    WithModule(module string) LoggerPort
}