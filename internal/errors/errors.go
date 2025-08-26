package errors

import (
	"errors"
	"fmt"
	"strings"
)

// UserError represents an error that should be shown to the user with helpful context
type UserError struct {
	Message     string
	Suggestion  string
	Details     string
	Err         error
}

func (e UserError) Error() string {
	var parts []string
	
	if e.Message != "" {
		parts = append(parts, e.Message)
	} else if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}
	
	if e.Details != "" {
		parts = append(parts, "\n  Details: "+e.Details)
	}
	
	if e.Suggestion != "" {
		parts = append(parts, "\n  💡 Try: "+e.Suggestion)
	}
	
	return strings.Join(parts, "")
}

func (e UserError) Unwrap() error {
	return e.Err
}

// ConfigError represents a configuration error with helpful context
type ConfigError struct {
	Field      string
	Value      interface{}
	Message    string
	Suggestion string
}

func (e ConfigError) Error() string {
	msg := "Configuration error"
	if e.Field != "" {
		msg += fmt.Sprintf(" in field '%s'", e.Field)
	}
	if e.Value != nil {
		msg += fmt.Sprintf(" (value: %v)", e.Value)
	}
	msg += ": " + e.Message
	
	if e.Suggestion != "" {
		msg += "\n  💡 " + e.Suggestion
	}
	
	return msg
}

// CommandError represents a command execution error
type CommandError struct {
	Command    string
	ExitCode   int
	Message    string
	Suggestion string
}

func (e CommandError) Error() string {
	msg := fmt.Sprintf("Command '%s' failed", e.Command)
	if e.ExitCode != 0 {
		msg += fmt.Sprintf(" (exit code: %d)", e.ExitCode)
	}
	if e.Message != "" {
		msg += ": " + e.Message
	}
	
	if e.Suggestion != "" {
		msg += "\n  💡 " + e.Suggestion
	}
	
	return msg
}

// ProviderError enhances provider-specific errors with context
func ProviderError(provider string, operation string, err error) error {
	// Check for common provider errors and add helpful context
	suggestion := getProviderSuggestion(provider, err)
	
	return UserError{
		Message:    fmt.Sprintf("%s provider error during %s", provider, operation),
		Suggestion: suggestion,
		Err:        err,
	}
}

// getProviderSuggestion returns helpful suggestions based on provider and error
func getProviderSuggestion(provider string, err error) string {
	errStr := err.Error()
	
	switch provider {
	case "bitwarden":
		if strings.Contains(errStr, "not logged in") {
			return "Run 'bw login' to authenticate with Bitwarden"
		}
		if strings.Contains(errStr, "vault is locked") {
			return "Run 'bw unlock' and export the BW_SESSION environment variable"
		}
		if strings.Contains(errStr, "Not found") {
			return "Verify the item name exists in Bitwarden. Use 'bw list items --search <name>' to search"
		}
		if strings.Contains(errStr, "command not found") {
			return "Install Bitwarden CLI: https://bitwarden.com/help/cli/"
		}
		
	case "1password", "onepassword":
		if strings.Contains(errStr, "not signed in") {
			return "Run 'op signin' to authenticate with 1Password"
		}
		if strings.Contains(errStr, "session expired") {
			return "Your 1Password session has expired. Run 'op signin' again"
		}
		if strings.Contains(errStr, "not found") {
			return "Verify the item exists. Use 'op item list' to see available items"
		}
		if strings.Contains(errStr, "command not found") {
			return "Install 1Password CLI: https://developer.1password.com/docs/cli/get-started/"
		}
		
	case "aws", "aws-secretsmanager":
		if strings.Contains(errStr, "credentials") || strings.Contains(errStr, "authorization") {
			return "Configure AWS credentials: 'aws configure' or set AWS_PROFILE"
		}
		if strings.Contains(errStr, "AccessDenied") {
			return "Check IAM permissions for secretsmanager:GetSecretValue"
		}
		if strings.Contains(errStr, "ResourceNotFoundException") {
			return "Verify the secret name and region. List secrets with: 'aws secretsmanager list-secrets'"
		}
		if strings.Contains(errStr, "ThrottlingException") {
			return "AWS rate limit exceeded. Wait a moment and try again"
		}
	}
	
	// Generic suggestions
	if strings.Contains(errStr, "timeout") {
		return "The operation timed out. Check your network connection and try again"
	}
	if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host") {
		return "Unable to connect. Check your network and provider configuration"
	}
	
	return ""
}

// WrapCommandNotFound wraps command not found errors with helpful suggestions
func WrapCommandNotFound(command string, err error) error {
	suggestions := map[string]string{
		"npm":    "Install Node.js from https://nodejs.org/",
		"yarn":   "Install Yarn from https://yarnpkg.com/",
		"python": "Install Python from https://python.org/",
		"pip":    "Install pip with your Python installation",
		"go":     "Install Go from https://golang.org/",
		"cargo":  "Install Rust from https://rustup.rs/",
		"docker": "Install Docker from https://docker.com/",
		"git":    "Install Git from https://git-scm.com/",
		"make":   "Install Make (usually comes with build tools)",
	}
	
	suggestion := suggestions[command]
	if suggestion == "" {
		suggestion = fmt.Sprintf("Make sure '%s' is installed and in your PATH", command)
	}
	
	return CommandError{
		Command:    command,
		Message:    "command not found",
		Suggestion: suggestion,
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	retryablePatterns := []string{
		"timeout",
		"temporary failure",
		"connection reset",
		"broken pipe",
		"rate limit",
		"throttling",
		"too many requests",
	}
	
	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}
	
	return false
}

// SimplifyError simplifies complex error messages for users
func SimplifyError(err error) error {
	if err == nil {
		return nil
	}
	
	// Unwrap to get the root cause
	rootErr := err
	for {
		unwrapped := errors.Unwrap(rootErr)
		if unwrapped == nil {
			break
		}
		rootErr = unwrapped
	}
	
	// Already a user-friendly error
	if _, ok := err.(UserError); ok {
		return err
	}
	if _, ok := err.(ConfigError); ok {
		return err
	}
	if _, ok := err.(CommandError); ok {
		return err
	}
	
	// Simplify common technical errors
	errStr := rootErr.Error()
	
	if strings.Contains(errStr, "yaml:") {
		return ConfigError{
			Message:    "Invalid YAML format",
			Suggestion: "Check for indentation errors and missing quotes",
		}
	}
	
	if strings.Contains(errStr, "json:") {
		return ConfigError{
			Message:    "Invalid JSON format",
			Suggestion: "Validate your JSON at https://jsonlint.com/",
		}
	}
	
	if strings.Contains(errStr, "permission denied") {
		return UserError{
			Message:    "Permission denied",
			Suggestion: "Check file permissions or run with appropriate privileges",
			Err:        err,
		}
	}
	
	if strings.Contains(errStr, "no such file or directory") {
		return UserError{
			Message:    "File or directory not found",
			Suggestion: "Verify the path exists and is spelled correctly",
			Err:        err,
		}
	}
	
	// Return original error if we can't simplify it
	return err
}