package rotation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/systmms/dsops/internal/logging"
)

// WebhookRotator implements rotation via HTTP webhook calls
type WebhookRotator struct {
	logger *logging.Logger
	client *http.Client
}

// NewWebhookRotator creates a new webhook rotation strategy
func NewWebhookRotator(logger *logging.Logger) *WebhookRotator {
	return &WebhookRotator{
		logger: logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the strategy name
func (w *WebhookRotator) Name() string {
	return "webhook"
}

// SupportsSecret checks if this strategy can rotate the given secret
func (w *WebhookRotator) SupportsSecret(ctx context.Context, secret SecretInfo) bool {
	// Webhook strategy can support secrets that have:
	// 1. webhook_url in metadata (legacy support)
	// 2. endpoint in metadata (from service instance)
	_, hasURL := secret.Metadata["webhook_url"]
	_, hasEndpoint := secret.Metadata["endpoint"]
	return hasURL || hasEndpoint
}

// WebhookRequest represents the request sent to the webhook
type WebhookRequest struct {
	Action       string                 `json:"action"`
	SecretInfo   SecretInfo             `json:"secret_info"`
	DryRun       bool                   `json:"dry_run"`
	Force        bool                   `json:"force"`
	NewValue     *NewSecretValue        `json:"new_value,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
}

// WebhookResponse represents the expected response from the webhook
type WebhookResponse struct {
	Success      bool                   `json:"success"`
	Message      string                 `json:"message,omitempty"`
	NewSecretRef *SecretReference       `json:"new_secret_ref,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Warnings     []string               `json:"warnings,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Rotate performs rotation via webhook call
func (w *WebhookRotator) Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error) {
	auditTrail := []AuditEntry{
		{
			Timestamp: time.Now(),
			Action:    "webhook_rotation_started",
			Component: "webhook_rotator",
			Status:    "info",
			Message:   "Starting webhook rotation",
			Details: map[string]interface{}{
				"secret_key": logging.Secret(request.Secret.Key),
				"dry_run":    request.DryRun,
			},
		},
	}

	// Get webhook URL from metadata or config (enhanced with service instance data)
	var webhookURL string
	var ok bool
	
	// First try legacy webhook_url in metadata
	webhookURL, ok = request.Secret.Metadata["webhook_url"]
	
	// Then try endpoint from service instance config
	if !ok && request.Config != nil {
		if endpoint, exists := request.Config["endpoint"]; exists {
			if endpointStr, isString := endpoint.(string); isString {
				webhookURL = endpointStr
				ok = true
				w.logger.Debug("Using service instance endpoint as webhook URL: %s", webhookURL)
			}
		}
	}
	
	if !ok {
		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      "webhook_url or endpoint not found in secret metadata or config",
			AuditTrail: auditTrail,
		}, fmt.Errorf("webhook_url or endpoint not found in secret metadata or config")
	}

	w.logger.Info("Starting webhook rotation for %s", logging.Secret(request.Secret.Key))

	// Prepare webhook request
	webhookReq := WebhookRequest{
		Action:     "rotate",
		SecretInfo: request.Secret,
		DryRun:     request.DryRun,
		Force:      request.Force,
		NewValue:   request.NewValue,
		Config:     request.Config,
		Timestamp:  time.Now(),
	}

	// Call webhook
	response, err := w.callWebhook(ctx, webhookURL, webhookReq)
	if err != nil {
		auditTrail = append(auditTrail, AuditEntry{
			Timestamp: time.Now(),
			Action:    "webhook_call_failed",
			Component: "webhook_rotator",
			Status:    "error",
			Message:   "Failed to call webhook",
			Error:     err.Error(),
		})

		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      fmt.Sprintf("webhook call failed: %v", err),
			AuditTrail: auditTrail,
		}, err
	}

	// Process response
	if !response.Success {
		auditTrail = append(auditTrail, AuditEntry{
			Timestamp: time.Now(),
			Action:    "webhook_rotation_failed",
			Component: "webhook_rotator",
			Status:    "error",
			Message:   "Webhook returned failure",
			Error:     response.Error,
		})

		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      response.Error,
			Warnings:   response.Warnings,
			AuditTrail: auditTrail,
		}, fmt.Errorf("webhook returned failure: %s", response.Error)
	}

	// Build result
	rotatedAt := time.Now()
	auditTrail = append(auditTrail, AuditEntry{
		Timestamp: rotatedAt,
		Action:    "webhook_rotation_completed",
		Component: "webhook_rotator",
		Status:    "info",
		Message:   "Webhook rotation completed successfully",
		Details:   response.Metadata,
	})

	w.logger.Info("Successfully completed webhook rotation for %s", logging.Secret(request.Secret.Key))

	return &RotationResult{
		Secret:       request.Secret,
		Status:       StatusCompleted,
		NewSecretRef: response.NewSecretRef,
		RotatedAt:    &rotatedAt,
		Warnings:     response.Warnings,
		AuditTrail:   auditTrail,
	}, nil
}

// Verify performs verification via webhook call
func (w *WebhookRotator) Verify(ctx context.Context, request VerificationRequest) error {
	webhookURL, ok := request.Secret.Metadata["webhook_url"]
	if !ok {
		return fmt.Errorf("webhook_url not found in secret metadata")
	}

	// Allow custom verification endpoint
	if verifyURL, ok := request.Secret.Metadata["webhook_verify_url"]; ok {
		webhookURL = verifyURL
	}

	w.logger.Debug("Verifying via webhook for %s", logging.Secret(request.Secret.Key))

	webhookReq := WebhookRequest{
		Action: "verify",
		SecretInfo: request.Secret,
		Config: map[string]interface{}{
			"new_secret_ref": request.NewSecretRef,
			"tests":          request.Tests,
		},
		Timestamp: time.Now(),
	}

	response, err := w.callWebhook(ctx, webhookURL, webhookReq)
	if err != nil {
		return fmt.Errorf("webhook verification failed: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("webhook verification failed: %s", response.Error)
	}

	return nil
}

// Rollback performs rollback via webhook call
func (w *WebhookRotator) Rollback(ctx context.Context, request RollbackRequest) error {
	webhookURL, ok := request.Secret.Metadata["webhook_url"]
	if !ok {
		return fmt.Errorf("webhook_url not found in secret metadata")
	}

	w.logger.Info("Rolling back via webhook for %s", logging.Secret(request.Secret.Key))

	webhookReq := WebhookRequest{
		Action: "rollback",
		SecretInfo: request.Secret,
		Config: map[string]interface{}{
			"old_secret_ref": request.OldSecretRef,
			"reason":         request.Reason,
		},
		Timestamp: time.Now(),
	}

	response, err := w.callWebhook(ctx, webhookURL, webhookReq)
	if err != nil {
		return fmt.Errorf("webhook rollback failed: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("webhook rollback failed: %s", response.Error)
	}

	return nil
}

// GetStatus performs status check via webhook call
func (w *WebhookRotator) GetStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
	webhookURL, ok := secret.Metadata["webhook_url"]
	if !ok {
		// If no webhook URL, assume rotation is always possible
		return &RotationStatusInfo{
			Status:    StatusPending,
			CanRotate: true,
			Reason:    "Webhook rotation available",
		}, nil
	}

	// Allow custom status endpoint
	if statusURL, ok := secret.Metadata["webhook_status_url"]; ok {
		webhookURL = statusURL
	}

	webhookReq := WebhookRequest{
		Action:     "status",
		SecretInfo: secret,
		Timestamp:  time.Now(),
	}

	response, err := w.callWebhook(ctx, webhookURL, webhookReq)
	if err != nil {
		// Don't fail completely if status check fails
		w.logger.Warn("Failed to get webhook status for %s: %v", logging.Secret(secret.Key), err)
		return &RotationStatusInfo{
			Status:    StatusPending,
			CanRotate: true,
			Reason:    "Status check failed, assuming rotation possible",
		}, nil
	}

	// Extract status info from metadata
	statusInfo := &RotationStatusInfo{
		Status:    StatusPending,
		CanRotate: true,
	}

	if response.Metadata != nil {
		if status, ok := response.Metadata["status"].(string); ok {
			statusInfo.Status = RotationStatus(status)
		}
		if canRotate, ok := response.Metadata["can_rotate"].(bool); ok {
			statusInfo.CanRotate = canRotate
		}
		if reason, ok := response.Metadata["reason"].(string); ok {
			statusInfo.Reason = reason
		}
		if lastRotated, ok := response.Metadata["last_rotated"].(string); ok {
			if t, err := time.Parse(time.RFC3339, lastRotated); err == nil {
				statusInfo.LastRotated = &t
			}
		}
		if nextRotation, ok := response.Metadata["next_rotation"].(string); ok {
			if t, err := time.Parse(time.RFC3339, nextRotation); err == nil {
				statusInfo.NextRotation = &t
			}
		}
	}

	return statusInfo, nil
}

// callWebhook makes the HTTP request to the webhook
func (w *WebhookRotator) callWebhook(ctx context.Context, url string, request WebhookRequest) (*WebhookResponse, error) {
	// Marshal request
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "dsops-rotation/1.0")

	// Add any custom headers from metadata (legacy support)
	if authHeader, ok := request.SecretInfo.Metadata["webhook_auth_header"]; ok {
		httpReq.Header.Set("Authorization", authHeader)
	}
	
	// Add auth from service instance config (enhanced support)
	if request.Config != nil {
		if auth, exists := request.Config["auth"]; exists {
			if authStr, isString := auth.(string); isString {
				// Support various auth formats from service instances
				httpReq.Header.Set("Authorization", authStr)
				w.logger.Debug("Using service instance auth for webhook request")
			}
		}
		
		// Support additional headers from config
		if headers, exists := request.Config["headers"]; exists {
			if headersMap, isMap := headers.(map[string]interface{}); isMap {
				for k, v := range headersMap {
					if vStr, isString := v.(string); isString {
						httpReq.Header.Set(k, vStr)
					}
				}
			}
		}
	}

	// Send request
	resp, err := w.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var webhookResp WebhookResponse
	if err := json.Unmarshal(respBody, &webhookResp); err != nil {
		return nil, fmt.Errorf("failed to parse webhook response: %w", err)
	}

	return &webhookResp, nil
}