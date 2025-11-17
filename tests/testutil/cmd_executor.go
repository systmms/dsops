// Package testutil provides testing utilities for dsops.
package testutil

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// CommandExecutor defines an interface for executing shell commands.
// This abstraction allows for mocking CLI tool behavior in tests.
type CommandExecutor interface {
	// Execute runs a command with the given context and arguments.
	// Returns stdout, stderr, and any error that occurred.
	Execute(ctx context.Context, name string, args ...string) (stdout []byte, stderr []byte, err error)
}

// RealCommandExecutor executes actual shell commands using os/exec.
// This is the production implementation.
type RealCommandExecutor struct{}

// Execute runs an actual shell command.
func (r *RealCommandExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// MockCommandExecutor provides a configurable mock for testing CLI-based providers.
type MockCommandExecutor struct {
	mu sync.Mutex

	// Responses maps command patterns to their mock responses.
	// Key format: "command arg1 arg2" (space-separated command and args)
	Responses map[string]MockResponse

	// DefaultResponse is used when no matching pattern is found.
	DefaultResponse *MockResponse

	// RecordedCalls stores all calls made to Execute for verification.
	RecordedCalls []RecordedCall

	// StrictMode causes Execute to fail if no matching response is found.
	StrictMode bool
}

// MockResponse defines the expected output for a mocked command.
type MockResponse struct {
	Stdout   []byte
	Stderr   []byte
	Err      error
	ExitCode int // Used to simulate exit codes when Err is nil
}

// RecordedCall stores information about a command execution.
type RecordedCall struct {
	Command string
	Args    []string
	Context context.Context
}

// NewMockCommandExecutor creates a new mock executor with empty responses.
func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		Responses:     make(map[string]MockResponse),
		RecordedCalls: make([]RecordedCall, 0),
	}
}

// Execute returns the mocked response for the given command.
func (m *MockCommandExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.RecordedCalls = append(m.RecordedCalls, RecordedCall{
		Command: name,
		Args:    args,
		Context: ctx,
	})

	// Build the command key for lookup
	key := m.buildKey(name, args)

	// Try exact match first
	if resp, ok := m.Responses[key]; ok {
		return resp.Stdout, resp.Stderr, resp.Err
	}

	// Try partial/prefix matching for flexibility
	for pattern, resp := range m.Responses {
		if m.matchesPattern(key, pattern) {
			return resp.Stdout, resp.Stderr, resp.Err
		}
	}

	// Use default response if available
	if m.DefaultResponse != nil {
		return m.DefaultResponse.Stdout, m.DefaultResponse.Stderr, m.DefaultResponse.Err
	}

	// Strict mode fails on unknown commands
	if m.StrictMode {
		return nil, nil, fmt.Errorf("mock: no response configured for command: %s", key)
	}

	// Non-strict mode returns empty success
	return []byte{}, []byte{}, nil
}

// buildKey creates a lookup key from command and arguments.
func (m *MockCommandExecutor) buildKey(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

// matchesPattern checks if the command key matches a pattern.
// Supports simple prefix matching for flexible response configuration.
func (m *MockCommandExecutor) matchesPattern(key, pattern string) bool {
	// Support wildcard patterns with "*"
	if strings.Contains(pattern, "*") {
		// Replace * with .* for prefix matching
		pattern = strings.ReplaceAll(pattern, "*", ".*")
		return strings.HasPrefix(key, strings.Split(pattern, ".*")[0])
	}

	// Check if key starts with pattern (allows additional args)
	return strings.HasPrefix(key, pattern)
}

// AddResponse registers a mock response for a specific command pattern.
func (m *MockCommandExecutor) AddResponse(commandPattern string, response MockResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses[commandPattern] = response
}

// AddJSONResponse is a convenience method to add a JSON response.
func (m *MockCommandExecutor) AddJSONResponse(commandPattern string, jsonData string) {
	m.AddResponse(commandPattern, MockResponse{
		Stdout: []byte(jsonData),
		Stderr: []byte{},
		Err:    nil,
	})
}

// AddErrorResponse adds an error response for a command pattern.
func (m *MockCommandExecutor) AddErrorResponse(commandPattern string, errMsg string, exitCode int) {
	m.AddResponse(commandPattern, MockResponse{
		Stdout:   []byte{},
		Stderr:   []byte(errMsg),
		Err:      fmt.Errorf("exit status %d: %s", exitCode, errMsg),
		ExitCode: exitCode,
	})
}

// GetCalls returns all recorded calls matching the given command name.
func (m *MockCommandExecutor) GetCalls(commandName string) []RecordedCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	var matches []RecordedCall
	for _, call := range m.RecordedCalls {
		if call.Command == commandName {
			matches = append(matches, call)
		}
	}
	return matches
}

// CallCount returns the number of times Execute was called.
func (m *MockCommandExecutor) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.RecordedCalls)
}

// Reset clears all recorded calls and responses.
func (m *MockCommandExecutor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses = make(map[string]MockResponse)
	m.RecordedCalls = make([]RecordedCall, 0)
	m.DefaultResponse = nil
}

// AssertCalled verifies that a specific command was called at least once.
func (m *MockCommandExecutor) AssertCalled(t interface{ Error(args ...interface{}) }, commandName string) bool {
	calls := m.GetCalls(commandName)
	if len(calls) == 0 {
		t.Error("expected command", commandName, "to be called, but it was not")
		return false
	}
	return true
}

// AssertNotCalled verifies that a specific command was never called.
func (m *MockCommandExecutor) AssertNotCalled(t interface{ Error(args ...interface{}) }, commandName string) bool {
	calls := m.GetCalls(commandName)
	if len(calls) > 0 {
		t.Error("expected command", commandName, "to not be called, but it was called", len(calls), "times")
		return false
	}
	return true
}

// AssertCallCount verifies the exact number of times a command was called.
func (m *MockCommandExecutor) AssertCallCount(t interface{ Error(args ...interface{}) }, commandName string, expected int) bool {
	calls := m.GetCalls(commandName)
	if len(calls) != expected {
		t.Error("expected command", commandName, "to be called", expected, "times, but was called", len(calls), "times")
		return false
	}
	return true
}

// BitwardenMockResponses provides pre-configured responses for Bitwarden CLI.
type BitwardenMockResponses struct{}

// StatusUnlocked returns a mock response for an unlocked Bitwarden vault.
func (BitwardenMockResponses) StatusUnlocked() MockResponse {
	return MockResponse{
		Stdout: []byte(`{
			"serverUrl": "https://vault.bitwarden.com",
			"lastSync": "2024-01-15T10:30:00.000Z",
			"userEmail": "user@example.com",
			"userId": "user-123",
			"status": "unlocked"
		}`),
		Err: nil,
	}
}

// StatusLocked returns a mock response for a locked Bitwarden vault.
func (BitwardenMockResponses) StatusLocked() MockResponse {
	return MockResponse{
		Stdout: []byte(`{
			"serverUrl": "https://vault.bitwarden.com",
			"lastSync": "2024-01-15T10:30:00.000Z",
			"userEmail": "user@example.com",
			"userId": "user-123",
			"status": "locked"
		}`),
		Err: nil,
	}
}

// StatusUnauthenticated returns a mock response for unauthenticated state.
func (BitwardenMockResponses) StatusUnauthenticated() MockResponse {
	return MockResponse{
		Stdout: []byte(`{
			"serverUrl": "https://vault.bitwarden.com",
			"lastSync": null,
			"userEmail": null,
			"userId": null,
			"status": "unauthenticated"
		}`),
		Err: nil,
	}
}

// Item returns a mock Bitwarden item response.
func (BitwardenMockResponses) Item(id, name, username, password string) MockResponse {
	return MockResponse{
		Stdout: []byte(fmt.Sprintf(`{
			"id": "%s",
			"name": "%s",
			"type": 1,
			"login": {
				"username": "%s",
				"password": "%s",
				"totp": "JBSWY3DPEHPK3PXP",
				"uris": [
					{"uri": "https://example.com", "match": null}
				]
			},
			"fields": [
				{"name": "api_key", "value": "secret-key-123", "type": 0}
			],
			"notes": "Test notes for the item"
		}`, id, name, username, password)),
		Err: nil,
	}
}

// OnePasswordMockResponses provides pre-configured responses for 1Password CLI.
type OnePasswordMockResponses struct{}

// AccountGet returns a mock response for op account get.
func (OnePasswordMockResponses) AccountGet() MockResponse {
	return MockResponse{
		Stdout: []byte(`{
			"id": "ABCD123",
			"name": "Personal",
			"domain": "my.1password.com",
			"type": "Individual",
			"state": "active",
			"created_at": "2024-01-01T00:00:00Z"
		}`),
		Err: nil,
	}
}

// ItemRead returns a mock 1Password item response.
func (OnePasswordMockResponses) ItemRead(vault, item, username, password string) MockResponse {
	return MockResponse{
		Stdout: []byte(fmt.Sprintf(`{
			"id": "%s",
			"title": "Test Login",
			"vault": {
				"id": "%s",
				"name": "Test Vault"
			},
			"category": "LOGIN",
			"fields": [
				{"id": "username", "type": "STRING", "purpose": "USERNAME", "label": "username", "value": "%s"},
				{"id": "password", "type": "CONCEALED", "purpose": "PASSWORD", "label": "password", "value": "%s"},
				{"id": "custom", "type": "STRING", "label": "api_key", "value": "custom-value-123"}
			]
		}`, item, vault, username, password)),
		Err: nil,
	}
}

// DopplerMockResponses provides pre-configured responses for Doppler CLI.
type DopplerMockResponses struct{}

// SecretsDownload returns a mock Doppler secrets response.
func (DopplerMockResponses) SecretsDownload() MockResponse {
	return MockResponse{
		Stdout: []byte(`{
			"API_KEY": {"computed": "sk-live-123456"},
			"DATABASE_URL": {"computed": "postgres://user:pass@localhost/db"},
			"SECRET_KEY": {"computed": "super-secret-key"}
		}`),
		Err: nil,
	}
}

// PassMockResponses provides pre-configured responses for pass CLI.
type PassMockResponses struct{}

// Show returns a mock password from pass store.
func (PassMockResponses) Show(password string) MockResponse {
	return MockResponse{
		Stdout: []byte(password + "\nuser: testuser\nurl: https://example.com\n"),
		Err:    nil,
	}
}
