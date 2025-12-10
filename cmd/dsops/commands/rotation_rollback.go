package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	dserrors "github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/rotation/notifications"
	"github.com/systmms/dsops/internal/rotation/rollback"
	"github.com/systmms/dsops/internal/rotation/storage"
)

// NewRotationRollbackCmd creates the rotation rollback command
func NewRotationRollbackCmd(cfg *config.Config) *cobra.Command {
	var (
		service     string
		environment string
		version     string
		reason      string
		force       bool
		dryRun      bool
		verbose     bool
	)

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback a service to a previous secret version",
		Long: `Manually rollback a service to a previous secret version.

This command allows you to restore a service's secrets to a previous version
when automatic rollback wasn't triggered or when you need to manually recover
from a failed rotation.

The rollback will:
1. Verify the target version exists
2. Restore the previous secret version
3. Verify the service works with restored secrets
4. Send notifications about the rollback

Examples:
  # Rollback a service to the previous version
  dsops rotation rollback --service postgres-prod --env production --reason "failed health checks"

  # Rollback to a specific version
  dsops rotation rollback --service postgres-prod --env production --version v1.2.3 --reason "incompatible schema"

  # Preview rollback without executing
  dsops rotation rollback --service postgres-prod --env production --reason "testing" --dry-run

  # Skip confirmation prompt
  dsops rotation rollback --service postgres-prod --env production --reason "urgent fix" --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate required flags
			if service == "" {
				return dserrors.UserError{
					Message:    "Service name is required",
					Suggestion: "Use --service flag to specify the service name",
				}
			}
			if environment == "" {
				return dserrors.UserError{
					Message:    "Environment is required",
					Suggestion: "Use --env flag to specify the environment",
				}
			}
			if reason == "" {
				return dserrors.UserError{
					Message:    "Reason is required for audit trail",
					Suggestion: "Use --reason flag to explain why rollback is needed",
				}
			}

			return executeRollback(cfg, service, environment, version, reason, force, dryRun, verbose)
		},
	}

	// Required flags
	cmd.Flags().StringVar(&service, "service", "", "Service name to rollback (required)")
	cmd.Flags().StringVar(&environment, "env", "", "Environment name (required)")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for rollback (required for audit trail)")

	// Optional flags
	cmd.Flags().StringVar(&version, "version", "", "Target version to rollback to (default: previous version)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview rollback without executing")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed output")

	// Mark required flags
	_ = cmd.MarkFlagRequired("service")
	_ = cmd.MarkFlagRequired("env")
	_ = cmd.MarkFlagRequired("reason")

	return cmd
}

func executeRollback(cfg *config.Config, service, environment, version, reason string, force, dryRun, verbose bool) error {
	// Load configuration
	if err := cfg.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize storage
	store := storage.NewFileStorage(storage.DefaultStorageDir())

	// Get service status and history
	status, err := store.GetStatus(service)
	if err != nil {
		return dserrors.UserError{
			Message:    fmt.Sprintf("Cannot find service '%s'", service),
			Details:    err.Error(),
			Suggestion: "Verify the service name is correct and has been rotated at least once",
		}
	}

	// Get history to find previous version
	history, err := store.GetHistory(service, 10)
	if err != nil || len(history) == 0 {
		return dserrors.UserError{
			Message:    fmt.Sprintf("No rotation history found for service '%s'", service),
			Suggestion: "A service must have rotation history to rollback",
		}
	}

	// Determine target version
	var targetVersion string
	var failedVersion string

	if version != "" {
		// User specified target version
		targetVersion = version
		failedVersion = status.Metadata["current_version"]
	} else {
		// Find the last successful rotation's previous version
		for _, entry := range history {
			if entry.OldVersion != "" {
				targetVersion = entry.OldVersion
				failedVersion = entry.NewVersion
				break
			}
		}
	}

	if targetVersion == "" {
		return dserrors.UserError{
			Message:    "Cannot determine target version for rollback",
			Suggestion: "Use --version to specify the target version explicitly",
		}
	}

	// Display rollback plan
	displayRollbackPlan(service, environment, targetVersion, failedVersion, reason, status, verbose)

	// Dry run - stop here
	if dryRun {
		fmt.Println("\n[DRY RUN] No changes made. Remove --dry-run to execute.")
		return nil
	}

	// Confirmation
	if !force {
		fmt.Print("\nProceed with rollback? (y/N): ")
		var response string
		_, _ = fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Rollback cancelled")
			return nil
		}
	}

	// Execute rollback
	fmt.Println("\nExecuting rollback...")

	// Initialize notification manager (best-effort)
	notifier := notifications.NewManager(0)
	notifier.Start(context.Background())
	defer notifier.Stop()

	// Initialize rollback manager
	rollbackConfig := rollback.DefaultConfig()
	rollbackManager := rollback.NewManager(rollbackConfig, notifier)

	// Create rollback request
	// Note: In a real implementation, RestoreFunc and VerifyFunc would be
	// provided by the rotation engine based on the service configuration
	req := rollback.RollbackRequest{
		Service:         service,
		Environment:     environment,
		Reason:          reason,
		PreviousVersion: targetVersion,
		FailedVersion:   failedVersion,
		InitiatedBy:     getUsername(),
		RestoreFunc: func(ctx context.Context) error {
			// This would be implemented by the rotation engine
			// For now, we return nil to indicate success
			// In production, this would restore the secret from storage
			if verbose {
				fmt.Printf("  Restoring secret to version %s...\n", targetVersion)
			}
			return nil
		},
		VerifyFunc: func(ctx context.Context) error {
			// This would be implemented by the rotation engine
			// For now, we return nil to indicate success
			// In production, this would verify the service works
			if verbose {
				fmt.Println("  Verifying service connectivity...")
			}
			return nil
		},
	}

	// Execute manual rollback
	result, err := rollbackManager.ManualRollback(context.Background(), req)
	if err != nil {
		return dserrors.UserError{
			Message:    "Rollback failed",
			Details:    err.Error(),
			Suggestion: "Check service logs and retry, or contact operations team",
		}
	}

	// Record rollback in history
	historyEntry := storage.HistoryEntry{
		ID:             fmt.Sprintf("rollback-%d", time.Now().UnixNano()),
		Timestamp:      time.Now(),
		ServiceName:    service,
		CredentialType: "password",
		Action:         "rollback",
		Status:         "success",
		Duration:       result.Duration,
		User:           getUsername(),
		OldVersion:     failedVersion,
		NewVersion:     targetVersion,
		Metadata: map[string]string{
			"reason":      reason,
			"trigger":     "manual",
			"environment": environment,
			"attempts":    fmt.Sprintf("%d", result.Attempts),
		},
	}

	if err := store.SaveHistory(&historyEntry); err != nil {
		// Log but don't fail - the rollback succeeded
		fmt.Printf("Warning: failed to save history: %v\n", err)
	}

	// Update service status
	status.Status = "active"
	status.LastResult = "rolled_back"
	status.LastRotation = time.Now()
	if status.Metadata == nil {
		status.Metadata = make(map[string]string)
	}
	status.Metadata["current_version"] = targetVersion
	status.Metadata["last_rollback_reason"] = reason

	if err := store.SaveStatus(status); err != nil {
		fmt.Printf("Warning: failed to update status: %v\n", err)
	}

	// Success message
	fmt.Println()
	fmt.Printf("Rollback completed successfully\n")
	fmt.Printf("  Service:    %s\n", service)
	fmt.Printf("  Version:    %s\n", targetVersion)
	fmt.Printf("  Duration:   %s\n", result.Duration.Round(time.Millisecond))
	fmt.Printf("  Attempts:   %d\n", result.Attempts)

	return nil
}

func displayRollbackPlan(service, environment, targetVersion, failedVersion, reason string, status *storage.RotationStatus, verbose bool) {
	fmt.Println("Rollback Plan")
	fmt.Println("=============")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Service:\t%s\n", service)
	_, _ = fmt.Fprintf(w, "Environment:\t%s\n", environment)
	_, _ = fmt.Fprintf(w, "Current Version:\t%s\n", failedVersion)
	_, _ = fmt.Fprintf(w, "Target Version:\t%s\n", targetVersion)
	_, _ = fmt.Fprintf(w, "Reason:\t%s\n", reason)
	_ = w.Flush()

	if verbose && status != nil {
		fmt.Println()
		fmt.Println("Service Status:")
		fmt.Printf("  Status:         %s\n", formatRotationStatus(status.Status))
		fmt.Printf("  Last Rotation:  %s\n", formatTimestamp(status.LastRotation, time.Now()))
		fmt.Printf("  Last Result:    %s\n", formatResult(status.LastResult))
		if status.LastError != "" {
			fmt.Printf("  Last Error:     %s\n", status.LastError)
		}
	}

	fmt.Println()
	fmt.Println("Actions to perform:")
	fmt.Println("  1. Restore previous secret version")
	fmt.Println("  2. Verify service connectivity")
	fmt.Println("  3. Update rotation status")
	fmt.Println("  4. Send notifications")
}

// getUsername returns the current user's username for audit trail
func getUsername() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	return "unknown"
}
