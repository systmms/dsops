// Package exec provides abstractions for command execution.
// This package enables testable code by allowing CLI commands to be mocked.
package exec

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

// CommandExecutor defines an interface for executing shell commands.
// This abstraction allows for mocking CLI tool behavior in tests.
type CommandExecutor interface {
	// Execute runs a command with the given context and arguments.
	// Returns stdout, stderr, and any error that occurred.
	Execute(ctx context.Context, name string, args ...string) (stdout []byte, stderr []byte, err error)
}

// EnvCommandExecutor extends CommandExecutor with per-call environment
// variable injection. Implementations append the supplied entries to the
// caller's existing environment (os.Environ) before spawning the child.
// Each entry is a "KEY=VALUE" string, matching exec.Cmd.Env semantics.
// When env is empty or nil the call is equivalent to Execute (no env
// mutation on the child).
type EnvCommandExecutor interface {
	CommandExecutor
	ExecuteWithEnv(ctx context.Context, env []string, name string, args ...string) (stdout []byte, stderr []byte, err error)
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

// ExecuteWithEnv runs a shell command with additional environment variables
// appended to the parent process's environment. Entries later in env
// override earlier ones, including any matching keys from os.Environ().
func (r *RealCommandExecutor) ExecuteWithEnv(ctx context.Context, env []string, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
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
