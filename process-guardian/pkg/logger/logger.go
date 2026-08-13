package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

var levelNames = map[LogLevel]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
	LevelFatal: "FATAL",
}

type Logger struct {
	mu       sync.Mutex
	out      io.Writer
	level    LogLevel
	prefix   string
	showTime bool
}

var defaultLogger *Logger

func init() {
	defaultLogger = NewLogger(os.Stdout, LevelInfo, "[guardian]", true)
}

func NewLogger(out io.Writer, level LogLevel, prefix string, showTime bool) *Logger {
	return &Logger{
		out:      out,
		level:    level,
		prefix:   prefix,
		showTime: showTime,
	}
}

func Default() *Logger {
	return defaultLogger
}

func SetLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}

func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	var timeStr string
	if l.showTime {
		timeStr = time.Now().Format("2006-01-02 15:04:05.000")
	}

	msg := fmt.Sprintf(format, args...)
	levelName := levelNames[level]

	if l.showTime {
		fmt.Fprintf(l.out, "%s [%s] %s: %s\n", timeStr, levelName, l.prefix, msg)
	} else {
		fmt.Fprintf(l.out, "[%s] %s: %s\n", levelName, l.prefix, msg)
	}
}

func Debug(format string, args ...interface{}) {
	defaultLogger.log(LevelDebug, format, args...)
}

func Info(format string, args ...interface{}) {
	defaultLogger.log(LevelInfo, format, args...)
}

func Warn(format string, args ...interface{}) {
	defaultLogger.log(LevelWarn, format, args...)
}

func Error(format string, args ...interface{}) {
	defaultLogger.log(LevelError, format, args...)
}

func Fatal(format string, args ...interface{}) {
	defaultLogger.log(LevelFatal, format, args...)
	os.Exit(1)
}

func Debugf(format string, args ...interface{}) {
	defaultLogger.log(LevelDebug, format, args...)
}

func Infof(format string, args ...interface{}) {
	defaultLogger.log(LevelInfo, format, args...)
}

func Warnf(format string, args ...interface{}) {
	defaultLogger.log(LevelWarn, format, args...)
}

func Errorf(format string, args ...interface{}) {
	defaultLogger.log(LevelError, format, args...)
}

func Fatalf(format string, args ...interface{}) {
	defaultLogger.log(LevelFatal, format, args...)
	os.Exit(1)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(LevelFatal, format, args...)
	os.Exit(1)
}