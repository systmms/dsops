package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/rotation/storage"
	"gopkg.in/yaml.v3"
)

// NewRotationHistoryCmd creates the rotation history command
func NewRotationHistoryCmd(cfg *config.Config) *cobra.Command {
	var (
		historyLimit  int
		historySince  string
		historyUntil  string
		historyStatus string
		historyFormat string
	)

	cmd := &cobra.Command{
		Use:   "history [service-name]",
		Short: "Show rotation history for services",
		Long: `Display the rotation history for one or all services.

Shows past rotation events including:
- Timestamp of rotation
- Service name and credential type
- Success/failure status
- Error messages (if any)
- Duration of rotation`,
		Example: `  # Show history for all services
  dsops rotation history

  # Show history for a specific service
  dsops rotation history postgres-prod

  # Show only last 10 entries
  dsops rotation history --limit 10

  # Filter by date range
  dsops rotation history --since 2024-01-01 --until 2024-12-31

  # Show only failures
  dsops rotation history --status failed`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			if err := cfg.Load(); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Initialize storage
			store := storage.NewFileStorage(storage.DefaultStorageDir())

			// Parse date filters
			var sinceTime, untilTime *time.Time
			if historySince != "" {
				t, err := time.Parse("2006-01-02", historySince)
				if err != nil {
					return fmt.Errorf("invalid since date format (use YYYY-MM-DD): %w", err)
				}
				sinceTime = &t
			}
			if historyUntil != "" {
				t, err := time.Parse("2006-01-02", historyUntil)
				if err != nil {
					return fmt.Errorf("invalid until date format (use YYYY-MM-DD): %w", err)
				}
				// Set to end of day
				endOfDay := t.Add(24*time.Hour - time.Second)
				untilTime = &endOfDay
			}

			// Get history entries
			var entries []storage.HistoryEntry

			if len(args) > 0 {
				// Specific service
				serviceName := args[0]
				if cfg.Definition != nil && cfg.Definition.Services != nil {
					if _, exists := cfg.Definition.Services[serviceName]; !exists {
						return fmt.Errorf("service '%s' not found in configuration", serviceName)
					}
				}

				history, err := store.GetHistory(serviceName, historyLimit)
				if err != nil {
					return fmt.Errorf("failed to get history: %w", err)
				}
				entries = history
			} else {
				// All services
				if cfg.Definition != nil && cfg.Definition.Services != nil {
					for serviceName := range cfg.Definition.Services {
						history, err := store.GetHistory(serviceName, historyLimit)
						if err != nil {
							continue // Skip services with no history
						}
						entries = append(entries, history...)
					}
				}

				// Sort by timestamp (newest first)
				sort.Slice(entries, func(i, j int) bool {
					return entries[i].Timestamp.After(entries[j].Timestamp)
				})

				// Apply limit after combining
				if len(entries) > historyLimit {
					entries = entries[:historyLimit]
				}
			}

			// Apply filters
			filtered := filterHistoryEntries(entries, sinceTime, untilTime, historyStatus)

			// Handle different output formats
			switch historyFormat {
			case "json":
				return outputRotationHistoryJSON(filtered)
			case "yaml":
				return outputRotationHistoryYAML(filtered)
			default:
				return outputRotationHistoryTable(filtered)
			}
		},
	}

	cmd.Flags().IntVar(&historyLimit, "limit", 50, "Maximum number of entries to show")
	cmd.Flags().StringVar(&historySince, "since", "", "Show entries since date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&historyUntil, "until", "", "Show entries until date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&historyStatus, "status", "", "Filter by status: success, failed, rolled_back")
	cmd.Flags().StringVar(&historyFormat, "format", "table", "Output format: table, json, yaml")

	return cmd
}

func filterHistoryEntries(entries []storage.HistoryEntry, since, until *time.Time, status string) []storage.HistoryEntry {
	var filtered []storage.HistoryEntry

	for _, entry := range entries {
		// Filter by date range
		if since != nil && entry.Timestamp.Before(*since) {
			continue
		}
		if until != nil && entry.Timestamp.After(*until) {
			continue
		}

		// Filter by status
		if status != "" && !strings.EqualFold(entry.Status, status) {
			continue
		}

		filtered = append(filtered, entry)
	}

	return filtered
}

func outputRotationHistoryTable(entries []storage.HistoryEntry) error {
	if len(entries) == 0 {
		fmt.Println("No rotation history found matching criteria")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	// Print header
	fmt.Fprintln(w, "TIMESTAMP\tSERVICE\tTYPE\tSTATUS\tDURATION\tERROR")
	fmt.Fprintln(w, "---------\t-------\t----\t------\t--------\t-----")

	for _, entry := range entries {
		timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")

		// Format credential type
		credType := entry.CredentialType
		if credType == "" {
			credType = "unknown"
		}

		// Format status with icons
		status := formatResult(entry.Status)

		// Format duration
		duration := formatDuration(entry.Duration)

		// Format error (truncate if too long)
		errorMsg := "-"
		if entry.Error != "" {
			errorMsg = entry.Error
			if len(errorMsg) > 50 {
				errorMsg = errorMsg[:47] + "..."
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			timestamp,
			entry.ServiceName,
			credType,
			status,
			duration,
			errorMsg,
		)
	}

	// Show summary
	fmt.Printf("\nShowing %d entries", len(entries))
	if historySince := getHistorySince(); historySince != "" {
		fmt.Printf(" (filtered by status: %s)", historySince)
	}
	fmt.Println()

	return nil
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}

	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}

func outputRotationHistoryJSON(entries []storage.HistoryEntry) error {
	// Convert entries to JSON-friendly format
	jsonEntries := make([]map[string]interface{}, len(entries))

	for i, entry := range entries {
		jsonEntry := map[string]interface{}{
			"timestamp":       entry.Timestamp,
			"service_name":    entry.ServiceName,
			"credential_type": entry.CredentialType,
			"action":          entry.Action,
			"status":          entry.Status,
			"duration_ms":     entry.Duration.Milliseconds(),
			"user":            entry.User,
			"metadata":        entry.Metadata,
		}

		if entry.Error != "" {
			jsonEntry["error"] = entry.Error
		}

		jsonEntries[i] = jsonEntry
	}

	result := map[string]interface{}{
		"count":   len(entries),
		"entries": jsonEntries,
	}

	return outputRotHistJSON(result)
}

func outputRotationHistoryYAML(entries []storage.HistoryEntry) error {
	// Similar to JSON but with YAML output
	yamlEntries := make([]map[string]interface{}, len(entries))

	for i, entry := range entries {
		yamlEntry := map[string]interface{}{
			"timestamp":       entry.Timestamp.Format(time.RFC3339),
			"service_name":    entry.ServiceName,
			"credential_type": entry.CredentialType,
			"action":          entry.Action,
			"status":          entry.Status,
			"duration_ms":     entry.Duration.Milliseconds(),
		}

		if entry.Error != "" {
			yamlEntry["error"] = entry.Error
		}

		if entry.User != "" {
			yamlEntry["user"] = entry.User
		}

		if len(entry.Metadata) > 0 {
			yamlEntry["metadata"] = entry.Metadata
		}

		yamlEntries[i] = yamlEntry
	}

	result := map[string]interface{}{
		"count":   len(entries),
		"entries": yamlEntries,
	}

	return outputRotHistYAML(result)
}

// outputRotHistJSON outputs data as JSON
func outputRotHistJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// outputRotHistYAML outputs data as YAML
func outputRotHistYAML(data interface{}) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	return encoder.Encode(data)
}

// getHistorySince is a helper function to access the flag value
func getHistorySince() string {
	// This is a placeholder - in real implementation, 
	// the flag value would be passed through the command context
	return ""
}