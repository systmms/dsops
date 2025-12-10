package logging_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systmms/dsops/internal/logging"
)

// captureStderr captures stderr output for testing
func captureStderr(fn func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// TestSecretRedactionAtInfoLevel verifies secrets are redacted in Info-level logs
func TestSecretRedactionAtInfoLevel(t *testing.T) {
	// Note: Cannot use t.Parallel() because captureStderr() modifies global os.Stderr

	logger := logging.New(false, true) // no debug, no color

	secretValue := "super-secret-password-12345"
	secret := logging.Secret(secretValue)

	output := captureStderr(func() {
		logger.Info("Retrieved secret: %s", secret)
	})

	assert.Contains(t, output, "[REDACTED]", "Log should contain redaction marker")
	assert.NotContains(t, output, secretValue, "Log must not contain actual secret value")
	assert.Contains(t, output, "Retrieved secret", "Log should contain message text")
}

// TestSecretRedactionAtDebugLevel verifies secrets are redacted in Debug-level logs
func TestSecretRedactionAtDebugLevel(t *testing.T) {
	// Note: Cannot use t.Parallel() because captureStderr() modifies global os.Stderr

	logger := logging.New(true, true) // debug enabled, no color

	secretValue := "debug-secret-api-key-67890"
	secret := logging.Secret(secretValue)

	output := captureStderr(func() {
		logger.Debug("Processing secret: %s", secret)
	})

	assert.Contains(t, output, "[REDACTED]", "Debug log should contain redaction marker")
	assert.NotContains(t, output, secretValue, "Debug log must not contain actual secret value")
	assert.Contains(t, output, "[DEBUG]", "Should indicate debug level")
}

// TestMultipleSecretsRedaction verifies multiple secrets in same log are all redacted
func TestMultipleSecretsRedaction(t *testing.T) {
	// Note: Cannot use t.Parallel() because captureStderr() modifies global os.Stderr

	logger := logging.New(false, true)

	secret1 := "password-123"
	secret2 := "api-key-456"
	secret3 := "token-789"

	output := captureStderr(func() {
		logger.Info("Credentials: password=%s, api_key=%s, token=%s",
			logging.Secret(secret1),
			logging.Secret(secret2),
			logging.Secret(secret3))
	})

	// All three secrets should be redacted
	redactedCount := strings.Count(output, "[REDACTED]")
	assert.Equal(t, 3, redactedCount, "All three secrets should be redacted")

	// None of the actual secrets should appear
	assert.NotContains(t, output, secret1)
	assert.NotContains(t, output, secret2)
	assert.NotContains(t, output, secret3)
}

// TestSecretRedactionInErrorMessages verifies secrets are redacted in error contexts
func TestSecretRedactionInErrorMessages(t *testing.T) {
	// Note: Cannot use t.Parallel() because captureStderr() modifies global os.Stderr

	logger := logging.New(false, true)

	secretValue := "error-context-secret-999"
	secret := logging.Secret(secretValue)

	output := captureStderr(func() {
		logger.Error("Authentication failed for secret: %s", secret)
	})

	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, secretValue)
	assert.Contains(t, output, "Authentication failed")
}

// TestSecretRedactionWithFormatting verifies secrets are redacted regardless of formatting
func TestSecretRedactionWithFormatting(t *testing.T) {
	// Note: Cannot use t.Parallel() because subtests use captureStderr() which modifies global os.Stderr

	tests := []struct {
		name       string
		secret     string
		formatStr  string
		formatArgs []interface{}
	}{
		{
			name:       "string_format",
			secret:     "secret-string-format",
			formatStr:  "Value: %s",
			formatArgs: []interface{}{logging.Secret("secret-string-format")},
		},
		{
			name:       "quoted_format",
			secret:     "secret-quoted",
			formatStr:  "Value: '%s'",
			formatArgs: []interface{}{logging.Secret("secret-quoted")},
		},
		{
			name:       "json_like_format",
			secret:     "secret-json",
			formatStr:  `{"key": "%s"}`,
			formatArgs: []interface{}{logging.Secret("secret-json")},
		},
		{
			name:       "multiple_placeholders",
			secret:     "secret-multi",
			formatStr:  "First: %s, Second: %s",
			formatArgs: []interface{}{"public", logging.Secret("secret-multi")},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// Note: Cannot use t.Parallel() because captureStderr() modifies global os.Stderr

			logger := logging.New(false, true)

			output := captureStderr(func() {
				logger.Info(tt.formatStr, tt.formatArgs...)
			})

			assert.Contains(t, output, "[REDACTED]")
			assert.NotContains(t, output, tt.secret)
		})
	}
}

// TestSecretTypeString verifies Secret type's String() method returns redaction
func TestSecretTypeString(t *testing.T) {
	t.Parallel()

	secretValue := "test-secret-value"
	secret := logging.Secret(secretValue)

	stringified := secret.String()

	assert.Equal(t, "[REDACTED]", stringified, "Secret.String() should return redaction marker")
	assert.NotContains(t, stringified, secretValue, "Secret.String() must not return actual value")
}

// TestSecretGoString verifies Secret type's GoString() method returns redaction
func TestSecretGoString(t *testing.T) {
	t.Parallel()

	secretValue := "test-gostring-secret"
	secret := logging.Secret(secretValue)

	goStringified := secret.GoString()

	assert.Equal(t, "[REDACTED]", goStringified, "Secret.GoString() should return redaction marker")
	assert.NotContains(t, goStringified, secretValue, "Secret.GoString() must not return actual value")
}

// TestSecretRedactionAcrossLogLevels verifies redaction works at all log levels
func TestSecretRedactionAcrossLogLevels(t *testing.T) {
	// Note: Cannot use t.Parallel() because subtests use captureStderr() which modifies global os.Stderr

	secretValue := "multi-level-secret-abc"

	levels := []struct {
		name  string
		debug bool
		logFn func(*logging.Logger, string, ...interface{})
	}{
		{"info", false, (*logging.Logger).Info},
		{"warn", false, (*logging.Logger).Warn},
		{"error", false, (*logging.Logger).Error},
		{"debug", true, (*logging.Logger).Debug},
	}

	for _, tt := range levels {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// Note: Cannot use t.Parallel() because captureStderr() modifies global os.Stderr

			logger := logging.New(tt.debug, true)

			output := captureStderr(func() {
				tt.logFn(logger, "Secret: %s", logging.Secret(secretValue))
			})

			if output != "" { // Debug only logs if debug enabled
				assert.Contains(t, output, "[REDACTED]")
				assert.NotContains(t, output, secretValue)
			}
		})
	}
}

// TestEmptySecretRedaction verifies empty secrets are handled correctly
func TestEmptySecretRedaction(t *testing.T) {
	// Note: Cannot use t.Parallel() because captureStderr() modifies global os.Stderr

	logger := logging.New(false, true)

	emptySecret := logging.Secret("")

	output := captureStderr(func() {
		logger.Info("Empty secret: %s", emptySecret)
	})

	assert.Contains(t, output, "[REDACTED]", "Even empty secrets should be redacted")
}

// TestSecretRedactionWithNonSecretData verifies non-secret data is not redacted
func TestSecretRedactionWithNonSecretData(t *testing.T) {
	// Note: Cannot use t.Parallel() because captureStderr() modifies global os.Stderr

	logger := logging.New(false, true)

	publicValue := "public-information"
	secretValue := "private-secret-123"

	output := captureStderr(func() {
		logger.Info("Public: %s, Secret: %s", publicValue, logging.Secret(secretValue))
	})

	// Public value should appear as-is
	assert.Contains(t, output, publicValue, "Public information should not be redacted")

	// Secret should be redacted
	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, secretValue)
}

// TestRedactFunction verifies the Redact helper function
func TestRedactFunction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		secrets  []string
		expected string
	}{
		{
			name:     "single_secret",
			input:    "password is secret123",
			secrets:  []string{"secret123"},
			expected: "password is [REDACTED]",
		},
		{
			name:     "multiple_secrets",
			input:    "user:admin password:secret123 token:xyz789",
			secrets:  []string{"admin", "secret123", "xyz789"},
			expected: "user:[REDACTED] password:[REDACTED] token:[REDACTED]",
		},
		{
			name:     "no_secrets",
			input:    "public information",
			secrets:  []string{},
			expected: "public information",
		},
		{
			name:     "short_secrets_not_redacted",
			input:    "value is abc",
			secrets:  []string{"abc"}, // Too short (len <= 3)
			expected: "value is abc",
		},
		{
			name:     "empty_secret_ignored",
			input:    "value is test",
			secrets:  []string{""},
			expected: "value is test",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := logging.Redact(tt.input, tt.secrets)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestColorOutputDisabled verifies logs work correctly without color
func TestColorOutputDisabled(t *testing.T) {
	// Note: Cannot use t.Parallel() because captureStderr() modifies global os.Stderr

	logger := logging.New(false, true) // noColor = true

	output := captureStderr(func() {
		logger.Info("Test message")
	})

	// Should not contain ANSI color codes
	assert.NotContains(t, output, "\033[", "Should not contain ANSI codes when color disabled")
	assert.Contains(t, output, "✓", "Should contain checkmark")
}

// TestDebugModeDisabled verifies debug logs don't appear when debug is off
func TestDebugModeDisabled(t *testing.T) {
	// Note: Cannot use t.Parallel() because captureStderr() modifies global os.Stderr

	logger := logging.New(false, true) // debug = false

	output := captureStderr(func() {
		logger.Debug("This should not appear")
	})

	assert.Empty(t, output, "Debug message should not appear when debug is disabled")
}

// TestDebugModeEnabled verifies debug logs appear when debug is on
func TestDebugModeEnabled(t *testing.T) {
	// Note: Cannot use t.Parallel() because captureStderr() modifies global os.Stderr

	logger := logging.New(true, true) // debug = true

	output := captureStderr(func() {
		logger.Debug("This should appear")
	})

	assert.Contains(t, output, "[DEBUG]", "Debug message should appear when debug is enabled")
	assert.Contains(t, output, "This should appear")
}
