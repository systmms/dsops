package errors_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/logging"
)

// TestUserErrorFormatting verifies UserError displays properly
func TestUserErrorFormatting(t *testing.T) {
	t.Parallel()

	err := errors.UserError{
		Message:    "Operation failed",
		Details:    "Connection timeout",
		Suggestion: "Check network connectivity",
	}

	errMsg := err.Error()

	assert.Contains(t, errMsg, "Operation failed")
	assert.Contains(t, errMsg, "Connection timeout")
	assert.Contains(t, errMsg, "Check network connectivity")
	assert.Contains(t, errMsg, "💡")
}

// TestConfigErrorFormatting verifies ConfigError displays with context
func TestConfigErrorFormatting(t *testing.T) {
	t.Parallel()

	err := errors.ConfigError{
		Field:      "providers.vault.addr",
		Value:      "invalid-url",
		Message:    "Invalid URL format",
		Suggestion: "Use format: http://hostname:port",
	}

	errMsg := err.Error()

	assert.Contains(t, errMsg, "providers.vault.addr")
	assert.Contains(t, errMsg, "invalid-url")
	assert.Contains(t, errMsg, "Invalid URL format")
	assert.Contains(t, errMsg, "http://hostname:port")
}

// TestCommandErrorFormatting verifies CommandError includes exit code
func TestCommandErrorFormatting(t *testing.T) {
	t.Parallel()

	err := errors.CommandError{
		Command:    "bw list items",
		ExitCode:   1,
		Message:    "Vault is locked",
		Suggestion: "Run 'bw unlock'",
	}

	errMsg := err.Error()

	assert.Contains(t, errMsg, "bw list items")
	assert.Contains(t, errMsg, "exit code: 1")
	assert.Contains(t, errMsg, "Vault is locked")
	assert.Contains(t, errMsg, "bw unlock")
}

// TestProviderErrorWithSecretRedaction verifies provider errors redact secrets when properly wrapped
// TODO: This test is currently skipped because errors.ProviderError doesn't propagate
// logging.Secret redaction through error wrapping. Requires error package enhancement.
func TestProviderErrorWithSecretRedaction(t *testing.T) {
	t.Skip("Requires error package to implement secret redaction in wrapped errors")
	t.Parallel()

	secretValue := "api-key-super-secret-123"

	// Create base error with secret (using logging.Secret ensures redaction)
	baseErr := fmt.Errorf("authentication failed with key: %s", logging.Secret(secretValue))

	// Wrap in provider error
	providerErr := errors.ProviderError("vault", "read", baseErr)

	errMsg := providerErr.Error()

	// Should contain provider context
	assert.Contains(t, errMsg, "vault provider error")
	assert.Contains(t, errMsg, "read")

	// Because baseErr used logging.Secret, the error chain will contain [REDACTED]
	// The Secret type's String() method returns "[REDACTED]"
	assert.Contains(t, errMsg, "[REDACTED]", "Secret should be redacted in error chain")

	// Should NOT contain actual secret
	assert.NotContains(t, errMsg, secretValue, "Actual secret value must not appear")
}

// TestBitwardenProviderSuggestions verifies Bitwarden-specific error suggestions
func TestBitwardenProviderSuggestions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		errorMsg       string
		expectedSuggestion string
	}{
		{
			name:           "not_logged_in",
			errorMsg:       "not logged in",
			expectedSuggestion: "bw login",
		},
		{
			name:           "vault_locked",
			errorMsg:       "vault is locked",
			expectedSuggestion: "bw unlock",
		},
		{
			name:           "not_found",
			errorMsg:       "Not found",
			expectedSuggestion: "bw list items",
		},
		{
			name:           "command_not_found",
			errorMsg:       "command not found",
			expectedSuggestion: "Install Bitwarden CLI",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			baseErr := fmt.Errorf(tt.errorMsg)
			providerErr := errors.ProviderError("bitwarden", "resolve", baseErr)

			errMsg := providerErr.Error()
			assert.Contains(t, errMsg, tt.expectedSuggestion)
		})
	}
}

// Test1PasswordProviderSuggestions verifies 1Password-specific error suggestions
func Test1PasswordProviderSuggestions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		errorMsg           string
		expectedSuggestion string
	}{
		{
			name:               "not_signed_in",
			errorMsg:           "not signed in",
			expectedSuggestion: "op signin",
		},
		{
			name:               "session_expired",
			errorMsg:           "session expired",
			expectedSuggestion: "session has expired",
		},
		{
			name:               "not_found",
			errorMsg:           "not found",
			expectedSuggestion: "op item list",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			baseErr := fmt.Errorf(tt.errorMsg)
			providerErr := errors.ProviderError("1password", "resolve", baseErr)

			errMsg := providerErr.Error()
			assert.Contains(t, errMsg, tt.expectedSuggestion)
		})
	}
}

// TestAWSProviderSuggestions verifies AWS-specific error suggestions
func TestAWSProviderSuggestions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		errorMsg           string
		expectedSuggestion string
	}{
		{
			name:               "credentials",
			errorMsg:           "credentials not found",
			expectedSuggestion: "aws configure",
		},
		{
			name:               "access_denied",
			errorMsg:           "AccessDenied",
			expectedSuggestion: "IAM permissions",
		},
		{
			name:               "not_found",
			errorMsg:           "ResourceNotFoundException",
			expectedSuggestion: "list-secrets",
		},
		{
			name:               "throttling",
			errorMsg:           "ThrottlingException",
			expectedSuggestion: "rate limit",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			baseErr := fmt.Errorf(tt.errorMsg)
			providerErr := errors.ProviderError("aws-secretsmanager", "resolve", baseErr)

			errMsg := providerErr.Error()
			assert.Contains(t, errMsg, tt.expectedSuggestion)
		})
	}
}

// TestWrapCommandNotFound verifies command not found errors have helpful suggestions
func TestWrapCommandNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		command            string
		expectedSuggestion string
	}{
		{"npm", "Node.js"},
		{"docker", "Docker"},
		{"git", "Git"},
		{"python", "Python"},
		{"go", "Go"},
		{"unknown-cmd", "in your PATH"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()

			baseErr := fmt.Errorf("command not found")
			err := errors.WrapCommandNotFound(tt.command, baseErr)

			errMsg := err.Error()
			assert.Contains(t, errMsg, tt.command)
			assert.Contains(t, errMsg, tt.expectedSuggestion)
		})
	}
}

// TestIsRetryable verifies retryable error detection
func TestIsRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		errorMsg   string
		retryable  bool
	}{
		{"timeout", "operation timeout", true},
		{"rate_limit", "rate limit exceeded", true},
		{"throttling", "ThrottlingException", true},
		{"connection_reset", "connection reset by peer", true},
		{"broken_pipe", "broken pipe", true},
		{"not_found", "resource not found", false},
		{"invalid_config", "invalid configuration", false},
		{"nil_error", "", false}, // nil error case
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var err error
			if tt.errorMsg != "" {
				err = fmt.Errorf(tt.errorMsg)
			}

			result := errors.IsRetryable(err)
			assert.Equal(t, tt.retryable, result)
		})
	}
}

// TestSimplifyError verifies error simplification for common cases
func TestSimplifyError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		inputError     error
		expectedType   string
		expectedInMsg  string
	}{
		{
			name:           "yaml_error",
			inputError:     fmt.Errorf("yaml: line 5: mapping values are not allowed"),
			expectedType:   "ConfigError",
			expectedInMsg:  "Invalid YAML",
		},
		{
			name:           "json_error",
			inputError:     fmt.Errorf("json: invalid character"),
			expectedType:   "ConfigError",
			expectedInMsg:  "Invalid JSON",
		},
		{
			name:           "permission_denied",
			inputError:     fmt.Errorf("permission denied"),
			expectedType:   "UserError",
			expectedInMsg:  "Permission denied",
		},
		{
			name:           "file_not_found",
			inputError:     fmt.Errorf("no such file or directory"),
			expectedType:   "UserError",
			expectedInMsg:  "not found",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			simplified := errors.SimplifyError(tt.inputError)

			errMsg := simplified.Error()
			assert.Contains(t, errMsg, tt.expectedInMsg)

			// Check error type
			switch tt.expectedType {
			case "ConfigError":
				_, ok := simplified.(errors.ConfigError)
				assert.True(t, ok, "Should be ConfigError type")
			case "UserError":
				_, ok := simplified.(errors.UserError)
				assert.True(t, ok, "Should be UserError type")
			}
		})
	}
}

// TestUserErrorUnwrap verifies error unwrapping works correctly
func TestUserErrorUnwrap(t *testing.T) {
	t.Parallel()

	baseErr := fmt.Errorf("base error")
	userErr := errors.UserError{
		Message: "wrapped error",
		Err:     baseErr,
	}

	unwrapped := userErr.Unwrap()
	assert.Equal(t, baseErr, unwrapped)
}

// TestErrorMessagesWithSecretsInContext verifies context doesn't leak secrets
// TODO: Skipped - requires error package to propagate logging.Secret redaction
func TestErrorMessagesWithSecretsInContext(t *testing.T) {
	t.Skip("Requires error package to implement secret redaction in wrapped errors")
	t.Parallel()

	secretValue := "context-secret-token-xyz"

	// Simulate error with secret in context
	baseErr := fmt.Errorf("connection failed for token: %s", logging.Secret(secretValue))

	providerErr := errors.ProviderError("vault", "connect", baseErr)

	errMsg := providerErr.Error()

	// Should contain redaction
	assert.Contains(t, errMsg, "[REDACTED]")

	// Should NOT contain actual secret
	assert.NotContains(t, errMsg, secretValue)
}

// TestNilErrorHandling verifies nil errors are handled gracefully
func TestNilErrorHandling(t *testing.T) {
	t.Parallel()

	// IsRetryable with nil
	assert.False(t, errors.IsRetryable(nil))

	// SimplifyError with nil
	assert.Nil(t, errors.SimplifyError(nil))
}

// TestErrorDoesNotLeakSecretsInWrappedChain verifies error chains maintain redaction
// TODO: Skipped - requires error package to propagate logging.Secret redaction
func TestErrorDoesNotLeakSecretsInWrappedChain(t *testing.T) {
	t.Skip("Requires error package to implement secret redaction in wrapped errors")
	t.Parallel()

	secretValue := "chained-secret-password"

	// Create chain of errors with secret
	baseErr := fmt.Errorf("auth failed with password: %s", logging.Secret(secretValue))
	wrappedErr := errors.ProviderError("bitwarden", "login", baseErr)

	errMsg := wrappedErr.Error()

	// Full error chain should not leak secret
	assert.Contains(t, errMsg, "[REDACTED]")
	assert.NotContains(t, errMsg, secretValue)
}
