package notifications

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"net/smtp"
	"regexp"
	"strings"
	"time"
)

// headerPattern matches common email header injection patterns.
// This catches: Bcc:, Cc:, To:, From:, Subject:, Reply-To:, X-*: headers
var headerPattern = regexp.MustCompile(`(?i)\b(bcc|cc|to|from|subject|reply-to|x-[a-z0-9-]+)\s*:`)

// BatchMode represents the email batching mode.
type BatchMode string

const (
	// BatchModeImmediate sends emails immediately for each event.
	BatchModeImmediate BatchMode = "immediate"

	// BatchModeHourly batches emails and sends hourly digests.
	BatchModeHourly BatchMode = "hourly"

	// BatchModeDaily batches emails and sends daily digests.
	BatchModeDaily BatchMode = "daily"
)

// SMTPConfig holds SMTP server configuration.
type SMTPConfig struct {
	// Host is the SMTP server hostname.
	Host string

	// Port is the SMTP server port.
	Port int

	// Username for SMTP authentication (optional).
	Username string

	// Password for SMTP authentication (optional).
	Password string

	// TLS enables TLS/STARTTLS for the connection.
	TLS bool
}

// EmailConfig holds configuration for email notifications.
type EmailConfig struct {
	// SMTP server configuration.
	SMTP SMTPConfig

	// From is the sender email address.
	From string

	// To is the list of recipient email addresses.
	To []string

	// Events specifies which rotation events trigger notifications.
	// If empty, all events are sent.
	Events []string

	// BatchMode controls email batching: immediate, hourly, daily.
	// Default is immediate.
	BatchMode string
}

// SMTPSendFunc is the function signature for sending emails via SMTP.
type SMTPSendFunc func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error

// EmailProvider sends rotation notifications via email.
type EmailProvider struct {
	config     EmailConfig
	smtpSender SMTPSendFunc
}

// NewEmailProvider creates a new email notification provider.
func NewEmailProvider(config EmailConfig) *EmailProvider {
	return &EmailProvider{
		config:     config,
		smtpSender: smtp.SendMail,
	}
}

// Name returns the provider name.
func (p *EmailProvider) Name() string {
	return "email"
}

// SupportsEvent returns true if this provider handles the given event type.
func (p *EmailProvider) SupportsEvent(eventType EventType) bool {
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
func (p *EmailProvider) Validate(ctx context.Context) error {
	if p.config.SMTP.Host == "" {
		return fmt.Errorf("SMTP host is required")
	}

	if p.config.SMTP.Port == 0 {
		return fmt.Errorf("SMTP port is required")
	}

	if p.config.From == "" {
		return fmt.Errorf("from address is required")
	}

	if len(p.config.To) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	// Validate batch mode if set
	if p.config.BatchMode != "" {
		switch strings.ToLower(p.config.BatchMode) {
		case "immediate", "hourly", "daily":
			// Valid
		default:
			return fmt.Errorf("invalid batch mode: %s (must be immediate, hourly, or daily)", p.config.BatchMode)
		}
	}

	return nil
}

// GetBatchMode returns the configured batch mode.
func (p *EmailProvider) GetBatchMode() BatchMode {
	switch strings.ToLower(p.config.BatchMode) {
	case "hourly":
		return BatchModeHourly
	case "daily":
		return BatchModeDaily
	default:
		return BatchModeImmediate
	}
}

// Send sends an email notification for the given rotation event.
func (p *EmailProvider) Send(ctx context.Context, event RotationEvent) error {
	// For batch modes, we would queue the event and return
	// For now, we only implement immediate mode
	// TODO: Implement batching logic with background timer
	// Currently only immediate mode is supported, batching falls through

	msg := p.buildMIMEMessage(event)

	addr := fmt.Sprintf("%s:%d", p.config.SMTP.Host, p.config.SMTP.Port)

	var auth smtp.Auth
	if p.config.SMTP.Username != "" {
		auth = smtp.PlainAuth("", p.config.SMTP.Username, p.config.SMTP.Password, p.config.SMTP.Host)
	}

	err := p.smtpSender(addr, auth, p.config.From, p.config.To, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// buildMIMEMessage creates a MIME multipart email with both HTML and plain-text parts.
func (p *EmailProvider) buildMIMEMessage(event RotationEvent) string {
	subject, htmlBody, textBody := p.buildMessage(event)

	// Generate a boundary for multipart
	boundary := fmt.Sprintf("----=_Part_%d", time.Now().UnixNano())

	var buf bytes.Buffer

	// Headers
	buf.WriteString(fmt.Sprintf("From: %s\r\n", p.config.From))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(p.config.To, ", ")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	buf.WriteString("\r\n")

	// Plain-text part
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(textBody)
	buf.WriteString("\r\n")

	// HTML part
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(htmlBody)
	buf.WriteString("\r\n")

	// End boundary
	buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return buf.String()
}

// buildMessage creates the email subject, HTML body, and plain-text body.
func (p *EmailProvider) buildMessage(event RotationEvent) (subject, htmlBody, textBody string) {
	// Sanitize service and environment for subject (prevent header injection)
	service := sanitizeHeader(event.Service)
	environment := sanitizeHeader(event.Environment)

	// Build subject
	subject = fmt.Sprintf("[dsops] %s: %s (%s)", p.getEventTitle(event.Type, event.Status), service, environment)

	// Build bodies
	htmlBody = p.buildHTMLBody(event)
	textBody = p.buildTextBody(event)

	return subject, htmlBody, textBody
}

// getEventTitle returns a human-readable title for the event.
func (p *EmailProvider) getEventTitle(eventType EventType, status RotationStatus) string {
	switch eventType {
	case EventTypeStarted:
		return "Rotation Started"
	case EventTypeCompleted:
		if status == StatusSuccess {
			return "Rotation Completed"
		}
		return "Rotation Completed with Warnings"
	case EventTypeFailed:
		return "Rotation Failed"
	case EventTypeRollback:
		return "Rotation Rolled Back"
	default:
		return "Rotation Event"
	}
}

// getEventEmoji returns an emoji for the event type (for HTML).
func (p *EmailProvider) getEventEmoji(eventType EventType, status RotationStatus) string {
	switch eventType {
	case EventTypeStarted:
		return "&#x1F504;" // 🔄
	case EventTypeCompleted:
		if status == StatusSuccess {
			return "&#x2705;" // ✅
		}
		return "&#x26A0;" // ⚠️
	case EventTypeFailed:
		return "&#x274C;" // ❌
	case EventTypeRollback:
		return "&#x23EA;" // ⏪
	default:
		return "&#x1F514;" // 🔔
	}
}

// getStatusColor returns a color for the status.
func (p *EmailProvider) getStatusColor(eventType EventType, status RotationStatus) string {
	switch eventType {
	case EventTypeCompleted:
		if status == StatusSuccess {
			return "#28a745" // green
		}
		return "#ffc107" // yellow
	case EventTypeFailed:
		return "#dc3545" // red
	case EventTypeRollback:
		return "#fd7e14" // orange
	default:
		return "#6c757d" // gray
	}
}

// buildHTMLBody creates the HTML email body.
func (p *EmailProvider) buildHTMLBody(event RotationEvent) string {
	title := p.getEventTitle(event.Type, event.Status)
	emoji := p.getEventEmoji(event.Type, event.Status)
	color := p.getStatusColor(event.Type, event.Status)

	var buf bytes.Buffer

	buf.WriteString(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>dsops Rotation Notification</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
`)

	// Header
	buf.WriteString(fmt.Sprintf(`<div style="background-color: %s; color: white; padding: 20px; border-radius: 8px 8px 0 0;">
<h1 style="margin: 0; font-size: 24px;">%s %s</h1>
</div>
`, color, emoji, html.EscapeString(title)))

	// Body
	buf.WriteString(`<div style="background-color: #f8f9fa; padding: 20px; border-radius: 0 0 8px 8px; border: 1px solid #dee2e6; border-top: none;">
`)

	// Service and Environment
	buf.WriteString(`<table style="width: 100%; border-collapse: collapse; margin-bottom: 20px;">
`)
	buf.WriteString(fmt.Sprintf(`<tr>
<td style="padding: 8px 0;"><strong>Service:</strong></td>
<td style="padding: 8px 0;">%s</td>
</tr>
`, html.EscapeString(event.Service)))
	buf.WriteString(fmt.Sprintf(`<tr>
<td style="padding: 8px 0;"><strong>Environment:</strong></td>
<td style="padding: 8px 0;">%s</td>
</tr>
`, html.EscapeString(event.Environment)))

	if event.Strategy != "" {
		buf.WriteString(fmt.Sprintf(`<tr>
<td style="padding: 8px 0;"><strong>Strategy:</strong></td>
<td style="padding: 8px 0;">%s</td>
</tr>
`, html.EscapeString(event.Strategy)))
	}

	if event.Duration > 0 {
		buf.WriteString(fmt.Sprintf(`<tr>
<td style="padding: 8px 0;"><strong>Duration:</strong></td>
<td style="padding: 8px 0;">%s</td>
</tr>
`, event.Duration.Round(time.Second).String()))
	}

	buf.WriteString(fmt.Sprintf(`<tr>
<td style="padding: 8px 0;"><strong>Timestamp:</strong></td>
<td style="padding: 8px 0;">%s</td>
</tr>
`, event.Timestamp.Format(time.RFC3339)))

	buf.WriteString(`</table>
`)

	// Error section if present
	if event.Error != nil {
		buf.WriteString(fmt.Sprintf(`<div style="background-color: #f8d7da; border: 1px solid #f5c6cb; border-radius: 4px; padding: 15px; margin-bottom: 20px;">
<strong>Error:</strong><br>
<code style="font-family: monospace; color: #721c24;">%s</code>
</div>
`, html.EscapeString(event.Error.Error())))
	}

	// Metadata section if present
	if len(event.Metadata) > 0 {
		buf.WriteString(`<div style="margin-top: 15px;">
<strong>Additional Details:</strong>
<ul style="margin-top: 10px;">
`)
		for key, value := range event.Metadata {
			buf.WriteString(fmt.Sprintf(`<li><strong>%s:</strong> %s</li>
`, html.EscapeString(key), html.EscapeString(value)))
		}
		buf.WriteString(`</ul>
</div>
`)
	}

	buf.WriteString(`</div>

<div style="margin-top: 20px; font-size: 12px; color: #6c757d; text-align: center;">
<p>This notification was sent by dsops rotation system.</p>
<p>Run <code>dsops rotation history --service ` + html.EscapeString(event.Service) + `</code> for details.</p>
</div>
</body>
</html>`)

	return buf.String()
}

// buildTextBody creates the plain-text email body.
func (p *EmailProvider) buildTextBody(event RotationEvent) string {
	title := p.getEventTitle(event.Type, event.Status)

	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("%s\n", title))
	buf.WriteString(strings.Repeat("=", len(title)))
	buf.WriteString("\n\n")

	buf.WriteString(fmt.Sprintf("Service: %s\n", event.Service))
	buf.WriteString(fmt.Sprintf("Environment: %s\n", event.Environment))

	if event.Strategy != "" {
		buf.WriteString(fmt.Sprintf("Strategy: %s\n", event.Strategy))
	}

	if event.Duration > 0 {
		buf.WriteString(fmt.Sprintf("Duration: %s\n", event.Duration.Round(time.Second).String()))
	}

	buf.WriteString(fmt.Sprintf("Timestamp: %s\n", event.Timestamp.Format(time.RFC3339)))

	if event.Error != nil {
		buf.WriteString(fmt.Sprintf("\nError: %s\n", event.Error.Error()))
	}

	if len(event.Metadata) > 0 {
		buf.WriteString("\nAdditional Details:\n")
		for key, value := range event.Metadata {
			buf.WriteString(fmt.Sprintf("  - %s: %s\n", key, value))
		}
	}

	buf.WriteString("\n---\n")
	buf.WriteString("This notification was sent by dsops rotation system.\n")
	buf.WriteString(fmt.Sprintf("Run `dsops rotation history --service %s` for details.\n", event.Service))

	return buf.String()
}

// sanitizeHeader removes newlines and header injection patterns to prevent
// both SMTP header injection and confusing subject lines.
func sanitizeHeader(s string) string {
	// Replace newlines with spaces (preserve readability)
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")

	// Remove header-like patterns (e.g., "Bcc:", "X-Custom:")
	s = headerPattern.ReplaceAllString(s, "")

	// Collapse multiple spaces into single space
	s = strings.Join(strings.Fields(s), " ")

	return s
}

// EmailNotificationConfig mirrors the config package type for internal use.
type EmailNotificationConfig struct {
	SMTP      SMTPConfigInput
	From      string
	To        []string
	Events    []string
	BatchMode string
}

// SMTPConfigInput mirrors the config package SMTP type.
type SMTPConfigInput struct {
	Host     string
	Port     int
	Username string
	Password string
	TLS      bool
}

// CreateEmailProvider creates an email provider from config notification settings.
func CreateEmailProvider(config *EmailNotificationConfig) (*EmailProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("email config is nil")
	}

	emailConfig := EmailConfig{
		SMTP: SMTPConfig{
			Host:     config.SMTP.Host,
			Port:     config.SMTP.Port,
			Username: config.SMTP.Username,
			Password: config.SMTP.Password,
			TLS:      config.SMTP.TLS,
		},
		From:      config.From,
		To:        config.To,
		Events:    config.Events,
		BatchMode: config.BatchMode,
	}

	provider := NewEmailProvider(emailConfig)
	if err := provider.Validate(context.Background()); err != nil {
		return nil, err
	}

	return provider, nil
}
