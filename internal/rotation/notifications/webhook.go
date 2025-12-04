package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"
)

// RetryConfig holds retry configuration for webhooks.
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts (default: 3).
	MaxAttempts int

	// Backoff strategy: linear, exponential (default: exponential).
	Backoff string

	// InitialWait is the initial wait time between retries.
	InitialWait time.Duration
}

// WebhookConfig holds configuration for webhook notifications.
type WebhookConfig struct {
	// Name is a human-readable name for this webhook.
	Name string

	// URL is the webhook endpoint URL.
	URL string

	// Method is the HTTP method to use (default: POST).
	Method string

	// Headers are additional HTTP headers to include.
	Headers map[string]string

	// Events specifies which rotation events trigger notifications.
	// If empty, all events are sent.
	Events []string

	// PayloadTemplate is a Go template for the request body.
	// If empty, a default JSON payload is used.
	PayloadTemplate string

	// Retry configuration.
	Retry *RetryConfig

	// Timeout for the HTTP request.
	Timeout time.Duration
}

// WebhookProvider sends rotation notifications via HTTP webhooks.
type WebhookProvider struct {
	config   WebhookConfig
	client   *http.Client
	template *template.Template
}

// NewWebhookProvider creates a new webhook notification provider.
func NewWebhookProvider(config WebhookConfig) *WebhookProvider {
	// Set defaults
	if config.Method == "" {
		config.Method = "POST"
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.Retry == nil {
		config.Retry = &RetryConfig{
			MaxAttempts: 3,
			Backoff:     "exponential",
			InitialWait: 1 * time.Second,
		}
	}
	if config.Retry.MaxAttempts == 0 {
		config.Retry.MaxAttempts = 3
	}
	if config.Retry.Backoff == "" {
		config.Retry.Backoff = "exponential"
	}
	if config.Retry.InitialWait == 0 {
		config.Retry.InitialWait = 1 * time.Second
	}

	provider := &WebhookProvider{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}

	// Parse template if provided
	if config.PayloadTemplate != "" {
		tmpl, err := template.New("payload").Parse(config.PayloadTemplate)
		if err == nil {
			provider.template = tmpl
		}
	}

	return provider
}

// Name returns the provider name.
func (p *WebhookProvider) Name() string {
	if p.config.Name != "" {
		return "webhook:" + p.config.Name
	}
	return "webhook"
}

// SupportsEvent returns true if this provider handles the given event type.
func (p *WebhookProvider) SupportsEvent(eventType EventType) bool {
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
func (p *WebhookProvider) Validate(ctx context.Context) error {
	if p.config.URL == "" {
		return fmt.Errorf("URL is required")
	}

	parsed, err := url.Parse(p.config.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid URL: %s", p.config.URL)
	}

	// Validate method
	method := strings.ToUpper(p.config.Method)
	switch method {
	case "POST", "PUT", "PATCH", "":
		// Valid
	default:
		return fmt.Errorf("invalid method: %s (must be POST, PUT, or PATCH)", p.config.Method)
	}

	// Validate backoff strategy if retry is configured
	if p.config.Retry != nil && p.config.Retry.Backoff != "" {
		switch strings.ToLower(p.config.Retry.Backoff) {
		case "linear", "exponential", "fixed":
			// Valid
		default:
			return fmt.Errorf("invalid backoff strategy: %s (must be linear, exponential, or fixed)", p.config.Retry.Backoff)
		}
	}

	return nil
}

// Send sends a webhook notification for the given rotation event.
func (p *WebhookProvider) Send(ctx context.Context, event RotationEvent) error {
	payload, err := p.buildPayload(event)
	if err != nil {
		return fmt.Errorf("failed to build payload: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= p.config.Retry.MaxAttempts; attempt++ {
		err := p.doSend(ctx, payload)
		if err == nil {
			return nil
		}
		lastErr = err

		// Don't sleep after the last attempt
		if attempt < p.config.Retry.MaxAttempts {
			sleepDuration := p.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(sleepDuration):
				// Continue to next attempt
			}
		}
	}

	return fmt.Errorf("webhook failed after %d attempts: %w", p.config.Retry.MaxAttempts, lastErr)
}

// doSend performs a single HTTP request.
func (p *WebhookProvider) doSend(ctx context.Context, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(p.config.Method), p.config.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for key, value := range p.config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// buildPayload creates the request body.
func (p *WebhookProvider) buildPayload(event RotationEvent) ([]byte, error) {
	if p.template != nil {
		return p.buildCustomPayload(event)
	}
	return p.buildDefaultPayload(event)
}

// webhookTemplateData provides template-friendly access to event data.
type webhookTemplateData struct {
	Type        string
	Service     string
	Environment string
	Strategy    string
	Status      string
	Error       string
	Duration    string
	Timestamp   string
	Metadata    map[string]string
}

// buildCustomPayload uses the configured template.
func (p *WebhookProvider) buildCustomPayload(event RotationEvent) ([]byte, error) {
	data := webhookTemplateData{
		Type:        string(event.Type),
		Service:     event.Service,
		Environment: event.Environment,
		Strategy:    event.Strategy,
		Status:      string(event.Status),
		Duration:    event.Duration.String(),
		Timestamp:   event.Timestamp.Format(time.RFC3339),
		Metadata:    event.Metadata,
	}

	if event.Error != nil {
		data.Error = event.Error.Error()
	}

	var buf bytes.Buffer
	if err := p.template.Execute(&buf, data); err != nil {
		// Fall back to default payload on template error
		return p.buildDefaultPayload(event)
	}

	return buf.Bytes(), nil
}

// buildDefaultPayload creates the default JSON payload.
func (p *WebhookProvider) buildDefaultPayload(event RotationEvent) ([]byte, error) {
	payload := map[string]interface{}{
		"event":       string(event.Type),
		"service":     event.Service,
		"environment": event.Environment,
		"status":      string(event.Status),
		"timestamp":   event.Timestamp.Format(time.RFC3339),
	}

	if event.Strategy != "" {
		payload["strategy"] = event.Strategy
	}

	if event.Duration > 0 {
		payload["duration_seconds"] = event.Duration.Seconds()
	}

	if event.Error != nil {
		payload["error"] = event.Error.Error()
	}

	if len(event.Metadata) > 0 {
		payload["metadata"] = event.Metadata
	}

	return json.Marshal(payload)
}

// calculateBackoff calculates the sleep duration for the given attempt.
func (p *WebhookProvider) calculateBackoff(attempt int) time.Duration {
	initial := p.config.Retry.InitialWait

	switch strings.ToLower(p.config.Retry.Backoff) {
	case "linear":
		return initial * time.Duration(attempt)
	case "exponential":
		// 2^(attempt-1) * initial
		multiplier := 1 << (attempt - 1)
		return initial * time.Duration(multiplier)
	case "fixed":
		return initial
	default:
		return initial
	}
}

// WebhookNotificationConfig mirrors the config package type for internal use.
type WebhookNotificationConfig struct {
	Name            string
	URL             string
	Method          string
	Headers         map[string]string
	Events          []string
	PayloadTemplate string
	Retry           *WebhookRetryConfig
	TimeoutSeconds  int
}

// WebhookRetryConfig mirrors the config package retry type.
type WebhookRetryConfig struct {
	MaxAttempts int
	Backoff     string
}

// CreateWebhookProvider creates a webhook provider from config notification settings.
func CreateWebhookProvider(config *WebhookNotificationConfig) (*WebhookProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("webhook config is nil")
	}

	webhookConfig := WebhookConfig{
		Name:            config.Name,
		URL:             config.URL,
		Method:          config.Method,
		Headers:         config.Headers,
		Events:          config.Events,
		PayloadTemplate: config.PayloadTemplate,
	}

	if config.TimeoutSeconds > 0 {
		webhookConfig.Timeout = time.Duration(config.TimeoutSeconds) * time.Second
	}

	if config.Retry != nil {
		webhookConfig.Retry = &RetryConfig{
			MaxAttempts: config.Retry.MaxAttempts,
			Backoff:     config.Retry.Backoff,
		}
	}

	provider := NewWebhookProvider(webhookConfig)
	if err := provider.Validate(context.Background()); err != nil {
		return nil, err
	}

	return provider, nil
}
