package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// Level represents the log level
type Level int

const (
	LevelInfo Level = iota
	LevelError
	LevelDebug
)

// Logger provides thread-safe logging with timestamps
type Logger struct {
	mu     sync.Mutex
	writer io.Writer
	level  Level
	file   *os.File
	prefix string
}

// New creates a new Logger
func New(verbose bool) *Logger {
	level := LevelInfo
	if verbose {
		level = LevelDebug
	}

	return &Logger{
		writer: os.Stdout,
		level:  level,
	}
}

// NewWithFile creates a new Logger that writes to both file and stdout
func NewWithFile(path string, verbose bool) (*Logger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Multi-writer for both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, file)

	level := LevelInfo
	if verbose {
		level = LevelDebug
	}

	return &Logger{
		writer: multiWriter,
		level:  level,
		file:   file,
	}, nil
}

// Close closes the log file if open
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// SetPrefix sets a prefix for log messages
func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

// formatTimestamp returns a formatted timestamp
func (l *Logger) formatTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// log writes a log message with the given level
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level > l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	prefix := l.prefix
	if prefix != "" {
		prefix = "[" + prefix + "] "
	}

	msg := fmt.Sprintf(format, args...)
	log.Printf("[%s] %s%s\n", l.formatTimestamp(), prefix, msg)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// Debug logs a debug message (only when verbose is enabled)
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

// WithPrefix returns a new logger with the given prefix
func (l *Logger) WithPrefix(prefix string) *Logger {
	return &Logger{
		writer: l.writer,
		level:  l.level,
		file:   l.file,
		prefix: prefix,
	}
}

// WithEntity returns a new logger with entity prefix
func (l *Logger) WithEntity(entity string) *Logger {
	return l.WithPrefix(entity)
}

// StdLogger returns a standard library logger
func (l *Logger) StdLogger() *log.Logger {
	return log.New(l.writer, "", 0)
}
