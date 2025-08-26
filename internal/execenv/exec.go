package execenv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"time"

	dserrors "github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/logging"
)

// Executor handles running commands with ephemeral environment variables
type Executor struct {
	logger *logging.Logger
}

// New creates a new executor
func New(logger *logging.Logger) *Executor {
	return &Executor{
		logger: logger,
	}
}

// ExecOptions configures command execution
type ExecOptions struct {
	Command         []string          // Command and arguments to run
	Environment     map[string]string // Environment variables to set
	AllowOverride   bool              // Allow existing env vars to override dsops values
	PrintVars       bool              // Print resolved variables (names only, values masked)
	WorkingDir      string            // Working directory for the command
	Timeout         int               // Timeout in seconds (0 for no timeout)
}

// Exec runs a command with the provided environment variables
func (e *Executor) Exec(ctx context.Context, options ExecOptions) error {
	if len(options.Command) == 0 {
		return dserrors.UserError{
			Message:    "No command specified",
			Suggestion: "Provide a command after -- (e.g., dsops exec development -- npm start)",
		}
	}

	// Apply timeout if specified
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(options.Timeout)*time.Second)
		defer cancel()
	}

	// Validate command exists
	cmdName := options.Command[0]
	if _, err := exec.LookPath(cmdName); err != nil {
		return dserrors.WrapCommandNotFound(cmdName, err)
	}

	// Build environment
	env, err := e.buildEnvironment(options.Environment, options.AllowOverride)
	if err != nil {
		return dserrors.UserError{
			Message:    "Failed to build environment",
			Details:    err.Error(),
			Suggestion: "Check your dsops.yaml configuration for errors",
			Err:        err,
		}
	}

	// Print variables if requested
	if options.PrintVars {
		e.printEnvironment(options.Environment)
	}

	// Create command
	cmd := exec.CommandContext(ctx, cmdName, options.Command[1:]...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Set working directory if specified
	if options.WorkingDir != "" {
		cmd.Dir = options.WorkingDir
	}

	e.logger.Debug("Executing command: %s", strings.Join(options.Command, " "))
	e.logger.Debug("Environment variables set: %d", len(options.Environment))

	// Run the command
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// Preserve the exit code from the child process
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
			os.Exit(1)
		}
		return dserrors.CommandError{
			Command:    strings.Join(options.Command, " "),
			Message:    err.Error(),
			Suggestion: "Check the command output above for details",
		}
	}

	return nil
}

// buildEnvironment creates the environment slice for the child process
func (e *Executor) buildEnvironment(dsopsVars map[string]string, allowOverride bool) ([]string, error) {
	// Start with current environment
	currentEnv := os.Environ()
	envMap := make(map[string]string)

	// Parse current environment into map
	for _, env := range currentEnv {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Add dsops variables
	for key, value := range dsopsVars {
		if allowOverride {
			// Only set if not already present
			if _, exists := envMap[key]; !exists {
				envMap[key] = value
			}
		} else {
			// dsops values take precedence
			envMap[key] = value
		}
	}

	// Convert back to environment slice
	result := make([]string, 0, len(envMap))
	for key, value := range envMap {
		result = append(result, fmt.Sprintf("%s=%s", key, value))
	}

	// Sort for consistent ordering (helps with debugging)
	sort.Strings(result)

	return result, nil
}

// printEnvironment displays the resolved variables (values masked for security)
func (e *Executor) printEnvironment(environment map[string]string) {
	if len(environment) == 0 {
		fmt.Println("No environment variables resolved")
		return
	}

	fmt.Printf("Resolved %d environment variables:\n", len(environment))
	
	// Sort keys for consistent output
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := environment[key]
		maskedValue := maskValue(value)
		fmt.Printf("  %s=%s\n", key, maskedValue)
	}
	fmt.Println()
}

// maskValue masks a secret value for display
func maskValue(value string) string {
	if len(value) == 0 {
		return "(empty)"
	}
	
	// Show first and last characters for very short values
	if len(value) <= 3 {
		return strings.Repeat("*", len(value))
	}
	
	// Show first 2 and last 1 characters for longer values
	if len(value) <= 8 {
		return value[:1] + strings.Repeat("*", len(value)-2) + value[len(value)-1:]
	}
	
	// For long values, show first 3 and last 2 with asterisks in between
	return value[:3] + strings.Repeat("*", 8) + value[len(value)-2:]
}

// ValidateCommand checks if a command is safe and accessible
func ValidateCommand(command []string) error {
	if len(command) == 0 {
		return dserrors.UserError{
			Message:    "No command specified",
			Suggestion: "Provide a command after -- (e.g., dsops exec development -- npm start)",
		}
	}

	cmdName := command[0]
	
	// Check if command exists in PATH
	if _, err := exec.LookPath(cmdName); err != nil {
		return dserrors.WrapCommandNotFound(cmdName, err)
	}

	// Security check: prevent some dangerous commands
	// Note: This is not comprehensive security - just basic safety
	dangerousCommands := []string{
		"rm", "rmdir", "del", "format", "fdisk",
		"dd", "mkfs", "parted", "shutdown", "reboot",
	}
	
	for _, dangerous := range dangerousCommands {
		if cmdName == dangerous || strings.HasSuffix(cmdName, "/"+dangerous) {
			return dserrors.UserError{
				Message:    fmt.Sprintf("Potentially dangerous command '%s'", cmdName),
				Suggestion: "Use this command with extreme caution or consider safer alternatives",
			}
		}
	}

	return nil
}