// Package exec provides abstractions for command execution.
// This package enables testable code by allowing CLI commands to be mocked.
package exec

import (
	"bytes"
	"context"
	"os/exec"
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

// DefaultExecutor returns the standard production executor.
// This is used as the default when no executor is injected.
func DefaultExecutor() CommandExecutor {
	return &RealCommandExecutor{}
}
