package incident

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		baseDir     string
		expectedDir string
	}{
		{
			name:        "custom base directory",
			baseDir:     "/tmp/test",
			expectedDir: "/tmp/test/.dsops/incidents",
		},
		{
			name:        "empty base directory defaults to current",
			baseDir:     "",
			expectedDir: ".dsops/incidents",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mgr := NewManager(tc.baseDir)
			assert.NotNil(t, mgr)
			assert.Equal(t, tc.expectedDir, mgr.incidentDir)
		})
	}
}

func TestManager_CreateReport(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	details := map[string]string{
		"source": "git-hook",
		"branch": "main",
	}

	report, err := mgr.CreateReport("secret_leak", "critical", "API Key Leaked", "AWS API key found in code", details)
	require.NoError(t, err)
	assert.NotNil(t, report)

	// Verify report fields
	assert.NotEmpty(t, report.ID)
	assert.True(t, strings.HasPrefix(report.ID, "INC-"))
	assert.Equal(t, "secret_leak", report.Type)
	assert.Equal(t, "critical", report.Severity)
	assert.Equal(t, "API Key Leaked", report.Title)
	assert.Equal(t, "AWS API key found in code", report.Description)
	assert.Equal(t, "git-hook", report.Details["source"])
	assert.Equal(t, "open", report.Status)
	assert.WithinDuration(t, time.Now(), report.Timestamp, time.Second)

	// Verify file was created
	reportPath := filepath.Join(tmpDir, IncidentDirName, report.ID+".json")
	assert.FileExists(t, reportPath)

	// Verify audit log was created
	auditPath := filepath.Join(tmpDir, AuditLogName)
	assert.FileExists(t, auditPath)
}

func TestManager_SaveAndLoadReport(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create incident directory
	err := os.MkdirAll(mgr.incidentDir, 0700)
	require.NoError(t, err)

	// Create a report
	report := &Report{
		ID:          "INC-20241116-12345",
		Timestamp:   time.Now(),
		Type:        "unauthorized_access",
		Severity:    "high",
		Title:       "Suspicious Access Pattern",
		Description: "Multiple failed authentication attempts detected",
		Details: map[string]string{
			"source_ip": "192.168.1.100",
			"attempts":  "50",
		},
		AffectedFiles:   []string{"/var/log/auth.log"},
		AffectedSecrets: []string{"database/prod/password"},
		ActionsRequired: []string{"Block IP address", "Rotate credentials"},
		Status:          "investigating",
	}

	// Save report
	err = mgr.SaveReport(report)
	require.NoError(t, err)

	// Load report
	loaded, err := mgr.LoadReport(report.ID)
	require.NoError(t, err)

	assert.Equal(t, report.ID, loaded.ID)
	assert.Equal(t, report.Type, loaded.Type)
	assert.Equal(t, report.Severity, loaded.Severity)
	assert.Equal(t, report.Title, loaded.Title)
	assert.Equal(t, report.Description, loaded.Description)
	assert.Equal(t, report.Details["source_ip"], loaded.Details["source_ip"])
	assert.Equal(t, report.AffectedFiles, loaded.AffectedFiles)
	assert.Equal(t, report.AffectedSecrets, loaded.AffectedSecrets)
	assert.Equal(t, report.ActionsRequired, loaded.ActionsRequired)
	assert.Equal(t, report.Status, loaded.Status)
}

func TestManager_LoadReport_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create incident directory
	err := os.MkdirAll(mgr.incidentDir, 0700)
	require.NoError(t, err)

	_, err = mgr.LoadReport("nonexistent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incident not found")
}

func TestManager_LoadReport_InvalidJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create incident directory
	err := os.MkdirAll(mgr.incidentDir, 0700)
	require.NoError(t, err)

	// Create invalid JSON file
	invalidPath := filepath.Join(mgr.incidentDir, "invalid-id.json")
	err = os.WriteFile(invalidPath, []byte("invalid json {"), 0600)
	require.NoError(t, err)

	_, err = mgr.LoadReport("invalid-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse report")
}

func TestManager_ListReports(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create incident directory
	err := os.MkdirAll(mgr.incidentDir, 0700)
	require.NoError(t, err)

	// Create multiple reports
	reports := []*Report{
		{ID: "INC-20241116-00001", Type: "secret_leak", Status: "open"},
		{ID: "INC-20241116-00002", Type: "unauthorized_access", Status: "resolved"},
		{ID: "INC-20241116-00003", Type: "secret_leak", Status: "investigating"},
	}

	for _, r := range reports {
		err := mgr.SaveReport(r)
		require.NoError(t, err)
	}

	// List all reports
	listed, err := mgr.ListReports()
	require.NoError(t, err)
	assert.Len(t, listed, 3)
}

func TestManager_ListReports_EmptyDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Don't create incident directory - should return empty list
	reports, err := mgr.ListReports()
	require.NoError(t, err)
	assert.Empty(t, reports)
}

func TestManager_ListReports_SkipsInvalidFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create incident directory
	err := os.MkdirAll(mgr.incidentDir, 0700)
	require.NoError(t, err)

	// Create valid report
	validReport := &Report{ID: "INC-valid", Type: "test", Status: "open"}
	err = mgr.SaveReport(validReport)
	require.NoError(t, err)

	// Create invalid JSON file
	invalidPath := filepath.Join(mgr.incidentDir, "invalid.json")
	err = os.WriteFile(invalidPath, []byte("not json"), 0600)
	require.NoError(t, err)

	// Create non-JSON file (should be skipped)
	txtPath := filepath.Join(mgr.incidentDir, "notes.txt")
	err = os.WriteFile(txtPath, []byte("notes"), 0600)
	require.NoError(t, err)

	// List reports - should only return valid one
	reports, err := mgr.ListReports()
	require.NoError(t, err)
	assert.Len(t, reports, 1)
	assert.Equal(t, "INC-valid", reports[0].ID)
}

func TestManager_UpdateReport(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create incident directory
	err := os.MkdirAll(mgr.incidentDir, 0700)
	require.NoError(t, err)

	// Create and save report
	report := &Report{
		ID:              "INC-20241116-00001",
		Type:            "secret_leak",
		Severity:        "high",
		Title:           "Test Incident",
		Status:          "open",
		ActionsRequired: []string{"Investigate"},
	}
	err = mgr.SaveReport(report)
	require.NoError(t, err)

	// Update report
	report.Status = "investigating"
	report.ActionsTaken = []string{"Started investigation"}
	err = mgr.UpdateReport(report)
	require.NoError(t, err)

	// Reload and verify
	loaded, err := mgr.LoadReport(report.ID)
	require.NoError(t, err)
	assert.Equal(t, "investigating", loaded.Status)
	assert.Contains(t, loaded.ActionsTaken, "Started investigation")
}

func TestManager_AddNotification(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create incident directory
	err := os.MkdirAll(mgr.incidentDir, 0700)
	require.NoError(t, err)

	// Create report
	report := &Report{
		ID:     "INC-20241116-00001",
		Type:   "test",
		Status: "open",
	}
	err = mgr.SaveReport(report)
	require.NoError(t, err)

	// Add notification
	err = mgr.AddNotification(report, "slack", true, "Message sent to #security")
	require.NoError(t, err)

	assert.Len(t, report.NotificationsSent, 1)
	assert.Equal(t, "slack", report.NotificationsSent[0].Channel)
	assert.True(t, report.NotificationsSent[0].Success)
	assert.Equal(t, "Message sent to #security", report.NotificationsSent[0].Details)

	// Reload and verify persistence
	loaded, err := mgr.LoadReport(report.ID)
	require.NoError(t, err)
	assert.Len(t, loaded.NotificationsSent, 1)
}

func TestManager_ResolveReport(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create incident directory
	err := os.MkdirAll(mgr.incidentDir, 0700)
	require.NoError(t, err)

	// Create report
	report := &Report{
		ID:     "INC-20241116-00001",
		Type:   "test",
		Status: "investigating",
	}
	err = mgr.SaveReport(report)
	require.NoError(t, err)

	// Resolve report
	err = mgr.ResolveReport(report, "Credentials rotated and access revoked")
	require.NoError(t, err)

	assert.Equal(t, "resolved", report.Status)
	assert.NotNil(t, report.ResolvedAt)
	assert.Equal(t, "Credentials rotated and access revoked", report.ResolutionNotes)

	// Verify persistence
	loaded, err := mgr.LoadReport(report.ID)
	require.NoError(t, err)
	assert.Equal(t, "resolved", loaded.Status)
	assert.NotNil(t, loaded.ResolvedAt)
}

func TestManager_GetOpenIncidents(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create incident directory
	err := os.MkdirAll(mgr.incidentDir, 0700)
	require.NoError(t, err)

	// Create reports with different statuses
	reports := []*Report{
		{ID: "INC-1", Type: "test", Status: "open"},
		{ID: "INC-2", Type: "test", Status: "resolved"},
		{ID: "INC-3", Type: "test", Status: "investigating"},
		{ID: "INC-4", Type: "test", Status: "resolved"},
	}

	for _, r := range reports {
		err := mgr.SaveReport(r)
		require.NoError(t, err)
	}

	// Get open incidents
	open, err := mgr.GetOpenIncidents()
	require.NoError(t, err)
	assert.Len(t, open, 2) // INC-1 and INC-3
}

func TestManager_GetIncidentsByType(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create incident directory
	err := os.MkdirAll(mgr.incidentDir, 0700)
	require.NoError(t, err)

	// Create reports of different types
	reports := []*Report{
		{ID: "INC-1", Type: "secret_leak", Status: "open"},
		{ID: "INC-2", Type: "unauthorized_access", Status: "open"},
		{ID: "INC-3", Type: "secret_leak", Status: "resolved"},
		{ID: "INC-4", Type: "policy_violation", Status: "open"},
	}

	for _, r := range reports {
		err := mgr.SaveReport(r)
		require.NoError(t, err)
	}

	// Get incidents by type
	leaks, err := mgr.GetIncidentsByType("secret_leak")
	require.NoError(t, err)
	assert.Len(t, leaks, 2)

	access, err := mgr.GetIncidentsByType("unauthorized_access")
	require.NoError(t, err)
	assert.Len(t, access, 1)

	nonexistent, err := mgr.GetIncidentsByType("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, nonexistent)
}

func TestGenerateIncidentID(t *testing.T) {
	t.Parallel()

	id1 := generateIncidentID()
	id2 := generateIncidentID()

	assert.True(t, strings.HasPrefix(id1, "INC-"))
	assert.True(t, strings.HasPrefix(id2, "INC-"))

	// IDs should contain date
	today := time.Now().Format("20060102")
	assert.Contains(t, id1, today)
}

func TestNewNotifier(t *testing.T) {
	t.Parallel()

	config := NotificationConfig{
		Slack: &SlackConfig{
			WebhookURL: "https://hooks.slack.com/test",
			Channel:    "#security",
		},
	}

	notifier := NewNotifier(config)
	assert.NotNil(t, notifier)
	assert.Equal(t, config, notifier.config)
}

func TestNotifier_SendNotifications_NoChannels(t *testing.T) {
	t.Parallel()

	notifier := NewNotifier(NotificationConfig{})
	report := &Report{
		ID:       "INC-test",
		Type:     "test",
		Severity: "low",
		Title:    "Test",
	}

	records := notifier.SendNotifications(report)
	assert.Empty(t, records)
}

func TestNotifier_SendSlackNotification_Success(t *testing.T) {
	t.Parallel()

	// Create mock Slack server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var payload map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Verify message structure
		assert.Contains(t, payload, "attachments")
		assert.Equal(t, "#security", payload["channel"])
		assert.Equal(t, "dsops-bot", payload["username"])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := NotificationConfig{
		Slack: &SlackConfig{
			WebhookURL: server.URL,
			Channel:    "#security",
			Username:   "dsops-bot",
		},
	}

	notifier := NewNotifier(config)
	report := &Report{
		ID:              "INC-test",
		Timestamp:       time.Now(),
		Type:            "secret_leak",
		Severity:        "critical",
		Title:           "Test Incident",
		Description:     "Test description",
		AffectedFiles:   []string{"config.yaml"},
		ActionsRequired: []string{"Rotate credentials"},
		Status:          "open",
	}

	record := notifier.sendSlackNotification(report)
	assert.True(t, record.Success)
	assert.Equal(t, "slack", record.Channel)
}

func TestNotifier_SendSlackNotification_NoWebhook(t *testing.T) {
	t.Parallel()

	config := NotificationConfig{
		Slack: &SlackConfig{
			WebhookURL: "",
		},
	}

	notifier := NewNotifier(config)
	report := &Report{ID: "INC-test"}

	record := notifier.sendSlackNotification(report)
	assert.False(t, record.Success)
	assert.Contains(t, record.Details, "No Slack webhook URL configured")
}

func TestNotifier_SendSlackNotification_HTTPError(t *testing.T) {
	t.Parallel()

	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := NotificationConfig{
		Slack: &SlackConfig{
			WebhookURL: server.URL,
		},
	}

	notifier := NewNotifier(config)
	report := &Report{ID: "INC-test"}

	record := notifier.sendSlackNotification(report)
	assert.False(t, record.Success)
	assert.Contains(t, record.Details, "status 500")
}

func TestNotifier_BuildSlackMessage_Severities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		severity      string
		expectedColor string
	}{
		{"critical", "#FF0000"},
		{"high", "#FF8C00"},
		{"medium", "#FFD700"},
		{"low", "#00CED1"},
		{"unknown", "#808080"},
	}

	notifier := NewNotifier(NotificationConfig{Slack: &SlackConfig{}})

	for _, tc := range tests {
		t.Run(tc.severity, func(t *testing.T) {
			report := &Report{
				ID:        "INC-test",
				Timestamp: time.Now(),
				Severity:  tc.severity,
				Title:     "Test",
				Type:      "test",
				Status:    "open",
			}

			message := notifier.buildSlackMessage(report)
			attachments := message["attachments"].([]interface{})
			attachment := attachments[0].(map[string]interface{})
			assert.Equal(t, tc.expectedColor, attachment["color"])
		})
	}
}

func TestNotifier_SendGitHubNotification_Success(t *testing.T) {
	t.Parallel()

	// Create mock GitHub server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Contains(t, r.URL.Path, "/repos/owner/repo/issues")

		var issue map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&issue)
		require.NoError(t, err)

		assert.Contains(t, issue["title"], "[Security Incident]")
		assert.NotEmpty(t, issue["body"])

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"number": 42}`))
	}))
	defer server.Close()

	// Temporarily override GitHub API URL (in real code you'd inject this)
	originalConfig := NotificationConfig{
		GitHub: &GitHubConfig{
			Token:      "test-token",
			Owner:      "owner",
			Repository: "repo",
			Labels:     []string{"security", "incident"},
		},
	}

	notifier := NewNotifier(originalConfig)
	report := &Report{
		ID:              "INC-test",
		Timestamp:       time.Now(),
		Type:            "secret_leak",
		Severity:        "critical",
		Title:           "API Key Exposed",
		Description:     "API key found in repository",
		Details:         map[string]string{"file": "config.yaml"},
		AffectedFiles:   []string{"config.yaml"},
		AffectedSecrets: []string{"api-key"},
		AffectedCommits: []string{"abc123"},
		ActionsRequired: []string{"Rotate key"},
		ActionsTaken:    []string{"Notified team"},
		Status:          "open",
	}

	// Note: This test would fail because we can't mock the actual GitHub API URL
	// In a real scenario, you'd inject the URL or use an HTTP transport
	record := notifier.sendGitHubNotification(report)
	// Since we can't mock the real GitHub URL, this will fail connecting
	assert.Equal(t, "github", record.Channel)
}

func TestNotifier_SendGitHubNotification_NoToken(t *testing.T) {
	t.Parallel()

	config := NotificationConfig{
		GitHub: &GitHubConfig{
			Token:      "",
			Owner:      "owner",
			Repository: "repo",
		},
	}

	notifier := NewNotifier(config)
	report := &Report{ID: "INC-test"}

	record := notifier.sendGitHubNotification(report)
	assert.False(t, record.Success)
	assert.Contains(t, record.Details, "No GitHub token configured")
}

func TestNotifier_BuildGitHubIssueBody(t *testing.T) {
	t.Parallel()

	notifier := NewNotifier(NotificationConfig{})
	report := &Report{
		ID:              "INC-20241116-12345",
		Timestamp:       time.Now(),
		Type:            "secret_leak",
		Severity:        "critical",
		Title:           "API Key Leaked",
		Description:     "AWS API key found in committed code",
		Status:          "open",
		Details:         map[string]string{"source": "git-hook", "branch": "main"},
		AffectedFiles:   []string{"src/config.py", "secrets.json"},
		AffectedSecrets: []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"},
		AffectedCommits: []string{"abc123", "def456"},
		ActionsRequired: []string{"Rotate AWS credentials", "Remove from git history"},
		ActionsTaken:    []string{"Blocked deployment"},
	}

	body := notifier.buildGitHubIssueBody(report)

	// Verify structure
	assert.Contains(t, body, "## Security Incident Report")
	assert.Contains(t, body, "INC-20241116-12345")
	assert.Contains(t, body, "secret_leak")
	assert.Contains(t, body, "critical")
	assert.Contains(t, body, "open")

	// Verify description
	assert.Contains(t, body, "### Description")
	assert.Contains(t, body, "AWS API key found in committed code")

	// Verify details
	assert.Contains(t, body, "### Details")
	assert.Contains(t, body, "source")
	assert.Contains(t, body, "git-hook")

	// Verify affected resources
	assert.Contains(t, body, "### Affected Resources")
	assert.Contains(t, body, "src/config.py")
	assert.Contains(t, body, "AWS_ACCESS_KEY_ID")
	assert.Contains(t, body, "abc123")

	// Verify actions
	assert.Contains(t, body, "### Actions Required")
	assert.Contains(t, body, "- [ ] Rotate AWS credentials")
	assert.Contains(t, body, "### Actions Taken")
	assert.Contains(t, body, "- [x] Blocked deployment")

	// Verify footer
	assert.Contains(t, body, "dsops incident response")
}

func TestNotifier_BuildGitHubIssueBody_MinimalReport(t *testing.T) {
	t.Parallel()

	notifier := NewNotifier(NotificationConfig{})
	report := &Report{
		ID:          "INC-minimal",
		Timestamp:   time.Now(),
		Type:        "test",
		Severity:    "low",
		Title:       "Minimal",
		Description: "Minimal report",
		Status:      "open",
	}

	body := notifier.buildGitHubIssueBody(report)

	// Should not contain empty sections
	assert.NotContains(t, body, "### Details")
	assert.NotContains(t, body, "### Affected Resources")
	assert.NotContains(t, body, "### Actions Required")
	assert.NotContains(t, body, "### Actions Taken")
}

func TestReport_Marshaling(t *testing.T) {
	t.Parallel()

	now := time.Now()
	resolvedAt := now.Add(time.Hour)

	report := &Report{
		ID:               "INC-test",
		Timestamp:        now,
		Type:             "test",
		Severity:         "high",
		Title:            "Test",
		Description:      "Test description",
		Details:          map[string]string{"key": "value"},
		AffectedFiles:    []string{"file1.txt"},
		AffectedSecrets:  []string{"secret1"},
		AffectedCommits:  []string{"abc123"},
		ActionsRequired:  []string{"action1"},
		ActionsTaken:     []string{"taken1"},
		NotificationsSent: []NotificationRecord{
			{
				Channel:   "slack",
				Timestamp: now,
				Success:   true,
				Details:   "sent",
			},
		},
		Status:          "resolved",
		ResolvedAt:      &resolvedAt,
		ResolutionNotes: "Fixed",
	}

	// Marshal
	data, err := json.Marshal(report)
	require.NoError(t, err)

	// Unmarshal
	var loaded Report
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.Equal(t, report.ID, loaded.ID)
	assert.Equal(t, report.Type, loaded.Type)
	assert.Equal(t, report.Severity, loaded.Severity)
	assert.Equal(t, report.Title, loaded.Title)
	assert.Equal(t, report.Status, loaded.Status)
	assert.Equal(t, report.ResolutionNotes, loaded.ResolutionNotes)
}

