// Package integration provides integration tests for dsops.
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/rotation/notifications"
	"github.com/systmms/dsops/tests/testutil"
)

// mailhogMessage represents a message in MailHog's API response.
type mailhogMessage struct {
	ID      string `json:"ID"`
	From    struct {
		Mailbox string `json:"Mailbox"`
		Domain  string `json:"Domain"`
	} `json:"From"`
	To []struct {
		Mailbox string `json:"Mailbox"`
		Domain  string `json:"Domain"`
	} `json:"To"`
	Content struct {
		Headers map[string][]string `json:"Headers"`
		Body    string              `json:"Body"`
	} `json:"Content"`
	Raw struct {
		From string   `json:"From"`
		To   []string `json:"To"`
		Data string   `json:"Data"`
	} `json:"Raw"`
}

// mailhogResponse represents MailHog's API response.
type mailhogResponse struct {
	Total int              `json:"total"`
	Count int              `json:"count"`
	Start int              `json:"start"`
	Items []mailhogMessage `json:"items"`
}

// fetchMailHogMessages fetches messages from MailHog API.
func fetchMailHogMessages(t *testing.T) []mailhogMessage {
	t.Helper()

	resp, err := http.Get("http://localhost:8025/api/v2/messages")
	require.NoError(t, err)
	defer resp.Body.Close()

	var result mailhogResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	return result.Items
}

// deleteMailHogMessages deletes all messages from MailHog.
func deleteMailHogMessages(t *testing.T) {
	t.Helper()

	req, err := http.NewRequest("DELETE", "http://localhost:8025/api/v1/messages", nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
}

// TestEmailSMTP_BasicDelivery tests basic email delivery via SMTP with MailHog.
func TestEmailSMTP_BasicDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start MailHog via Docker
	env := testutil.StartDockerEnv(t, []string{"mailhog"})
	_ = env // Keep reference

	// Clear any existing messages
	deleteMailHogMessages(t)

	// Wait a moment for MailHog to be ready
	time.Sleep(500 * time.Millisecond)

	// Create email provider with MailHog config
	provider := notifications.NewEmailProvider(notifications.EmailConfig{
		SMTP: notifications.SMTPConfig{
			Host: "localhost",
			Port: 1025, // MailHog SMTP port
		},
		From: "dsops@example.com",
		To:   []string{"team@example.com"},
	})

	// Send failed rotation event
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "payment-api",
		Environment: "production",
		Strategy:    "two-key",
		Status:      notifications.StatusFailure,
		Error:       fmt.Errorf("database connection timeout"),
		Duration:    30 * time.Second,
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"rotation_id": "rot-12345",
			"version":     "v2.1.0",
		},
	}

	err := provider.Send(ctx, event)
	require.NoError(t, err)

	// Wait for MailHog to receive the message
	time.Sleep(500 * time.Millisecond)

	// Fetch messages from MailHog API
	messages := fetchMailHogMessages(t)
	require.Len(t, messages, 1, "Expected exactly one email")

	message := messages[0]

	// Validate sender
	assert.Equal(t, "dsops", message.From.Mailbox)
	assert.Equal(t, "example.com", message.From.Domain)

	// Validate recipient
	require.Len(t, message.To, 1)
	assert.Equal(t, "team", message.To[0].Mailbox)
	assert.Equal(t, "example.com", message.To[0].Domain)

	// Validate subject
	subjects, ok := message.Content.Headers["Subject"]
	require.True(t, ok, "Subject header should exist")
	require.Len(t, subjects, 1)
	assert.Contains(t, subjects[0], "[dsops] Rotation Failed")
	assert.Contains(t, subjects[0], "payment-api")
	assert.Contains(t, subjects[0], "production")

	// Validate MIME structure
	contentType := message.Content.Headers["Content-Type"]
	require.Len(t, contentType, 1)
	assert.Contains(t, contentType[0], "multipart/alternative")

	// Validate body contains expected content
	body := message.Content.Body
	assert.Contains(t, body, "payment-api")
	assert.Contains(t, body, "production")
	assert.Contains(t, body, "database connection timeout")
}

// TestEmailSMTP_MultipleRecipients tests sending email to multiple recipients.
func TestEmailSMTP_MultipleRecipients(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start MailHog via Docker
	env := testutil.StartDockerEnv(t, []string{"mailhog"})
	_ = env

	// Clear any existing messages
	deleteMailHogMessages(t)
	time.Sleep(500 * time.Millisecond)

	// Create email provider with multiple recipients
	provider := notifications.NewEmailProvider(notifications.EmailConfig{
		SMTP: notifications.SMTPConfig{
			Host: "localhost",
			Port: 1025,
		},
		From: "dsops@example.com",
		To: []string{
			"alice@example.com",
			"bob@example.com",
			"team@example.com",
		},
	})

	// Send event
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:        notifications.EventTypeCompleted,
		Service:     "api-gateway",
		Environment: "staging",
		Status:      notifications.StatusSuccess,
		Timestamp:   time.Now(),
	}

	err := provider.Send(ctx, event)
	require.NoError(t, err)

	// Wait for MailHog to receive the message
	time.Sleep(500 * time.Millisecond)

	// Fetch messages from MailHog
	messages := fetchMailHogMessages(t)
	require.Len(t, messages, 1, "Should send one message with multiple recipients")

	message := messages[0]

	// Validate all recipients received the email
	require.Len(t, message.To, 3)

	recipients := make(map[string]bool)
	for _, to := range message.To {
		fullAddr := fmt.Sprintf("%s@%s", to.Mailbox, to.Domain)
		recipients[fullAddr] = true
	}

	assert.True(t, recipients["alice@example.com"])
	assert.True(t, recipients["bob@example.com"])
	assert.True(t, recipients["team@example.com"])

	// Validate To header contains all recipients
	toHeader := message.Content.Headers["To"]
	require.Len(t, toHeader, 1)
	assert.Contains(t, toHeader[0], "alice@example.com")
	assert.Contains(t, toHeader[0], "bob@example.com")
	assert.Contains(t, toHeader[0], "team@example.com")
}

// TestEmailSMTP_EventFiltering tests that event filtering works correctly.
func TestEmailSMTP_EventFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start MailHog via Docker
	env := testutil.StartDockerEnv(t, []string{"mailhog"})
	_ = env

	// Clear any existing messages
	deleteMailHogMessages(t)
	time.Sleep(500 * time.Millisecond)

	// Create email provider that only sends for "failed" events
	provider := notifications.NewEmailProvider(notifications.EmailConfig{
		SMTP: notifications.SMTPConfig{
			Host: "localhost",
			Port: 1025,
		},
		From:   "dsops@example.com",
		To:     []string{"oncall@example.com"},
		Events: []string{"failed"}, // Only send for failed events
	})

	ctx := context.Background()

	// Verify provider doesn't support "completed" events
	assert.False(t, provider.SupportsEvent(notifications.EventTypeCompleted))

	// Verify provider supports "failed" events
	assert.True(t, provider.SupportsEvent(notifications.EventTypeFailed))

	// Send failed event (should be sent)
	failedEvent := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "test-service",
		Environment: "test",
		Status:      notifications.StatusFailure,
		Error:       fmt.Errorf("test error"),
		Timestamp:   time.Now(),
	}

	err := provider.Send(ctx, failedEvent)
	require.NoError(t, err)

	// Wait for MailHog
	time.Sleep(500 * time.Millisecond)

	// Should have exactly 1 email (only the failed event)
	messages := fetchMailHogMessages(t)
	require.Len(t, messages, 1)

	// Validate it's the failed event
	subjects := messages[0].Content.Headers["Subject"]
	require.Len(t, subjects, 1)
	assert.Contains(t, subjects[0], "Rotation Failed")
}

// TestEmailSMTP_HTMLAndTextParts tests that MIME multipart structure includes both HTML and text.
func TestEmailSMTP_HTMLAndTextParts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start MailHog via Docker
	env := testutil.StartDockerEnv(t, []string{"mailhog"})
	_ = env

	// Clear any existing messages
	deleteMailHogMessages(t)
	time.Sleep(500 * time.Millisecond)

	// Create email provider
	provider := notifications.NewEmailProvider(notifications.EmailConfig{
		SMTP: notifications.SMTPConfig{
			Host: "localhost",
			Port: 1025,
		},
		From: "dsops@example.com",
		To:   []string{"user@example.com"},
	})

	// Send event
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "database",
		Environment: "production",
		Status:      notifications.StatusFailure,
		Error:       fmt.Errorf("connection pool exhausted"),
		Timestamp:   time.Now(),
	}

	err := provider.Send(ctx, event)
	require.NoError(t, err)

	// Wait for MailHog
	time.Sleep(500 * time.Millisecond)

	// Fetch message
	messages := fetchMailHogMessages(t)
	require.Len(t, messages, 1)

	message := messages[0]

	// Validate MIME multipart
	contentType := message.Content.Headers["Content-Type"]
	require.Len(t, contentType, 1)
	assert.Contains(t, contentType[0], "multipart/alternative")

	// Validate body contains both text and HTML parts
	body := message.Raw.Data

	// Should have text/plain part
	assert.Contains(t, body, "Content-Type: text/plain")
	assert.Contains(t, body, "Rotation Failed")
	assert.Contains(t, body, "connection pool exhausted")

	// Should have text/html part
	assert.Contains(t, body, "Content-Type: text/html")
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "<html>")
	assert.Contains(t, body, "</html>")

	// HTML should contain colored styling and emojis
	assert.Contains(t, body, "background-color")
	assert.Contains(t, body, "&#x274C;") // ❌ emoji for failed
}

// TestEmailSMTP_MetadataInBody tests that event metadata appears in email body.
func TestEmailSMTP_MetadataInBody(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start MailHog via Docker
	env := testutil.StartDockerEnv(t, []string{"mailhog"})
	_ = env

	// Clear any existing messages
	deleteMailHogMessages(t)
	time.Sleep(500 * time.Millisecond)

	// Create email provider
	provider := notifications.NewEmailProvider(notifications.EmailConfig{
		SMTP: notifications.SMTPConfig{
			Host: "localhost",
			Port: 1025,
		},
		From: "dsops@example.com",
		To:   []string{"ops@example.com"},
	})

	// Send event with rich metadata
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "payment-gateway",
		Environment: "production",
		Strategy:    "two-key",
		Status:      notifications.StatusFailure,
		Error:       fmt.Errorf("API rate limit exceeded"),
		Duration:    45 * time.Second,
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"rotation_id":      "rot-abc123",
			"previous_version": "v1.2.3",
			"new_version":      "v1.2.4",
			"triggered_by":     "scheduler",
		},
	}

	err := provider.Send(ctx, event)
	require.NoError(t, err)

	// Wait for MailHog
	time.Sleep(500 * time.Millisecond)

	// Fetch message
	messages := fetchMailHogMessages(t)
	require.Len(t, messages, 1)

	body := messages[0].Raw.Data

	// Validate metadata fields are in the email
	assert.Contains(t, body, "rot-abc123")
	assert.Contains(t, body, "v1.2.3")
	assert.Contains(t, body, "v1.2.4")
	assert.Contains(t, body, "scheduler")

	// Validate core fields
	assert.Contains(t, body, "payment-gateway")
	assert.Contains(t, body, "production")
	assert.Contains(t, body, "two-key")
	assert.Contains(t, body, "API rate limit exceeded")
	assert.Contains(t, body, "45s")
}

// TestEmailSMTP_HeaderInjectionPrevention tests that header injection is prevented.
func TestEmailSMTP_HeaderInjectionPrevention(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start MailHog via Docker
	env := testutil.StartDockerEnv(t, []string{"mailhog"})
	_ = env

	// Clear any existing messages
	deleteMailHogMessages(t)
	time.Sleep(500 * time.Millisecond)

	// Create email provider
	provider := notifications.NewEmailProvider(notifications.EmailConfig{
		SMTP: notifications.SMTPConfig{
			Host: "localhost",
			Port: 1025,
		},
		From: "dsops@example.com",
		To:   []string{"security@example.com"},
	})

	// Send event with malicious service name containing newlines
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "evil-service\r\nBcc: attacker@evil.com",
		Environment: "prod\nX-Injected-Header: malicious",
		Status:      notifications.StatusFailure,
		Timestamp:   time.Now(),
	}

	err := provider.Send(ctx, event)
	require.NoError(t, err)

	// Wait for MailHog
	time.Sleep(500 * time.Millisecond)

	// Fetch message
	messages := fetchMailHogMessages(t)
	require.Len(t, messages, 1)

	message := messages[0]

	// Validate subject doesn't contain injected headers
	subjects := message.Content.Headers["Subject"]
	require.Len(t, subjects, 1)

	// Newlines should be stripped from subject
	assert.NotContains(t, subjects[0], "\r")
	assert.NotContains(t, subjects[0], "\n")
	assert.NotContains(t, subjects[0], "Bcc:")
	assert.NotContains(t, subjects[0], "X-Injected-Header")

	// Validate no Bcc header was injected
	_, hasBcc := message.Content.Headers["Bcc"]
	assert.False(t, hasBcc, "Should not have Bcc header from injection")

	// Validate no custom injected header
	_, hasInjected := message.Content.Headers["X-Injected-Header"]
	assert.False(t, hasInjected, "Should not have X-Injected-Header from injection")
}

// TestEmailSMTP_DifferentEventTypes tests email content for different event types.
func TestEmailSMTP_DifferentEventTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	eventTypes := []struct {
		name          string
		eventType     notifications.EventType
		status        notifications.RotationStatus
		expectedTitle string
		expectedEmoji string
	}{
		{
			name:          "started",
			eventType:     notifications.EventTypeStarted,
			status:        notifications.StatusSuccess,
			expectedTitle: "Rotation Started",
			expectedEmoji: "&#x1F504;", // 🔄
		},
		{
			name:          "completed_success",
			eventType:     notifications.EventTypeCompleted,
			status:        notifications.StatusSuccess,
			expectedTitle: "Rotation Completed",
			expectedEmoji: "&#x2705;", // ✅
		},
		{
			name:          "failed",
			eventType:     notifications.EventTypeFailed,
			status:        notifications.StatusFailure,
			expectedTitle: "Rotation Failed",
			expectedEmoji: "&#x274C;", // ❌
		},
		{
			name:          "rollback",
			eventType:     notifications.EventTypeRollback,
			status:        notifications.StatusRolledBack,
			expectedTitle: "Rotation Rolled Back",
			expectedEmoji: "&#x23EA;", // ⏪
		},
	}

	for _, tt := range eventTypes {
		t.Run(tt.name, func(t *testing.T) {
			// Start MailHog via Docker
			env := testutil.StartDockerEnv(t, []string{"mailhog"})
			_ = env

			// Clear messages
			deleteMailHogMessages(t)
			time.Sleep(200 * time.Millisecond)

			// Create provider
			provider := notifications.NewEmailProvider(notifications.EmailConfig{
				SMTP: notifications.SMTPConfig{
					Host: "localhost",
					Port: 1025,
				},
				From: "dsops@example.com",
				To:   []string{"test@example.com"},
			})

			// Send event
			ctx := context.Background()
			event := notifications.RotationEvent{
				Type:        tt.eventType,
				Service:     "test-service",
				Environment: "test",
				Status:      tt.status,
				Timestamp:   time.Now(),
			}

			err := provider.Send(ctx, event)
			require.NoError(t, err)

			// Wait for MailHog
			time.Sleep(500 * time.Millisecond)

			// Fetch message
			messages := fetchMailHogMessages(t)
			require.Len(t, messages, 1)

			// Validate subject contains expected title
			subjects := messages[0].Content.Headers["Subject"]
			require.Len(t, subjects, 1)
			assert.Contains(t, subjects[0], tt.expectedTitle)

			// Validate body contains expected emoji (in HTML part)
			body := messages[0].Raw.Data
			assert.Contains(t, body, tt.expectedEmoji)
		})
	}
}

// TestEmailSMTP_ConnectionError tests handling of SMTP connection errors.
func TestEmailSMTP_ConnectionError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create email provider with invalid SMTP server
	provider := notifications.NewEmailProvider(notifications.EmailConfig{
		SMTP: notifications.SMTPConfig{
			Host: "localhost",
			Port: 9999, // Invalid port
		},
		From: "dsops@example.com",
		To:   []string{"test@example.com"},
	})

	// Send event
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "test",
		Environment: "test",
		Timestamp:   time.Now(),
	}

	// Should return error
	err := provider.Send(ctx, event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send email")
}

// TestEmailSMTP_TimeoutHandling tests that email sending respects context timeout.
func TestEmailSMTP_TimeoutHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Note: This test is tricky because smtp.SendMail doesn't respect context
	// We're just testing that the provider handles context properly
	// In reality, SMTP has its own timeout mechanisms

	// Start MailHog
	env := testutil.StartDockerEnv(t, []string{"mailhog"})
	_ = env

	deleteMailHogMessages(t)
	time.Sleep(500 * time.Millisecond)

	provider := notifications.NewEmailProvider(notifications.EmailConfig{
		SMTP: notifications.SMTPConfig{
			Host: "localhost",
			Port: 1025,
		},
		From: "dsops@example.com",
		To:   []string{"test@example.com"},
	})

	// Create context with very short timeout
	// Note: smtp.SendMail may still complete even with cancelled context
	// This is a limitation of the standard library
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	event := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "test",
		Environment: "test",
		Timestamp:   time.Now(),
	}

	// The Send function itself doesn't use context for SMTP operations
	// (limitation of smtp.SendMail), so this will likely succeed
	// This test documents current behavior
	_ = provider.Send(ctx, event)

	// Note: In a production system, you'd want to use a context-aware SMTP library
}

// TestEmailSMTP_RealWorldScenario tests a realistic rotation failure scenario.
func TestEmailSMTP_RealWorldScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start MailHog via Docker
	env := testutil.StartDockerEnv(t, []string{"mailhog"})
	_ = env

	// Clear messages
	deleteMailHogMessages(t)
	time.Sleep(500 * time.Millisecond)

	// Create provider with realistic config
	provider := notifications.NewEmailProvider(notifications.EmailConfig{
		SMTP: notifications.SMTPConfig{
			Host: "localhost",
			Port: 1025,
		},
		From: "dsops-prod@company.com",
		To: []string{
			"platform-team@company.com",
			"sre-oncall@company.com",
			"security-team@company.com",
		},
		Events: []string{"failed", "rollback"}, // Only critical events
	})

	// Simulate production database rotation failure
	ctx := context.Background()
	failureEvent := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "postgres-primary",
		Environment: "production",
		Strategy:    "two-key",
		Status:      notifications.StatusFailure,
		Error:       fmt.Errorf("new credentials validation failed: permission denied for database 'analytics'"),
		Duration:    2*time.Minute + 34*time.Second,
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"rotation_id":         "rot-2024-12-09-143022-abc123",
			"previous_key_id":     "key-v47",
			"new_key_id":          "key-v48",
			"triggered_by":        "scheduled-rotation",
			"affected_databases":  "analytics,reporting,metrics",
			"runbook_url":         "https://wiki.company.com/runbooks/postgres-rotation-failure",
			"pagerduty_incident":  "PD-12345",
		},
	}

	err := provider.Send(ctx, failureEvent)
	require.NoError(t, err)

	// Wait for MailHog
	time.Sleep(500 * time.Millisecond)

	// Fetch and validate message
	messages := fetchMailHogMessages(t)
	require.Len(t, messages, 1)

	message := messages[0]

	// Validate all recipients
	require.Len(t, message.To, 3)

	// Validate subject is informative
	subjects := message.Content.Headers["Subject"]
	require.Len(t, subjects, 1)
	subject := subjects[0]
	assert.Contains(t, subject, "[dsops]")
	assert.Contains(t, subject, "Rotation Failed")
	assert.Contains(t, subject, "postgres-primary")
	assert.Contains(t, subject, "production")

	// Validate body contains all critical information
	body := message.Raw.Data

	// Core event details
	assert.Contains(t, body, "postgres-primary")
	assert.Contains(t, body, "production")
	assert.Contains(t, body, "two-key")
	assert.Contains(t, body, "permission denied")

	// Metadata
	assert.Contains(t, body, "rot-2024-12-09-143022-abc123")
	assert.Contains(t, body, "key-v47")
	assert.Contains(t, body, "key-v48")
	assert.Contains(t, body, "analytics,reporting,metrics")
	assert.Contains(t, body, "https://wiki.company.com/runbooks/postgres-rotation-failure")
	assert.Contains(t, body, "PD-12345")

	// Duration
	assert.Contains(t, body, "2m")

	// Should have both HTML and text parts
	assert.Contains(t, body, "text/html")
	assert.Contains(t, body, "text/plain")
}
