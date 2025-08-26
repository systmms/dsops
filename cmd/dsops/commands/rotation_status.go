package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/rotation/storage"
	"gopkg.in/yaml.v3"
)

// NewRotationStatusCmd creates the rotation status command
func NewRotationStatusCmd(cfg *config.Config) *cobra.Command {
	var (
		statusVerbose bool
		statusFormat  string
	)
	
	cmd := &cobra.Command{
		Use:   "status [service-name]",
		Short: "Show rotation status for services",
		Long: `Display the current rotation status for one or all services.

Shows information including:
- Current rotation state
- Last rotation timestamp
- Next scheduled rotation
- Recent rotation results
- Health check status`,
		Example: `  # Show status for all services
  dsops rotation status

  # Show status for a specific service
  dsops rotation status postgres-prod

  # Show status with additional details
  dsops rotation status --verbose`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			if err := cfg.Load(); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			
			// Initialize storage
			store := storage.NewFileStorage(storage.DefaultStorageDir())
			
			// Get specific service or all services
			var servicesToShow []string
			if len(args) > 0 {
				servicesToShow = []string{args[0]}
				// Verify service exists
				if cfg.Definition != nil && cfg.Definition.Services != nil {
					if _, exists := cfg.Definition.Services[args[0]]; !exists {
						return fmt.Errorf("service '%s' not found in configuration", args[0])
					}
				}
			} else {
				// Show all services
				if cfg.Definition != nil && cfg.Definition.Services != nil {
					for name := range cfg.Definition.Services {
						servicesToShow = append(servicesToShow, name)
					}
				}
			}
			
			// Handle different output formats
			switch statusFormat {
			case "json":
				return outputRotationStatusJSON(store, servicesToShow)
			case "yaml":
				return outputRotationStatusYAML(store, servicesToShow)
			default:
				return outputRotationStatusTable(store, servicesToShow, statusVerbose)
			}
		},
	}
	
	cmd.Flags().BoolVarP(&statusVerbose, "verbose", "v", false, "Show detailed status information")
	cmd.Flags().StringVar(&statusFormat, "format", "table", "Output format: table, json, yaml")
	
	return cmd
}

func outputRotationStatusTable(store storage.Storage, services []string, verbose bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()
	
	// Print header
	fmt.Fprintln(w, "SERVICE\tSTATUS\tLAST ROTATION\tNEXT ROTATION\tRESULT")
	fmt.Fprintln(w, "-------\t------\t-------------\t-------------\t------")
	
	now := time.Now()
	
	for _, serviceName := range services {
		// Get rotation status from storage
		status, err := store.GetStatus(serviceName)
		if err != nil {
			// No status yet
			status = &storage.RotationStatus{
				ServiceName: serviceName,
				Status:      "never_rotated",
			}
		}
		
		// Format status
		statusStr := formatRotationStatus(status.Status)
		
		// Format last rotation
		lastRotationStr := "Never"
		if !status.LastRotation.IsZero() {
			lastRotationStr = formatTimestamp(status.LastRotation, now)
		}
		
		// Calculate next rotation
		nextRotationStr := "Not scheduled"
		if status.NextRotation != nil && !status.NextRotation.IsZero() {
			nextRotationStr = formatTimestamp(*status.NextRotation, now)
		} else if status.RotationInterval > 0 && !status.LastRotation.IsZero() {
			nextRotation := status.LastRotation.Add(status.RotationInterval)
			nextRotationStr = formatTimestamp(nextRotation, now)
		}
		
		// Format last result
		resultStr := "-"
		if status.LastResult != "" {
			resultStr = formatResult(status.LastResult)
		}
		
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			serviceName,
			statusStr,
			lastRotationStr,
			nextRotationStr,
			resultStr,
		)
		
		// Show additional details in verbose mode
		if verbose && status.LastError != "" {
			fmt.Fprintf(w, "  └─ Error: %s\n", status.LastError)
		}
	}
	
	return nil
}

func formatRotationStatus(status string) string {
	switch status {
	case "active", "healthy":
		return "✅ Active"
	case "rotating":
		return "🔄 Rotating"
	case "failed":
		return "❌ Failed"
	case "never_rotated":
		return "⚪ Never Rotated"
	case "needs_rotation":
		return "🟡 Needs Rotation"
	default:
		return status
	}
}

func formatTimestamp(t time.Time, now time.Time) string {
	diff := now.Sub(t)
	
	// Future time
	if diff < 0 {
		diff = -diff
		if diff < time.Hour {
			return fmt.Sprintf("in %d min", int(diff.Minutes()))
		} else if diff < 24*time.Hour {
			return fmt.Sprintf("in %d hr", int(diff.Hours()))
		} else {
			return fmt.Sprintf("in %d days", int(diff.Hours()/24))
		}
	}
	
	// Past time
	if diff < time.Minute {
		return "Just now"
	} else if diff < time.Hour {
		return fmt.Sprintf("%d min ago", int(diff.Minutes()))
	} else if diff < 24*time.Hour {
		return fmt.Sprintf("%d hr ago", int(diff.Hours()))
	} else if diff < 7*24*time.Hour {
		return fmt.Sprintf("%d days ago", int(diff.Hours()/24))
	} else {
		return t.Format("2006-01-02")
	}
}

func formatResult(result string) string {
	switch result {
	case "success", "completed":
		return "✅ Success"
	case "failed":
		return "❌ Failed"
	case "partial":
		return "🟡 Partial"
	case "rolled_back":
		return "↩️ Rolled Back"
	default:
		return result
	}
}

func outputRotationStatusJSON(store storage.Storage, services []string) error {
	// Collect status for all services
	statuses := make(map[string]interface{})
	
	for _, serviceName := range services {
		status, err := store.GetStatus(serviceName)
		if err != nil {
			status = &storage.RotationStatus{
				ServiceName: serviceName,
				Status:      "never_rotated",
			}
		}
		
		// Convert to JSON-friendly format
		statusData := map[string]interface{}{
			"service_name":      status.ServiceName,
			"status":           status.Status,
			"last_rotation":    status.LastRotation,
			"next_rotation":    status.NextRotation,
			"last_result":      status.LastResult,
			"last_error":       status.LastError,
			"rotation_count":   status.RotationCount,
			"success_count":    status.SuccessCount,
			"failure_count":    status.FailureCount,
		}
		
		statuses[serviceName] = statusData
	}
	
	return outputRotJSON(statuses)
}

func outputRotationStatusYAML(store storage.Storage, services []string) error {
	// Similar to JSON but with YAML output
	statuses := make(map[string]interface{})
	
	for _, serviceName := range services {
		status, err := store.GetStatus(serviceName)
		if err != nil {
			status = &storage.RotationStatus{
				ServiceName: serviceName,
				Status:      "never_rotated",
			}
		}
		
		statusData := map[string]interface{}{
			"service_name":      status.ServiceName,
			"status":           status.Status,
			"last_rotation":    status.LastRotation,
			"next_rotation":    status.NextRotation,
			"last_result":      status.LastResult,
			"rotation_count":   status.RotationCount,
			"success_count":    status.SuccessCount,
			"failure_count":    status.FailureCount,
		}
		
		if status.LastError != "" {
			statusData["last_error"] = status.LastError
		}
		
		statuses[serviceName] = statusData
	}
	
	return outputRotYAML(statuses)
}

// outputRotJSON outputs data as JSON
func outputRotJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// outputRotYAML outputs data as YAML
func outputRotYAML(data interface{}) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	return encoder.Encode(data)
}