// Package logger provides a simple, readable leveled logger for the Gumi runtime.
//
// It is intentionally small in Sprint 1: it supports Info, Error, and Debug
// levels and writes human-readable lines to stdout/stderr. Structured logging
// and more advanced formatting will be added as the runtime grows.
package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents a logging severity level.
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	ErrorLevel
)

// Logger is a simple leveled logger.
type Logger struct {
	level  Level
	out    io.Writer
	errOut io.Writer
	mu     sync.Mutex
}

// New creates a logger with the given level string (debug, info, error).
func New(level string) *Logger {
	return &Logger{
		level:  parseLevel(level),
		out:    os.Stdout,
		errOut: os.Stderr,
	}
}

// NewWithWriters creates a logger that writes to custom outputs.
func NewWithWriters(level string, out, errOut io.Writer) *Logger {
	return &Logger{
		level:  parseLevel(level),
		out:    out,
		errOut: errOut,
	}
}

// Info writes an informational message.
func (l *Logger) Info(msg string, pairs ...any) {
	l.log(l.out, "INFO", msg, pairs...)
}

// Error writes an error message.
func (l *Logger) Error(msg string, err error, pairs ...any) {
	args := append([]any{"error", err}, pairs...)
	if err == nil {
		args = pairs
	}
	l.log(l.errOut, "ERROR", msg, args...)
}

// Debug writes a debug message.
func (l *Logger) Debug(msg string, pairs ...any) {
	l.log(l.out, "DEBUG", msg, pairs...)
}

func (l *Logger) log(w io.Writer, level, msg string, pairs ...any) {
	if !l.shouldLog(level) {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fields := ""
	for i := 0; i+1 < len(pairs); i += 2 {
		key := fmt.Sprintf("%v", pairs[i])
		val := fmt.Sprintf("%v", pairs[i+1])
		fields += fmt.Sprintf(" %s=%s", key, val)
	}

	fmt.Fprintf(w, "[%s] %s: %s%s\n", timestamp, level, msg, fields)
}

func (l *Logger) shouldLog(level string) bool {
	switch level {
	case "DEBUG":
		return l.level <= DebugLevel
	case "INFO":
		return l.level <= InfoLevel
	case "ERROR":
		return l.level <= ErrorLevel
	}
	return true
}

func parseLevel(level string) Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "error":
		return ErrorLevel
	default:
		return InfoLevel
	}
}
