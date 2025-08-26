package incident

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// NotificationConfig holds configuration for incident notifications
type NotificationConfig struct {
	Slack  *SlackConfig  `yaml:"slack,omitempty"`
	GitHub *GitHubConfig `yaml:"github,omitempty"`
}

// SlackConfig holds Slack webhook configuration
type SlackConfig struct {
	WebhookURL string `yaml:"webhook_url"`
	Channel    string `yaml:"channel,omitempty"`
	Username   string `yaml:"username,omitempty"`
}

// GitHubConfig holds GitHub integration configuration
type GitHubConfig struct {
	Token      string   `yaml:"token"`       // GitHub personal access token
	Owner      string   `yaml:"owner"`       // Repository owner
	Repository string   `yaml:"repository"`  // Repository name
	Labels     []string `yaml:"labels,omitempty"` // Labels to add to issues
}

// Notifier handles sending incident notifications
type Notifier struct {
	config NotificationConfig
}

// NewNotifier creates a new notifier
func NewNotifier(config NotificationConfig) *Notifier {
	return &Notifier{config: config}
}

// SendNotifications sends notifications to all configured channels
func (n *Notifier) SendNotifications(report *Report) []NotificationRecord {
	var records []NotificationRecord
	
	// Send to Slack
	if n.config.Slack != nil {
		record := n.sendSlackNotification(report)
		records = append(records, record)
	}
	
	// Send to GitHub
	if n.config.GitHub != nil {
		record := n.sendGitHubNotification(report)
		records = append(records, record)
	}
	
	return records
}

// sendSlackNotification sends a notification to Slack
func (n *Notifier) sendSlackNotification(report *Report) NotificationRecord {
	record := NotificationRecord{
		Channel:   "slack",
		Timestamp: time.Now(),
	}
	
	// Override webhook URL from environment if set
	webhookURL := n.config.Slack.WebhookURL
	if envURL := os.Getenv("DSOPS_SLACK_WEBHOOK"); envURL != "" {
		webhookURL = envURL
	}
	
	if webhookURL == "" {
		record.Success = false
		record.Details = "No Slack webhook URL configured"
		return record
	}
	
	// Build Slack message
	message := n.buildSlackMessage(report)
	
	// Send to Slack
	data, err := json.Marshal(message)
	if err != nil {
		record.Success = false
		record.Details = fmt.Sprintf("Failed to marshal message: %v", err)
		return record
	}
	
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		record.Success = false
		record.Details = fmt.Sprintf("Failed to send: %v", err)
		return record
	}
	defer func() { _ = resp.Body.Close() }()
	
	if resp.StatusCode != http.StatusOK {
		record.Success = false
		record.Details = fmt.Sprintf("Slack returned status %d", resp.StatusCode)
		return record
	}
	
	record.Success = true
	return record
}

// buildSlackMessage creates a Slack message from an incident report
func (n *Notifier) buildSlackMessage(report *Report) map[string]interface{} {
	// Determine color based on severity
	color := "#808080" // gray for unknown
	switch report.Severity {
	case "critical":
		color = "#FF0000" // red
	case "high":
		color = "#FF8C00" // dark orange
	case "medium":
		color = "#FFD700" // gold
	case "low":
		color = "#00CED1" // dark turquoise
	}
	
	// Build fields
	var fields []map[string]interface{}
	fields = append(fields, map[string]interface{}{
		"title": "Incident ID",
		"value": report.ID,
		"short": true,
	})
	fields = append(fields, map[string]interface{}{
		"title": "Severity",
		"value": report.Severity,
		"short": true,
	})
	fields = append(fields, map[string]interface{}{
		"title": "Type",
		"value": report.Type,
		"short": true,
	})
	fields = append(fields, map[string]interface{}{
		"title": "Status",
		"value": report.Status,
		"short": true,
	})
	
	// Add affected resources
	if len(report.AffectedFiles) > 0 {
		fields = append(fields, map[string]interface{}{
			"title": "Affected Files",
			"value": strings.Join(report.AffectedFiles, "\n"),
			"short": false,
		})
	}
	
	// Add actions required
	if len(report.ActionsRequired) > 0 {
		fields = append(fields, map[string]interface{}{
			"title": "Actions Required",
			"value": "• " + strings.Join(report.ActionsRequired, "\n• "),
			"short": false,
		})
	}
	
	attachment := map[string]interface{}{
		"color":      color,
		"title":      fmt.Sprintf("🚨 Security Incident: %s", report.Title),
		"text":       report.Description,
		"fields":     fields,
		"footer":     "dsops incident response",
		"ts":         report.Timestamp.Unix(),
		"mrkdwn_in":  []string{"text", "fields"},
	}
	
	message := map[string]interface{}{
		"attachments": []interface{}{attachment},
	}
	
	// Add channel if configured
	if n.config.Slack.Channel != "" {
		message["channel"] = n.config.Slack.Channel
	}
	
	// Add username if configured
	if n.config.Slack.Username != "" {
		message["username"] = n.config.Slack.Username
	}
	
	return message
}

// sendGitHubNotification creates a GitHub issue for the incident
func (n *Notifier) sendGitHubNotification(report *Report) NotificationRecord {
	record := NotificationRecord{
		Channel:   "github",
		Timestamp: time.Now(),
	}
	
	// Get token from config or environment
	token := n.config.GitHub.Token
	if envToken := os.Getenv("GITHUB_TOKEN"); envToken != "" {
		token = envToken
	}
	
	if token == "" {
		record.Success = false
		record.Details = "No GitHub token configured"
		return record
	}
	
	// Build issue body
	body := n.buildGitHubIssueBody(report)
	
	// Create issue
	issue := map[string]interface{}{
		"title":  fmt.Sprintf("[Security Incident] %s", report.Title),
		"body":   body,
		"labels": n.config.GitHub.Labels,
	}
	
	data, err := json.Marshal(issue)
	if err != nil {
		record.Success = false
		record.Details = fmt.Sprintf("Failed to marshal issue: %v", err)
		return record
	}
	
	// Create request
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", 
		n.config.GitHub.Owner, n.config.GitHub.Repository)
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		record.Success = false
		record.Details = fmt.Sprintf("Failed to create request: %v", err)
		return record
	}
	
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")
	
	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		record.Success = false
		record.Details = fmt.Sprintf("Failed to send request: %v", err)
		return record
	}
	defer func() { _ = resp.Body.Close() }()
	
	if resp.StatusCode != http.StatusCreated {
		record.Success = false
		record.Details = fmt.Sprintf("GitHub returned status %d", resp.StatusCode)
		return record
	}
	
	// Parse response to get issue number
	var respData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err == nil {
		if number, ok := respData["number"].(float64); ok {
			record.Details = fmt.Sprintf("Created issue #%d", int(number))
		}
	}
	
	record.Success = true
	return record
}

// buildGitHubIssueBody creates a GitHub issue body from an incident report
func (n *Notifier) buildGitHubIssueBody(report *Report) string {
	var body strings.Builder
	
	body.WriteString("## Security Incident Report\n\n")
	
	// Metadata table
	body.WriteString("| Field | Value |\n")
	body.WriteString("|-------|-------|\n")
	body.WriteString(fmt.Sprintf("| **Incident ID** | %s |\n", report.ID))
	body.WriteString(fmt.Sprintf("| **Type** | %s |\n", report.Type))
	body.WriteString(fmt.Sprintf("| **Severity** | %s |\n", report.Severity))
	body.WriteString(fmt.Sprintf("| **Status** | %s |\n", report.Status))
	body.WriteString(fmt.Sprintf("| **Timestamp** | %s |\n", report.Timestamp.Format(time.RFC3339)))
	body.WriteString("\n")
	
	// Description
	body.WriteString("### Description\n\n")
	body.WriteString(report.Description + "\n\n")
	
	// Details
	if len(report.Details) > 0 {
		body.WriteString("### Details\n\n")
		for key, value := range report.Details {
			body.WriteString(fmt.Sprintf("- **%s**: %s\n", key, value))
		}
		body.WriteString("\n")
	}
	
	// Affected Resources
	if len(report.AffectedFiles) > 0 || len(report.AffectedSecrets) > 0 || len(report.AffectedCommits) > 0 {
		body.WriteString("### Affected Resources\n\n")
		
		if len(report.AffectedFiles) > 0 {
			body.WriteString("**Files:**\n")
			for _, file := range report.AffectedFiles {
				body.WriteString(fmt.Sprintf("- `%s`\n", file))
			}
			body.WriteString("\n")
		}
		
		if len(report.AffectedSecrets) > 0 {
			body.WriteString("**Secrets:**\n")
			for _, secret := range report.AffectedSecrets {
				body.WriteString(fmt.Sprintf("- %s\n", secret))
			}
			body.WriteString("\n")
		}
		
		if len(report.AffectedCommits) > 0 {
			body.WriteString("**Commits:**\n")
			for _, commit := range report.AffectedCommits {
				body.WriteString(fmt.Sprintf("- %s\n", commit))
			}
			body.WriteString("\n")
		}
	}
	
	// Actions Required
	if len(report.ActionsRequired) > 0 {
		body.WriteString("### Actions Required\n\n")
		for _, action := range report.ActionsRequired {
			body.WriteString(fmt.Sprintf("- [ ] %s\n", action))
		}
		body.WriteString("\n")
	}
	
	// Actions Taken
	if len(report.ActionsTaken) > 0 {
		body.WriteString("### Actions Taken\n\n")
		for _, action := range report.ActionsTaken {
			body.WriteString(fmt.Sprintf("- [x] %s\n", action))
		}
		body.WriteString("\n")
	}
	
	body.WriteString("---\n")
	body.WriteString("*This issue was automatically created by dsops incident response*\n")
	
	return body.String()
}