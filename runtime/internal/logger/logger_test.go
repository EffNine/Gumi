package logger

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

var errTest = errors.New("test error")

func TestLoggerInfo(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriters("info", &buf, &buf)

	log.Info("runtime started", "version", "0.1.0")

	out := buf.String()
	if !strings.Contains(out, "INFO: runtime started") {
		t.Errorf("expected info message in output, got %q", out)
	}
	if !strings.Contains(out, "version=0.1.0") {
		t.Errorf("expected field in output, got %q", out)
	}
}

func TestLoggerRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriters("error", &buf, &buf)

	log.Info("should not appear")
	if buf.Len() != 0 {
		t.Errorf("expected no output for error level, got %q", buf.String())
	}
}

func TestLoggerError(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriters("error", &buf, &buf)

	log.Error("startup failed", errTest)

	out := buf.String()
	if !strings.Contains(out, "ERROR: startup failed") {
		t.Errorf("expected error message in output, got %q", out)
	}
	if !strings.Contains(out, "error=test error") {
		t.Errorf("expected error field in output, got %q", out)
	}
}
