package utils

import (
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarning
	LogLevelError
)

var (
	currentLogLevel LogLevel = LogLevelInfo
	logger          *log.Logger
)

func init() {
	logger = log.New(os.Stdout, "", 0)
	
	// Set log level from environment
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "DEBUG":
		currentLogLevel = LogLevelDebug
	case "INFO":
		currentLogLevel = LogLevelInfo
	case "WARNING", "WARN":
		currentLogLevel = LogLevelWarning
	case "ERROR":
		currentLogLevel = LogLevelError
	default:
		currentLogLevel = LogLevelInfo
	}
}

// Log logs a message with the specified level
func Log(level LogLevel, format string, args ...interface{}) {
	if level < currentLogLevel {
		return
	}
	
	levelStr := []string{"DEBUG", "INFO", "WARNING", "ERROR"}[level]
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	
	logger.Printf("[%s] [%s] %s", timestamp, levelStr, message)
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	Log(LogLevelDebug, format, args...)
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	Log(LogLevelInfo, format, args...)
}

// Warning logs a warning message
func Warning(format string, args ...interface{}) {
	Log(LogLevelWarning, format, args...)
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	Log(LogLevelError, format, args...)
}

// LogError logs an error with context
func LogError(err error, context string) {
	if err == nil {
		return
	}
	
	Error("%s: %v", context, err)
}

// SetLogLevel sets the current log level
func SetLogLevel(level LogLevel) {
	currentLogLevel = level
}


