package rotation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/systmms/dsops/internal/logging"
)

func TestWebhookRotator_SupportsSecret(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewWebhookRotator(logger)

	tests := []struct {
		name     string
		secret   SecretInfo
		expected bool
	}{
		{
			name: "supports_with_webhook_url",
			secret: SecretInfo{
				Metadata: map[string]string{
					"webhook_url": "http://example.com/rotate",
				},
			},
			expected: true,
		},
		{
			name: "supports_with_endpoint",
			secret: SecretInfo{
				Metadata: map[string]string{
					"endpoint": "http://example.com/rotate",
				},
			},
			expected: true,
		},
		{
			name: "no_webhook_url_or_endpoint",
			secret: SecretInfo{
				Metadata: map[string]string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rotator.SupportsSecret(context.Background(), tt.secret)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestWebhookRotator_Rotate(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewWebhookRotator(logger)

	// Create test server
	successResponse := WebhookResponse{
		Success: true,
		Message: "Rotation successful",
		NewSecretRef: &SecretReference{
			Provider: "test-store",
			Key:      "new/secret/path",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		var webhookReq WebhookRequest
		if err := json.NewDecoder(r.Body).Decode(&webhookReq); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Verify request contents
		if webhookReq.Action != "rotate" {
			t.Errorf("expected action rotate, got %s", webhookReq.Action)
		}

		// Send response
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(successResponse)
	}))
	defer server.Close()

	// Test rotation
	request := RotationRequest{
		Secret: SecretInfo{
			Key:        "test-secret",
			SecretType: "api-key",
			Provider:   "webhook",
			Metadata: map[string]string{
				"webhook_url": server.URL,
			},
		},
		DryRun: false,
		Force:  false,
	}

	ctx := context.Background()
	result, err := rotator.Rotate(ctx, request)
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	if result.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, result.Status)
	}
	if result.NewSecretRef == nil {
		t.Error("expected new secret ref")
	} else {
		if result.NewSecretRef.Provider != "test-store" {
			t.Errorf("expected provider test-store, got %s", result.NewSecretRef.Provider)
		}
		if result.NewSecretRef.Key != "new/secret/path" {
			t.Errorf("expected key new/secret/path, got %s", result.NewSecretRef.Key)
		}
	}
}

func TestWebhookRotator_RotateWithError(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewWebhookRotator(logger)

	// Create test server that returns error
	errorResponse := WebhookResponse{
		Success: false,
		Error:   "Rotation failed: invalid credentials",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(errorResponse)
	}))
	defer server.Close()

	request := RotationRequest{
		Secret: SecretInfo{
			Key:        "test-secret",
			SecretType: "api-key",
			Provider:   "webhook",
			Metadata: map[string]string{
				"webhook_url": server.URL,
			},
		},
	}

	ctx := context.Background()
	result, err := rotator.Rotate(ctx, request)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
	if result.Error != "Rotation failed: invalid credentials" {
		t.Errorf("expected error message 'Rotation failed: invalid credentials', got %s", result.Error)
	}
}

func TestWebhookRotator_Verify(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewWebhookRotator(logger)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var webhookReq WebhookRequest
		_ = json.NewDecoder(r.Body).Decode(&webhookReq)

		if webhookReq.Action != "verify" {
			t.Errorf("expected action verify, got %s", webhookReq.Action)
		}

		response := WebhookResponse{
			Success: true,
			Message: "Verification successful",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	request := VerificationRequest{
		Secret: SecretInfo{
			Key: "test-secret",
			Metadata: map[string]string{
				"webhook_url": server.URL,
			},
		},
		NewSecretRef: SecretReference{
			Provider: "test-store",
			Key:      "new/secret/path",
		},
	}

	ctx := context.Background()
	err := rotator.Verify(ctx, request)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}
}

func TestWebhookRotator_GetStatus(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewWebhookRotator(logger)

	// Test without webhook URL
	secret := SecretInfo{
		Key:      "test-secret",
		Metadata: map[string]string{},
	}

	ctx := context.Background()
	status, err := rotator.GetStatus(ctx, secret)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}

	if status.Status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, status.Status)
	}
	if !status.CanRotate {
		t.Error("expected CanRotate to be true")
	}

	// Test with webhook URL
	lastRotated := time.Now().Add(-24 * time.Hour)
	nextRotation := time.Now().Add(24 * time.Hour)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := WebhookResponse{
			Success: true,
			Metadata: map[string]interface{}{
				"status":        string(StatusCompleted),
				"can_rotate":    true,
				"reason":        "Ready for rotation",
				"last_rotated":  lastRotated.Format(time.RFC3339),
				"next_rotation": nextRotation.Format(time.RFC3339),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	secret.Metadata["webhook_url"] = server.URL
	status, err = rotator.GetStatus(ctx, secret)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}

	if status.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, status.Status)
	}
	if !status.CanRotate {
		t.Error("expected CanRotate to be true")
	}
	if status.Reason != "Ready for rotation" {
		t.Errorf("expected reason 'Ready for rotation', got %s", status.Reason)
	}
}

func TestWebhookRotator_ServiceInstanceConfig(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewWebhookRotator(logger)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for auth header from service instance config
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Authorization 'Bearer test-token', got %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("expected X-Custom-Header 'custom-value', got %s", r.Header.Get("X-Custom-Header"))
		}

		response := WebhookResponse{
			Success: true,
			Message: "Rotation successful",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Test with service instance config
	request := RotationRequest{
		Secret: SecretInfo{
			Key:        "test-secret",
			SecretType: "api-key",
			Provider:   "webhook",
			Metadata:   map[string]string{},
		},
		Config: map[string]interface{}{
			"endpoint": server.URL,
			"auth":     "Bearer test-token",
			"headers": map[string]interface{}{
				"X-Custom-Header": "custom-value",
			},
		},
	}

	ctx := context.Background()
	result, err := rotator.Rotate(ctx, request)
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	if result.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, result.Status)
	}
}