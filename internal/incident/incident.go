package incident

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	IncidentDirName = ".dsops/incidents"
	AuditLogName    = ".dsops/audit.log"
)

// Report represents a security incident report
type Report struct {
	ID          string            `json:"id"`
	Timestamp   time.Time         `json:"timestamp"`
	Type        string            `json:"type"`
	Severity    string            `json:"severity"` // critical, high, medium, low
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Details     map[string]string `json:"details"`
	
	// Affected resources
	AffectedFiles    []string `json:"affected_files,omitempty"`
	AffectedSecrets  []string `json:"affected_secrets,omitempty"`
	AffectedCommits  []string `json:"affected_commits,omitempty"`
	
	// Response actions
	ActionsRequired  []string `json:"actions_required"`
	ActionsTaken     []string `json:"actions_taken,omitempty"`
	
	// Notification status
	NotificationsSent []NotificationRecord `json:"notifications_sent,omitempty"`
	
	// Resolution
	Status           string    `json:"status"` // open, investigating, resolved
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
	ResolutionNotes  string    `json:"resolution_notes,omitempty"`
}

// NotificationRecord tracks sent notifications
type NotificationRecord struct {
	Channel   string    `json:"channel"` // slack, github, email
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Details   string    `json:"details,omitempty"`
}

// Manager handles incident creation and management
type Manager struct {
	incidentDir string
	auditPath   string
}

// NewManager creates a new incident manager
func NewManager(baseDir string) *Manager {
	if baseDir == "" {
		baseDir = "."
	}
	
	return &Manager{
		incidentDir: filepath.Join(baseDir, IncidentDirName),
		auditPath:   filepath.Join(baseDir, AuditLogName),
	}
}

// CreateReport creates a new incident report
func (m *Manager) CreateReport(incidentType, severity, title, description string, details map[string]string) (*Report, error) {
	// Ensure incident directory exists
	if err := os.MkdirAll(m.incidentDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create incident directory: %w", err)
	}
	
	// Generate incident ID
	id := generateIncidentID()
	
	report := &Report{
		ID:          id,
		Timestamp:   time.Now(),
		Type:        incidentType,
		Severity:    severity,
		Title:       title,
		Description: description,
		Details:     details,
		Status:      "open",
	}
	
	// Save report
	if err := m.SaveReport(report); err != nil {
		return nil, err
	}
	
	// Log to audit
	if err := m.logToAudit(report, "incident_created"); err != nil {
		// Don't fail on audit log errors
		fmt.Fprintf(os.Stderr, "Warning: failed to write audit log: %v\n", err)
	}
	
	return report, nil
}

// SaveReport saves an incident report to disk
func (m *Manager) SaveReport(report *Report) error {
	filename := fmt.Sprintf("%s.json", report.ID)
	path := filepath.Join(m.incidentDir, filename)
	
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}
	
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}
	
	return nil
}

// LoadReport loads an incident report by ID
func (m *Manager) LoadReport(id string) (*Report, error) {
	filename := fmt.Sprintf("%s.json", id)
	path := filepath.Join(m.incidentDir, filename)
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("incident not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read report: %w", err)
	}
	
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse report: %w", err)
	}
	
	return &report, nil
}

// ListReports returns all incident reports
func (m *Manager) ListReports() ([]*Report, error) {
	// Check if directory exists
	if _, err := os.Stat(m.incidentDir); os.IsNotExist(err) {
		return []*Report{}, nil // No incidents yet
	}
	
	entries, err := os.ReadDir(m.incidentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read incident directory: %w", err)
	}
	
	var reports []*Report
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		
		id := strings.TrimSuffix(entry.Name(), ".json")
		report, err := m.LoadReport(id)
		if err != nil {
			// Skip invalid reports
			continue
		}
		
		reports = append(reports, report)
	}
	
	return reports, nil
}

// UpdateReport updates an existing report
func (m *Manager) UpdateReport(report *Report) error {
	if err := m.SaveReport(report); err != nil {
		return err
	}
	
	// Log update
	if err := m.logToAudit(report, "incident_updated"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write audit log: %v\n", err)
	}
	
	return nil
}

// AddNotification records a notification being sent
func (m *Manager) AddNotification(report *Report, channel string, success bool, details string) error {
	record := NotificationRecord{
		Channel:   channel,
		Timestamp: time.Now(),
		Success:   success,
		Details:   details,
	}
	
	report.NotificationsSent = append(report.NotificationsSent, record)
	return m.UpdateReport(report)
}

// ResolveReport marks an incident as resolved
func (m *Manager) ResolveReport(report *Report, resolutionNotes string) error {
	now := time.Now()
	report.Status = "resolved"
	report.ResolvedAt = &now
	report.ResolutionNotes = resolutionNotes
	
	if err := m.UpdateReport(report); err != nil {
		return err
	}
	
	// Log resolution
	if err := m.logToAudit(report, "incident_resolved"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write audit log: %v\n", err)
	}
	
	return nil
}

// logToAudit writes an entry to the audit log
func (m *Manager) logToAudit(report *Report, action string) error {
	// Ensure audit directory exists
	auditDir := filepath.Dir(m.auditPath)
	if err := os.MkdirAll(auditDir, 0700); err != nil {
		return fmt.Errorf("failed to create audit directory: %w", err)
	}
	
	// Create audit entry
	entry := map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"action":      action,
		"incident_id": report.ID,
		"type":        report.Type,
		"severity":    report.Severity,
		"title":       report.Title,
		"status":      report.Status,
	}
	
	// Convert to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}
	
	// Append to audit log
	f, err := os.OpenFile(m.auditPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open audit log: %w", err)
	}
	defer func() { _ = f.Close() }()
	
	if _, err := fmt.Fprintf(f, "%s\n", data); err != nil {
		return fmt.Errorf("failed to write audit log: %w", err)
	}
	
	return nil
}

// generateIncidentID creates a unique incident ID
func generateIncidentID() string {
	return fmt.Sprintf("INC-%s-%d", 
		time.Now().Format("20060102"), 
		time.Now().Unix()%100000)
}

// GetOpenIncidents returns all open incidents
func (m *Manager) GetOpenIncidents() ([]*Report, error) {
	allReports, err := m.ListReports()
	if err != nil {
		return nil, err
	}
	
	var openReports []*Report
	for _, report := range allReports {
		if report.Status != "resolved" {
			openReports = append(openReports, report)
		}
	}
	
	return openReports, nil
}

// GetIncidentsByType returns incidents of a specific type
func (m *Manager) GetIncidentsByType(incidentType string) ([]*Report, error) {
	allReports, err := m.ListReports()
	if err != nil {
		return nil, err
	}
	
	var filteredReports []*Report
	for _, report := range allReports {
		if report.Type == incidentType {
			filteredReports = append(filteredReports, report)
		}
	}
	
	return filteredReports, nil
}