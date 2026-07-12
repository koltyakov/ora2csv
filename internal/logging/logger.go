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
	mu     *sync.Mutex
	stdout io.Writer
	stderr io.Writer
	level  Level
	file   *os.File
	prefix string
	now    func() time.Time
}

// New creates a new Logger
func New(verbose bool) *Logger {
	level := LevelInfo
	if verbose {
		level = LevelDebug
	}
	return &Logger{
		mu:     &sync.Mutex{},
		stdout: os.Stdout,
		stderr: os.Stderr,
		level:  level,
		now:    time.Now,
	}
}

// NewWithFile creates a new Logger that writes to both file and stdout
func NewWithFile(path string, verbose bool) (*Logger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	level := LevelInfo
	if verbose {
		level = LevelDebug
	}

	return &Logger{
		mu:     &sync.Mutex{},
		stdout: io.MultiWriter(os.Stdout, file),
		stderr: io.MultiWriter(os.Stderr, file),
		level:  level,
		file:   file,
		now:    time.Now,
	}, nil
}

// Close closes the log file if open
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
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
	return l.now().UTC().Format(time.RFC3339)
}

// log writes a log message with the given level
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level == LevelDebug && l.level != LevelDebug {
		return
	}
	if level == LevelInfo && l.level == LevelError {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	prefix := l.prefix
	if prefix != "" {
		prefix = "[" + prefix + "] "
	}

	msg := fmt.Sprintf(format, args...)
	writer := l.stdout
	if level == LevelError {
		writer = l.stderr
	}
	fmt.Fprintf(writer, "[%s] [%s] %s%s\n", l.formatTimestamp(), level.String(), prefix, msg)
}

func (l Level) String() string {
	switch l {
	case LevelError:
		return "ERROR"
	case LevelDebug:
		return "DEBUG"
	default:
		return "INFO"
	}
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
		mu:     l.mu,
		stdout: l.stdout,
		stderr: l.stderr,
		level:  l.level,
		file:   l.file,
		prefix: prefix,
		now:    l.now,
	}
}

// WithEntity returns a new logger with entity prefix
func (l *Logger) WithEntity(entity string) *Logger {
	return l.WithPrefix(entity)
}

// StdLogger returns a standard library logger
func (l *Logger) StdLogger() *log.Logger {
	return log.New(l.stdout, "", 0)
}
