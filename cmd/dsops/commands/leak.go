package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	dserrors "github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/incident"
)

func NewLeakCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "leak",
		Short: "Security incident response and management",
		Long: `Manage security incidents including secret leaks, compromised credentials, and policy violations.

Subcommands:
  report  Create a new incident report
  list    List all incidents
  show    Show details of a specific incident
  update  Update an existing incident
  resolve Mark an incident as resolved

Examples:
  dsops leak report                    # Interactive incident reporting
  dsops leak list                      # Show all incidents
  dsops leak show INC-20250118-12345   # Show specific incident
  dsops leak resolve INC-20250118-12345`,
	}

	cmd.AddCommand(
		NewLeakReportCommand(cfg),
		NewLeakListCommand(cfg),
		NewLeakShowCommand(cfg),
		NewLeakUpdateCommand(cfg),
		NewLeakResolveCommand(cfg),
	)

	return cmd
}

func NewLeakReportCommand(cfg *config.Config) *cobra.Command {
	var (
		incidentType string
		severity     string
		title        string
		description  string
		files        []string
		secrets      []string
		commits      []string
		notify       bool
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Create a new incident report",
		Long: `Create a new security incident report with details about the leak.

Severity levels:
  critical  Immediate action required, production impact
  high      Urgent action needed, potential production impact
  medium    Action needed soon, limited impact
  low       Minor issue, fix when convenient

Incident types:
  secret-leak       Secrets found in code/logs/etc
  credential-exposure  Credentials exposed publicly
  policy-violation  Security policy was violated
  suspicious-activity  Unusual or suspicious behavior detected`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Interactive mode if no flags provided
			if title == "" && !cfg.NonInteractive {
				return interactiveIncidentReport(cfg, notify)
			}

			// Validate inputs
			if title == "" {
				return dserrors.UserError{
					Message:    "Incident title is required",
					Suggestion: "Provide --title or use interactive mode",
				}
			}

			if !isValidSeverity(severity) {
				return dserrors.UserError{
					Message:    fmt.Sprintf("Invalid severity: %s", severity),
					Suggestion: "Use one of: critical, high, medium, low",
				}
			}

			if !isValidIncidentType(incidentType) {
				return dserrors.UserError{
					Message:    fmt.Sprintf("Invalid incident type: %s", incidentType),
					Suggestion: "Use one of: secret-leak, credential-exposure, policy-violation, suspicious-activity",
				}
			}

			// Create incident manager
			manager := incident.NewManager(".")

			// Build details
			details := make(map[string]string)
			if len(files) > 0 {
				details["affected_files_count"] = fmt.Sprintf("%d", len(files))
			}
			if len(secrets) > 0 {
				details["affected_secrets_count"] = fmt.Sprintf("%d", len(secrets))
			}
			if len(commits) > 0 {
				details["affected_commits_count"] = fmt.Sprintf("%d", len(commits))
			}

			// Create report
			report, err := manager.CreateReport(incidentType, severity, title, description, details)
			if err != nil {
				return fmt.Errorf("failed to create incident report: %w", err)
			}

			// Add affected resources
			report.AffectedFiles = files
			report.AffectedSecrets = secrets
			report.AffectedCommits = commits

			// Add standard actions based on type
			report.ActionsRequired = getStandardActions(incidentType, severity)

			// Save updated report
			if err := manager.SaveReport(report); err != nil {
				return fmt.Errorf("failed to save report: %w", err)
			}

			fmt.Printf("✅ Created incident report: %s\n", report.ID)
			fmt.Printf("   Title: %s\n", report.Title)
			fmt.Printf("   Severity: %s\n", report.Severity)
			fmt.Printf("   Type: %s\n", report.Type)

			// Send notifications if requested
			if notify {
				if err := sendIncidentNotifications(cfg, manager, report); err != nil {
					fmt.Printf("\n⚠️  Failed to send notifications: %v\n", err)
				}
			}

			fmt.Println("\nNext steps:")
			fmt.Printf("  • View details: dsops leak show %s\n", report.ID)
			fmt.Printf("  • Update status: dsops leak update %s\n", report.ID)
			fmt.Printf("  • Mark resolved: dsops leak resolve %s\n", report.ID)

			return nil
		},
	}

	cmd.Flags().StringVarP(&incidentType, "type", "t", "secret-leak", "Type of incident")
	cmd.Flags().StringVarP(&severity, "severity", "s", "high", "Severity level (critical|high|medium|low)")
	cmd.Flags().StringVar(&title, "title", "", "Incident title")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Incident description")
	cmd.Flags().StringArrayVar(&files, "file", nil, "Affected file paths")
	cmd.Flags().StringArrayVar(&secrets, "secret", nil, "Affected secret names")
	cmd.Flags().StringArrayVar(&commits, "commit", nil, "Affected commit hashes")
	cmd.Flags().BoolVarP(&notify, "notify", "n", false, "Send notifications (Slack, GitHub)")

	return cmd
}

func NewLeakListCommand(cfg *config.Config) *cobra.Command {
	var (
		showAll      bool
		filterType   string
		filterStatus string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List incident reports",
		Long:  `List all security incident reports with filtering options.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := incident.NewManager(".")

			// Get reports
			reports, err := manager.ListReports()
			if err != nil {
				return fmt.Errorf("failed to list incidents: %w", err)
			}

			if len(reports) == 0 {
				fmt.Println("No incident reports found")
				return nil
			}

			// Filter reports
			var filtered []*incident.Report
			for _, report := range reports {
				// Filter by status
				if !showAll && report.Status == "resolved" {
					continue
				}
				if filterStatus != "" && report.Status != filterStatus {
					continue
				}

				// Filter by type
				if filterType != "" && report.Type != filterType {
					continue
				}

				filtered = append(filtered, report)
			}

			if len(filtered) == 0 {
				fmt.Println("No incidents match the filter criteria")
				return nil
			}

			// Display reports
			fmt.Printf("%-20s %-15s %-10s %-10s %s\n", "ID", "TYPE", "SEVERITY", "STATUS", "TITLE")
			fmt.Println(strings.Repeat("-", 80))

			for _, report := range filtered {
				fmt.Printf("%-20s %-15s %-10s %-10s %s\n",
					report.ID,
					report.Type,
					report.Severity,
					report.Status,
					report.Title,
				)
			}

			fmt.Printf("\nTotal: %d incidents\n", len(filtered))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all incidents including resolved")
	cmd.Flags().StringVar(&filterType, "type", "", "Filter by incident type")
	cmd.Flags().StringVar(&filterStatus, "status", "", "Filter by status (open|investigating|resolved)")

	return cmd
}

func NewLeakShowCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [incident-id]",
		Short: "Show incident details",
		Long:  `Display detailed information about a specific incident.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			incidentID := args[0]
			manager := incident.NewManager(".")

			report, err := manager.LoadReport(incidentID)
			if err != nil {
				return fmt.Errorf("failed to load incident: %w", err)
			}

			// Display report
			fmt.Printf("=== Incident Report: %s ===\n\n", report.ID)
			fmt.Printf("Title:       %s\n", report.Title)
			fmt.Printf("Type:        %s\n", report.Type)
			fmt.Printf("Severity:    %s\n", report.Severity)
			fmt.Printf("Status:      %s\n", report.Status)
			fmt.Printf("Created:     %s\n", report.Timestamp.Format(time.RFC3339))
			
			if report.ResolvedAt != nil {
				fmt.Printf("Resolved:    %s\n", report.ResolvedAt.Format(time.RFC3339))
			}

			fmt.Printf("\nDescription:\n%s\n", report.Description)

			// Show affected resources
			if len(report.AffectedFiles) > 0 {
				fmt.Printf("\nAffected Files (%d):\n", len(report.AffectedFiles))
				for _, file := range report.AffectedFiles {
					fmt.Printf("  • %s\n", file)
				}
			}

			if len(report.AffectedSecrets) > 0 {
				fmt.Printf("\nAffected Secrets (%d):\n", len(report.AffectedSecrets))
				for _, secret := range report.AffectedSecrets {
					fmt.Printf("  • %s\n", secret)
				}
			}

			if len(report.AffectedCommits) > 0 {
				fmt.Printf("\nAffected Commits (%d):\n", len(report.AffectedCommits))
				for _, commit := range report.AffectedCommits {
					fmt.Printf("  • %s\n", commit)
				}
			}

			// Show actions
			if len(report.ActionsRequired) > 0 {
				fmt.Printf("\nActions Required (%d):\n", len(report.ActionsRequired))
				for _, action := range report.ActionsRequired {
					fmt.Printf("  □ %s\n", action)
				}
			}

			if len(report.ActionsTaken) > 0 {
				fmt.Printf("\nActions Taken (%d):\n", len(report.ActionsTaken))
				for _, action := range report.ActionsTaken {
					fmt.Printf("  ✓ %s\n", action)
				}
			}

			// Show notifications
			if len(report.NotificationsSent) > 0 {
				fmt.Printf("\nNotifications Sent:\n")
				for _, notif := range report.NotificationsSent {
					status := "✓"
					if !notif.Success {
						status = "✗"
					}
					fmt.Printf("  %s %s - %s", status, notif.Channel, notif.Timestamp.Format("15:04:05"))
					if notif.Details != "" {
						fmt.Printf(" (%s)", notif.Details)
					}
					fmt.Println()
				}
			}

			// Show resolution
			if report.Status == "resolved" && report.ResolutionNotes != "" {
				fmt.Printf("\nResolution Notes:\n%s\n", report.ResolutionNotes)
			}

			return nil
		},
	}

	return cmd
}

func NewLeakUpdateCommand(cfg *config.Config) *cobra.Command {
	var (
		status       string
		addAction    []string
		addFile      []string
		addSecret    []string
		addCommit    []string
		notify       bool
	)

	cmd := &cobra.Command{
		Use:   "update [incident-id]",
		Short: "Update an incident",
		Long:  `Update an existing incident with new information or status changes.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			incidentID := args[0]
			manager := incident.NewManager(".")

			report, err := manager.LoadReport(incidentID)
			if err != nil {
				return fmt.Errorf("failed to load incident: %w", err)
			}

			// Update status
			if status != "" {
				if !isValidStatus(status) {
					return dserrors.UserError{
						Message:    fmt.Sprintf("Invalid status: %s", status),
						Suggestion: "Use one of: open, investigating, resolved",
					}
				}
				report.Status = status
			}

			// Add new resources
			report.AffectedFiles = append(report.AffectedFiles, addFile...)
			report.AffectedSecrets = append(report.AffectedSecrets, addSecret...)
			report.AffectedCommits = append(report.AffectedCommits, addCommit...)

			// Add actions taken
			report.ActionsTaken = append(report.ActionsTaken, addAction...)

			// Save updated report
			if err := manager.UpdateReport(report); err != nil {
				return fmt.Errorf("failed to update report: %w", err)
			}

			fmt.Printf("✅ Updated incident: %s\n", report.ID)

			// Send notifications if requested
			if notify {
				if err := sendIncidentNotifications(cfg, manager, report); err != nil {
					fmt.Printf("\n⚠️  Failed to send notifications: %v\n", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "Update status (open|investigating|resolved)")
	cmd.Flags().StringArrayVar(&addAction, "action", nil, "Add action taken")
	cmd.Flags().StringArrayVar(&addFile, "file", nil, "Add affected file")
	cmd.Flags().StringArrayVar(&addSecret, "secret", nil, "Add affected secret")
	cmd.Flags().StringArrayVar(&addCommit, "commit", nil, "Add affected commit")
	cmd.Flags().BoolVarP(&notify, "notify", "n", false, "Send update notifications")

	return cmd
}

func NewLeakResolveCommand(cfg *config.Config) *cobra.Command {
	var (
		notes  string
		notify bool
	)

	cmd := &cobra.Command{
		Use:   "resolve [incident-id]",
		Short: "Mark an incident as resolved",
		Long:  `Mark an incident as resolved with resolution notes.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			incidentID := args[0]
			manager := incident.NewManager(".")

			report, err := manager.LoadReport(incidentID)
			if err != nil {
				return fmt.Errorf("failed to load incident: %w", err)
			}

			if report.Status == "resolved" {
				fmt.Printf("Incident %s is already resolved\n", incidentID)
				return nil
			}

			// Interactive resolution notes if not provided
			if notes == "" && !cfg.NonInteractive {
				fmt.Print("Resolution notes: ")
				reader := bufio.NewReader(os.Stdin)
				notes, _ = reader.ReadString('\n')
				notes = strings.TrimSpace(notes)
			}

			// Resolve incident
			if err := manager.ResolveReport(report, notes); err != nil {
				return fmt.Errorf("failed to resolve incident: %w", err)
			}

			fmt.Printf("✅ Resolved incident: %s\n", report.ID)

			// Send notifications if requested
			if notify {
				if err := sendIncidentNotifications(cfg, manager, report); err != nil {
					fmt.Printf("\n⚠️  Failed to send notifications: %v\n", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&notes, "notes", "n", "", "Resolution notes")
	cmd.Flags().BoolVar(&notify, "notify", false, "Send resolution notifications")

	return cmd
}

// Helper functions

func isValidSeverity(severity string) bool {
	switch severity {
	case "critical", "high", "medium", "low":
		return true
	}
	return false
}

func isValidIncidentType(incidentType string) bool {
	switch incidentType {
	case "secret-leak", "credential-exposure", "policy-violation", "suspicious-activity":
		return true
	}
	return false
}

func isValidStatus(status string) bool {
	switch status {
	case "open", "investigating", "resolved":
		return true
	}
	return false
}

func getStandardActions(incidentType, severity string) []string {
	actions := []string{}

	switch incidentType {
	case "secret-leak":
		actions = append(actions,
			"Identify all locations where the secret was exposed",
			"Rotate the compromised credential immediately",
			"Update all systems using the old credential",
			"Review access logs for unauthorized usage",
			"Remove secret from git history if committed",
		)
	case "credential-exposure":
		actions = append(actions,
			"Disable or rotate the exposed credential",
			"Audit usage of the credential",
			"Identify how the credential was exposed",
			"Update security policies to prevent recurrence",
		)
	case "policy-violation":
		actions = append(actions,
			"Document the policy violation details",
			"Identify root cause of the violation",
			"Update policies or controls as needed",
			"Communicate changes to relevant teams",
		)
	case "suspicious-activity":
		actions = append(actions,
			"Investigate the suspicious activity",
			"Check for additional indicators of compromise",
			"Review security logs and alerts",
			"Update monitoring and detection rules",
		)
	}

	// Add severity-specific actions
	if severity == "critical" || severity == "high" {
		actions = append([]string{"Notify security team immediately"}, actions...)
	}

	return actions
}

func interactiveIncidentReport(cfg *config.Config, notify bool) error {
	fmt.Println("🚨 Security Incident Report")
	fmt.Println("==========================")
	fmt.Println()

	// Get incident details interactively
	var title, incidentType, severity, description string

	fmt.Print("Title: ")
	reader := bufio.NewReader(os.Stdin)
	title, _ = reader.ReadString('\n')
	title = strings.TrimSpace(title)

	fmt.Println("\nIncident Types:")
	fmt.Println("  1. secret-leak         - Secrets found in code/logs")
	fmt.Println("  2. credential-exposure - Credentials exposed publicly")
	fmt.Println("  3. policy-violation    - Security policy violated")
	fmt.Println("  4. suspicious-activity - Unusual behavior detected")
	fmt.Print("\nSelect type (1-4): ")
	var typeChoice int
	_, _ = fmt.Scanln(&typeChoice)

	switch typeChoice {
	case 1:
		incidentType = "secret-leak"
	case 2:
		incidentType = "credential-exposure"
	case 3:
		incidentType = "policy-violation"
	case 4:
		incidentType = "suspicious-activity"
	default:
		incidentType = "secret-leak"
	}

	fmt.Println("\nSeverity Levels:")
	fmt.Println("  1. critical - Immediate action required")
	fmt.Println("  2. high     - Urgent action needed")
	fmt.Println("  3. medium   - Action needed soon")
	fmt.Println("  4. low      - Minor issue")
	fmt.Print("\nSelect severity (1-4): ")
	var severityChoice int
	_, _ = fmt.Scanln(&severityChoice)

	switch severityChoice {
	case 1:
		severity = "critical"
	case 2:
		severity = "high"
	case 3:
		severity = "medium"
	case 4:
		severity = "low"
	default:
		severity = "high"
	}

	fmt.Print("\nDescription: ")
	description, _ = reader.ReadString('\n')
	description = strings.TrimSpace(description)

	// Create the incident
	manager := incident.NewManager(".")
	report, err := manager.CreateReport(incidentType, severity, title, description, nil)
	if err != nil {
		return fmt.Errorf("failed to create incident: %w", err)
	}

	// Add standard actions
	report.ActionsRequired = getStandardActions(incidentType, severity)

	// Save report
	if err := manager.SaveReport(report); err != nil {
		return fmt.Errorf("failed to save report: %w", err)
	}

	fmt.Printf("\n✅ Created incident report: %s\n", report.ID)

	// Send notifications if requested
	if notify {
		if err := sendIncidentNotifications(cfg, manager, report); err != nil {
			fmt.Printf("\n⚠️  Failed to send notifications: %v\n", err)
		}
	}

	return nil
}

func sendIncidentNotifications(cfg *config.Config, manager *incident.Manager, report *incident.Report) error {
	// Load notification config from dsops.yaml
	if cfg.Definition == nil || cfg.Definition.Policies == nil {
		return fmt.Errorf("no notification configuration found")
	}

	// TODO: Add notification config to policies section
	// For now, we'll use environment variables
	
	notifConfig := incident.NotificationConfig{}
	
	// Check for Slack webhook
	if webhook := os.Getenv("DSOPS_SLACK_WEBHOOK"); webhook != "" {
		notifConfig.Slack = &incident.SlackConfig{
			WebhookURL: webhook,
		}
	}
	
	// Check for GitHub config
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		owner := os.Getenv("GITHUB_OWNER")
		repo := os.Getenv("GITHUB_REPO")
		if owner != "" && repo != "" {
			notifConfig.GitHub = &incident.GitHubConfig{
				Token:      token,
				Owner:      owner,
				Repository: repo,
				Labels:     []string{"security", "incident"},
			}
		}
	}
	
	// Send notifications
	notifier := incident.NewNotifier(notifConfig)
	records := notifier.SendNotifications(report)
	
	// Update report with notification records
	for _, record := range records {
		if err := manager.AddNotification(report, record.Channel, record.Success, record.Details); err != nil {
			return fmt.Errorf("failed to record notification: %w", err)
		}
	}
	
	// Report results
	successCount := 0
	for _, record := range records {
		if record.Success {
			successCount++
			fmt.Printf("✅ Sent notification to %s\n", record.Channel)
		} else {
			fmt.Printf("❌ Failed to notify %s: %s\n", record.Channel, record.Details)
		}
	}
	
	if successCount == 0 && len(records) > 0 {
		return fmt.Errorf("all notifications failed")
	}
	
	return nil
}