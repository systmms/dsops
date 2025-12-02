package testutil

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLogger captures log output for validation in tests.
//
// This logger redirects all logging output to an in-memory buffer,
// allowing tests to verify that secrets are properly redacted and
// that expected log messages are produced.
//
// Example usage:
//
//	logger := NewTestLogger(t)
//	logger.Info("Processing secret: %s", logging.Secret("password123"))
//
//	output := logger.GetOutput()
//	logger.AssertContains(t, "[REDACTED]")
//	logger.AssertNotContains(t, "password123")
type TestLogger struct {
	buffer *bytes.Buffer
	debug  bool
	mu     sync.Mutex
}

// NewTestLogger creates a new TestLogger with default settings.
//
// Debug mode is disabled by default. Use NewTestLoggerWithDebug
// if you need to capture debug messages.
func NewTestLogger(t *testing.T) *TestLogger {
	t.Helper()

	return &TestLogger{
		buffer: &bytes.Buffer{},
		debug:  false,
	}
}

// NewTestLoggerWithDebug creates a new TestLogger with debug mode enabled.
//
// When debug is true, Debug() method calls will be captured in the buffer.
func NewTestLoggerWithDebug(t *testing.T, debug bool) *TestLogger {
	t.Helper()

	return &TestLogger{
		buffer: &bytes.Buffer{},
		debug:  debug,
	}
}

// Info logs an informational message.
func (l *TestLogger) Info(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.buffer, "✓ %s\n", msg)
}

// Warn logs a warning message.
func (l *TestLogger) Warn(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.buffer, "⚠ %s\n", msg)
}

// Error logs an error message.
func (l *TestLogger) Error(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.buffer, "✗ %s\n", msg)
}

// Debug logs a debug message if debug mode is enabled.
func (l *TestLogger) Debug(format string, args ...interface{}) {
	if !l.debug {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.buffer, "[DEBUG] %s\n", msg)
}

// Capture executes a function and captures its log output.
//
// This is useful for testing functions that log internally.
//
// Example:
//
//	output := logger.Capture(func() {
//	    someFunction()
//	})
func (l *TestLogger) Capture(fn func()) string {
	l.Clear() // Start with clean buffer
	fn()
	return l.GetOutput()
}

// GetOutput returns the captured log output as a string.
//
// The output includes all log messages captured since the logger
// was created or since the last Clear() call.
func (l *TestLogger) GetOutput() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.buffer.String()
}

// Clear clears the captured log output.
//
// This is useful when reusing the same logger across multiple test cases.
func (l *TestLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.buffer.Reset()
}

// AssertContains asserts that the log output contains the specified substring.
//
// This is a convenience wrapper around testify's Contains assertion.
func (l *TestLogger) AssertContains(t *testing.T, substr string) {
	t.Helper()

	output := l.GetOutput()
	assert.Contains(t, output, substr, "Expected log output to contain %q", substr)
}

// AssertNotContains asserts that the log output does NOT contain the specified substring.
//
// This is particularly useful for verifying that secrets are redacted.
func (l *TestLogger) AssertNotContains(t *testing.T, substr string) {
	t.Helper()

	output := l.GetOutput()
	assert.NotContains(t, output, substr, "Expected log output to NOT contain %q", substr)
}

// AssertRedacted asserts that a secret value is redacted in the log output.
//
// This checks that:
// 1. The secret value itself does NOT appear in logs
// 2. The [REDACTED] marker DOES appear in logs
//
// This is the primary assertion for security tests.
func (l *TestLogger) AssertRedacted(t *testing.T, secretValue string) {
	t.Helper()

	output := l.GetOutput()

	// Secret value must not appear
	assert.NotContains(t, output, secretValue,
		"Secret value %q should be redacted, but appears in logs", secretValue)

	// [REDACTED] marker should appear
	assert.Contains(t, output, "[REDACTED]",
		"Expected [REDACTED] marker in logs when secret is used")
}

// AssertLogCount asserts that a specific log level appears a certain number of times.
//
// Level markers:
//   - Info: "✓"
//   - Warn: "⚠"
//   - Error: "✗"
//   - Debug: "[DEBUG]"
func (l *TestLogger) AssertLogCount(t *testing.T, level string, count int) {
	t.Helper()

	output := l.GetOutput()

	var marker string
	switch level {
	case "info":
		marker = "✓"
	case "warn":
		marker = "⚠"
	case "error":
		marker = "✗"
	case "debug":
		marker = "[DEBUG]"
	default:
		t.Fatalf("Unknown log level: %s", level)
	}

	actual := strings.Count(output, marker)
	assert.Equal(t, count, actual,
		"Expected %d %s log messages, got %d", count, level, actual)
}

// AssertEmpty asserts that no log output was captured.
//
// Useful for verifying that quiet operations produce no logs.
func (l *TestLogger) AssertEmpty(t *testing.T) {
	t.Helper()

	output := l.GetOutput()
	assert.Empty(t, output, "Expected no log output, but got:\n%s", output)
}

// Lines returns the log output split into individual lines.
//
// Empty lines are filtered out. Useful for line-by-line validation.
func (l *TestLogger) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	output := l.buffer.String()
	lines := strings.Split(output, "\n")

	// Filter empty lines
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}

	return result
}
