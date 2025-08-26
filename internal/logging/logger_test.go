package logging

import (
	"testing"
)

func TestSecretRedaction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "secret is redacted",
			input:    "my-secret-password",
			expected: "[REDACTED]",
		},
		{
			name:     "empty secret is still redacted",
			input:    "",
			expected: "[REDACTED]",
		},
		{
			name:     "complex secret is redacted",
			input:    "password123!@#",
			expected: "[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Secret(tt.input).String()
			if result != tt.expected {
				t.Errorf("Secret(%q).String() = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoggerSecretRedaction(t *testing.T) {
	// Test that secrets are properly redacted when logged
	secret := "super-secret-password"
	redactedValue := Secret(secret).String()
	
	if redactedValue != "[REDACTED]" {
		t.Errorf("Expected [REDACTED], got %s", redactedValue)
	}
	
	// Test GoString interface for %#v formatting
	goStringValue := Secret(secret).GoString()
	if goStringValue != "[REDACTED]" {
		t.Errorf("Expected [REDACTED] for GoString, got %s", goStringValue)
	}
}

func TestLoggerDebugMode(t *testing.T) {
	// Test that debug mode can be toggled
	logger := New(false, true) // debug=false, noColor=true
	
	// Test debug logger creation
	debugLogger := New(true, true) // debug=true, noColor=true
	
	// Since we can't easily capture stderr in tests, just verify the loggers were created
	if logger == nil {
		t.Error("Failed to create non-debug logger")
	}
	if debugLogger == nil {
		t.Error("Failed to create debug logger")
	}
}

func TestLoggerLevels(t *testing.T) {
	// Test that logger methods exist and can be called
	logger := New(true, true)
	
	// Test that all logging methods exist and don't panic
	logger.Info("info message")
	logger.Warn("warn message") 
	logger.Error("error message")
	logger.Debug("debug message")
	
	// Test with formatted strings
	logger.Info("formatted %s message", "info")
	logger.Warn("formatted %s message", "warn")
	logger.Error("formatted %s message", "error")
	logger.Debug("formatted %s message", "debug")
}

// TestRedactFunction tests the Redact utility function
func TestRedactFunction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		secrets  []string
		expected string
	}{
		{
			name:     "single secret redacted",
			input:    "The password is secret123",
			secrets:  []string{"secret123"},
			expected: "The password is [REDACTED]",
		},
		{
			name:     "multiple secrets redacted",
			input:    "User admin with password secret123 and API key abc123",
			secrets:  []string{"admin", "secret123", "abc123"},
			expected: "User [REDACTED] with password [REDACTED] and API key [REDACTED]",
		},
		{
			name:     "no secrets to redact",
			input:    "This has no secrets",
			secrets:  []string{},
			expected: "This has no secrets",
		},
		{
			name:     "empty secret ignored",
			input:    "This has no secrets",
			secrets:  []string{""},
			expected: "This has no secrets",
		},
		{
			name:     "short secret ignored",
			input:    "Short secret: ab",
			secrets:  []string{"ab"},
			expected: "Short secret: ab", // Too short to redact
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Redact(tt.input, tt.secrets)
			if result != tt.expected {
				t.Errorf("Redact() = %q, want %q", result, tt.expected)
			}
		})
	}
}