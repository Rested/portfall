package logger

import (
	"fmt"
	"github.com/wailsapp/wails"
	l "github.com/wailsapp/wails/lib/logger"
)

// Custom logger implements wails' logging but emits all logs to frontend
type CustomLogger struct {
	prefix    string
	runtime   *wails.Runtime
	errorOnly bool
}

// NewCustomLogger creates a new custom logger with the given prefix
func NewCustomLogger(prefix string, runtime *wails.Runtime) *CustomLogger {
	return &CustomLogger{
		prefix: "[" + prefix + "] ",
		runtime: runtime,
	}
}

type logMessage struct {
	prefix  string
	message string
	fields l.Fields
}

// Info level message
func (c *CustomLogger) Info(message string) {
	l.GlobalLogger.Info(c.prefix + message)
	c.runtime.Events.Emit("info", fmt.Sprintf("%s:%s", c.prefix, message))
}

// Infof - formatted message
func (c *CustomLogger) Infof(message string, args ...interface{}) {
	l.GlobalLogger.Infof(c.prefix+message, args...)
	c.runtime.Events.Emit("log:info", fmt.Sprintf("%s:%s", c.prefix,  fmt.Sprintf(message, args...)))
}

// InfoFields - message with fields
func (c *CustomLogger) InfoFields(message string, fields l.Fields) {
	l.GlobalLogger.WithFields(map[string]interface{}(fields)).Info(c.prefix + message)
	c.runtime.Events.Emit("log:info", fmt.Sprintf("%s:%s", c.prefix, message))
}

// Debug level message
func (c *CustomLogger) Debug(message string) {
	l.GlobalLogger.Debug(c.prefix + message)
	c.runtime.Events.Emit("log:debug", fmt.Sprintf("%s:%s", c.prefix, message))
}

// Debugf - formatted message
func (c *CustomLogger) Debugf(message string, args ...interface{}) {
	l.GlobalLogger.Debugf(c.prefix+message, args...)
	c.runtime.Events.Emit("log:debug", fmt.Sprintf("%s:%s", c.prefix,  fmt.Sprintf(message, args...)))
}

// DebugFields - message with fields
func (c *CustomLogger) DebugFields(message string, fields l.Fields) {
	l.GlobalLogger.WithFields(map[string]interface{}(fields)).Debug(c.prefix + message)
	c.runtime.Events.Emit("log:debug", fmt.Sprintf("%s:%s", c.prefix, message))
}

// Warn level message
func (c *CustomLogger) Warn(message string) {
	l.GlobalLogger.Warn(c.prefix + message)
	c.runtime.Events.Emit("log:warn", fmt.Sprintf("%s:%s", c.prefix, message))
}

// Warnf - formatted message
func (c *CustomLogger) Warnf(message string, args ...interface{}) {
	l.GlobalLogger.Warnf(c.prefix+message, args...)
	c.runtime.Events.Emit("log:warn", fmt.Sprintf("%s:%s", c.prefix,  fmt.Sprintf(message, args...)))
}

// WarnFields - message with fields
func (c *CustomLogger) WarnFields(message string, fields l.Fields) {
	l.GlobalLogger.WithFields(map[string]interface{}(fields)).Warn(c.prefix + message)
	c.runtime.Events.Emit("log:warn", fmt.Sprintf("%s:%s", c.prefix, message))
}

// Error level message
func (c *CustomLogger) Error(message string) {
	l.GlobalLogger.Error(c.prefix + message)
	c.runtime.Events.Emit("log:error", fmt.Sprintf("%s:%s", c.prefix, message))
}

// Errorf - formatted message
func (c *CustomLogger) Errorf(message string, args ...interface{}) {
	l.GlobalLogger.Errorf(c.prefix+message, args...)
	c.runtime.Events.Emit("log:error", fmt.Sprintf("%s:%s", c.prefix,  fmt.Sprintf(message, args...)))
}

// ErrorFields - message with fields
func (c *CustomLogger) ErrorFields(message string, fields l.Fields) {
	l.GlobalLogger.WithFields(map[string]interface{}(fields)).Error(c.prefix + message)
	c.runtime.Events.Emit("log:error", fmt.Sprintf("%s:%s", c.prefix, message))
}

// Fatal level message
func (c *CustomLogger) Fatal(message string) {
	l.GlobalLogger.Fatal(c.prefix + message)
	c.runtime.Events.Emit("log:fatal", fmt.Sprintf("%s:%s", c.prefix, message))
}

// Fatalf - formatted message
func (c *CustomLogger) Fatalf(message string, args ...interface{}) {
	l.GlobalLogger.Fatalf(c.prefix+message, args...)
	c.runtime.Events.Emit("log:fatal", fmt.Sprintf("%s:%s", c.prefix,  fmt.Sprintf(message, args...)))
}

// FatalFields - message with fields
func (c *CustomLogger) FatalFields(message string, fields l.Fields) {
	l.GlobalLogger.WithFields(map[string]interface{}(fields)).Fatal(c.prefix + message)
	c.runtime.Events.Emit("log:fatal", fmt.Sprintf("%s:%s", c.prefix, message))
}