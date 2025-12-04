package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// PagerDuty Events API v2 endpoint
const pagerDutyAPIURL = "https://events.pagerduty.com/v2/enqueue"

// PagerDutySeverity represents PagerDuty incident severity levels.
type PagerDutySeverity string

const (
	SeverityCritical PagerDutySeverity = "critical"
	SeverityError    PagerDutySeverity = "error"
	SeverityWarning  PagerDutySeverity = "warning"
	SeverityInfo     PagerDutySeverity = "info"
)

// PagerDutyConfig holds configuration for PagerDuty notifications.
type PagerDutyConfig struct {
	// IntegrationKey is the PagerDuty Events API v2 integration key.
	IntegrationKey string

	// ServiceID is the PagerDuty service ID (optional, for reference).
	ServiceID string

	// Severity is the default incident severity: critical, error, warning, info.
	// Defaults to "error" if empty.
	Severity string

	// Events specifies which rotation events trigger notifications.
	// If empty, all events are sent.
	Events []string

	// AutoResolve indicates whether to auto-resolve incidents on successful completion.
	AutoResolve bool
}

// PagerDutyProvider sends rotation notifications to PagerDuty.
type PagerDutyProvider struct {
	config PagerDutyConfig
	client *http.Client
	apiURL string
}

// NewPagerDutyProvider creates a new PagerDuty notification provider.
func NewPagerDutyProvider(config PagerDutyConfig) *PagerDutyProvider {
	return &PagerDutyProvider{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiURL: pagerDutyAPIURL,
	}
}

// Name returns the provider name.
func (p *PagerDutyProvider) Name() string {
	return "pagerduty"
}

// SupportsEvent returns true if this provider handles the given event type.
func (p *PagerDutyProvider) SupportsEvent(eventType EventType) bool {
	// If no events are configured, support all
	if len(p.config.Events) == 0 {
		return true
	}

	eventStr := string(eventType)
	for _, e := range p.config.Events {
		if strings.EqualFold(e, eventStr) {
			return true
		}
	}
	return false
}

// Validate checks if the provider configuration is valid.
func (p *PagerDutyProvider) Validate(ctx context.Context) error {
	if p.config.IntegrationKey == "" {
		return fmt.Errorf("integration key is required")
	}

	// Validate severity if set
	if p.config.Severity != "" {
		switch strings.ToLower(p.config.Severity) {
		case "critical", "error", "warning", "info":
			// Valid
		default:
			return fmt.Errorf("invalid severity: %s (must be critical, error, warning, or info)", p.config.Severity)
		}
	}

	return nil
}

// Send sends a PagerDuty event for the given rotation event.
func (p *PagerDutyProvider) Send(ctx context.Context, event RotationEvent) error {
	action := p.determineAction(event)

	// If this is a resolve action but AutoResolve is disabled, skip
	if action == "resolve" && !p.config.AutoResolve {
		return nil
	}

	payload := p.buildPayload(event, action)

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal PagerDuty payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send PagerDuty notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PagerDuty returned status %d", resp.StatusCode)
	}

	return nil
}

// determineAction returns the PagerDuty event action based on the rotation event.
func (p *PagerDutyProvider) determineAction(event RotationEvent) string {
	switch event.Type {
	case EventTypeCompleted:
		if event.Status == StatusSuccess {
			return "resolve"
		}
		return "trigger"
	case EventTypeFailed, EventTypeRollback:
		return "trigger"
	default:
		return "trigger"
	}
}

// buildPayload creates the PagerDuty Events API v2 payload.
func (p *PagerDutyProvider) buildPayload(event RotationEvent, action string) map[string]interface{} {
	payload := map[string]interface{}{
		"routing_key":  p.config.IntegrationKey,
		"event_action": action,
		"dedup_key":    p.buildDedupKey(event),
	}

	// Add payload details for trigger/acknowledge actions
	if action != "resolve" {
		payload["payload"] = p.buildEventPayload(event)
	} else {
		// For resolve, still include minimal payload
		payload["payload"] = map[string]interface{}{
			"summary":  fmt.Sprintf("dsops rotation completed: %s (%s)", event.Service, event.Environment),
			"severity": p.getSeverity(),
			"source":   "dsops-rotation",
		}
	}

	return payload
}

// buildEventPayload creates the payload section for PagerDuty events.
func (p *PagerDutyProvider) buildEventPayload(event RotationEvent) map[string]interface{} {
	summary := p.buildSummary(event)

	customDetails := map[string]interface{}{
		"service":     event.Service,
		"environment": event.Environment,
		"event_type":  string(event.Type),
		"status":      string(event.Status),
		"timestamp":   event.Timestamp.Format(time.RFC3339),
	}

	if event.Strategy != "" {
		customDetails["strategy"] = event.Strategy
	}

	if event.Duration > 0 {
		customDetails["duration"] = event.Duration.String()
	}

	if event.Error != nil {
		customDetails["error"] = event.Error.Error()
	}

	// Add metadata
	for k, v := range event.Metadata {
		customDetails[k] = v
	}

	payload := map[string]interface{}{
		"summary":        summary,
		"severity":       p.getSeverity(),
		"source":         "dsops-rotation",
		"custom_details": customDetails,
	}

	// Add timestamp if available
	if !event.Timestamp.IsZero() {
		payload["timestamp"] = event.Timestamp.Format(time.RFC3339)
	}

	return payload
}

// buildSummary creates a human-readable summary for the PagerDuty incident.
func (p *PagerDutyProvider) buildSummary(event RotationEvent) string {
	var action string
	switch event.Type {
	case EventTypeStarted:
		action = "started"
	case EventTypeCompleted:
		if event.Status == StatusSuccess {
			action = "completed successfully"
		} else {
			action = "completed with warnings"
		}
	case EventTypeFailed:
		action = "failed"
	case EventTypeRollback:
		action = "rollback triggered"
	default:
		action = "event"
	}

	summary := fmt.Sprintf("dsops rotation %s: %s (%s)", action, event.Service, event.Environment)

	if event.Error != nil {
		summary = fmt.Sprintf("%s - %s", summary, event.Error.Error())
	}

	// Truncate to PagerDuty's limit (1024 chars)
	if len(summary) > 1024 {
		summary = summary[:1021] + "..."
	}

	return summary
}

// buildDedupKey creates a deduplication key for the event.
// This ensures related events (trigger, resolve) are grouped together.
func (p *PagerDutyProvider) buildDedupKey(event RotationEvent) string {
	parts := []string{"dsops", event.Service, event.Environment}

	// Include rotation_id if available
	if rotationID, ok := event.Metadata["rotation_id"]; ok && rotationID != "" {
		parts = append(parts, rotationID)
	}

	return strings.Join(parts, "-")
}

// getSeverity returns the configured severity or default.
func (p *PagerDutyProvider) getSeverity() string {
	if p.config.Severity == "" {
		return string(SeverityError)
	}
	return strings.ToLower(p.config.Severity)
}

// PagerDutyNotificationConfig mirrors the config package type for internal use.
type PagerDutyNotificationConfig struct {
	IntegrationKey string
	ServiceID      string
	Severity       string
	Events         []string
	AutoResolve    bool
}

// CreatePagerDutyProvider creates a PagerDuty provider from config notification settings.
func CreatePagerDutyProvider(config *PagerDutyNotificationConfig) (*PagerDutyProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("pagerduty config is nil")
	}

	pdConfig := PagerDutyConfig{
		IntegrationKey: config.IntegrationKey,
		ServiceID:      config.ServiceID,
		Severity:       config.Severity,
		Events:         config.Events,
		AutoResolve:    config.AutoResolve,
	}

	provider := NewPagerDutyProvider(pdConfig)
	if err := provider.Validate(context.Background()); err != nil {
		return nil, err
	}

	return provider, nil
}
