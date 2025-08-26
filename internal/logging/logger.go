package logging

import (
	"fmt"
	"os"
	"strings"
)

// Logger provides structured logging with redaction support
type Logger struct {
	debug   bool
	noColor bool
}

// New creates a new logger instance
func New(debug, noColor bool) *Logger {
	return &Logger{
		debug:   debug,
		noColor: noColor,
	}
}

// Info logs an informational message
func (l *Logger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if !l.noColor {
		fmt.Fprintf(os.Stderr, "\033[32m✓\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(os.Stderr, "✓ %s\n", msg)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if !l.noColor {
		fmt.Fprintf(os.Stderr, "\033[33m⚠\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(os.Stderr, "⚠ %s\n", msg)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if !l.noColor {
		fmt.Fprintf(os.Stderr, "\033[31m✗\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(os.Stderr, "✗ %s\n", msg)
	}
}

// Debug logs a debug message if debug mode is enabled
func (l *Logger) Debug(format string, args ...interface{}) {
	if !l.debug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if !l.noColor {
		fmt.Fprintf(os.Stderr, "\033[36m[DEBUG]\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(os.Stderr, "[DEBUG] %s\n", msg)
	}
}

// Secret represents a value that should be redacted in logs
type Secret string

// String implements the Stringer interface, always returning a redacted value
func (s Secret) String() string {
	return "[REDACTED]"
}

// GoString implements the GoStringer interface for %#v formatting
func (s Secret) GoString() string {
	return "[REDACTED]"
}

// Redact replaces sensitive values in a string with [REDACTED]
func Redact(s string, secrets []string) string {
	result := s
	for _, secret := range secrets {
		if secret != "" && len(secret) > 3 { // Only redact non-trivial secrets
			result = strings.ReplaceAll(result, secret, "[REDACTED]")
		}
	}
	return result
}