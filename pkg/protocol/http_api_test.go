package protocol

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPAPIAdapterExecute(t *testing.T) {
	t.Parallel()

	t.Run("successful create operation", func(t *testing.T) {
		t.Parallel()

		// Create a test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != "POST" {
				t.Errorf("Expected POST method, got %s", r.Method)
			}
			if r.URL.Path != "/api/keys" {
				t.Errorf("Expected /api/keys path, got %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("Expected Bearer auth, got %s", r.Header.Get("Authorization"))
			}

			// Send response
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "key-123",
				"api_key": "generated-key",
				"status":  "active",
			})
		}))
		defer server.Close()

		adapter := NewHTTPAPIAdapter()
		ctx := context.Background()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": server.URL,
			},
			Auth: map[string]string{
				"type":  "bearer",
				"value": "test-token",
			},
			ServiceConfig: map[string]interface{}{
				"endpoints": map[string]interface{}{
					"create": map[string]interface{}{
						"path": "/api/keys",
						"body": `{"name": "{{.Parameters.name}}"}`,
					},
				},
			},
		}

		operation := Operation{
			Action: "create",
			Target: "api_key",
			Parameters: map[string]interface{}{
				"name": "test-key",
			},
		}

		result, err := adapter.Execute(ctx, operation, config)
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		if result.Data["id"] != "key-123" {
			t.Errorf("Expected id key-123, got %v", result.Data["id"])
		}
		if result.Metadata["status_code"] != "201" {
			t.Errorf("Expected status 201, got %s", result.Metadata["status_code"])
		}
	})

	t.Run("successful verify operation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Errorf("Expected GET method, got %s", r.Method)
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"valid":      true,
				"expires_at": "2025-12-31T23:59:59Z",
			})
		}))
		defer server.Close()

		adapter := NewHTTPAPIAdapter()
		ctx := context.Background()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": server.URL,
			},
			ServiceConfig: map[string]interface{}{
				"endpoints": map[string]interface{}{
					"verify": map[string]interface{}{
						"path": "/api/keys/verify",
					},
				},
			},
		}

		operation := Operation{
			Action: "verify",
			Target: "api_key",
		}

		result, err := adapter.Execute(ctx, operation, config)
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success")
		}
		if result.Data["valid"] != true {
			t.Errorf("Expected valid=true, got %v", result.Data["valid"])
		}
	})

	t.Run("api key auth in header", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-Custom-Key")
			if apiKey != "my-api-key" {
				t.Errorf("Expected API key in X-Custom-Key header, got %s", apiKey)
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}))
		defer server.Close()

		adapter := NewHTTPAPIAdapter()
		ctx := context.Background()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": server.URL,
			},
			Auth: map[string]string{
				"type":        "api_key",
				"value":       "my-api-key",
				"header_name": "X-Custom-Key",
			},
			ServiceConfig: map[string]interface{}{
				"endpoints": map[string]interface{}{
					"list": map[string]interface{}{
						"path": "/api/list",
					},
				},
			},
		}

		operation := Operation{
			Action: "list",
			Target: "keys",
		}

		_, err := adapter.Execute(ctx, operation, config)
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}
	})

	t.Run("api key auth in query param", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.URL.Query().Get("key")
			if apiKey != "my-api-key" {
				t.Errorf("Expected API key in query param 'key', got %s", apiKey)
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}))
		defer server.Close()

		adapter := NewHTTPAPIAdapter()
		ctx := context.Background()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": server.URL,
			},
			Auth: map[string]string{
				"type":       "api_key",
				"value":      "my-api-key",
				"location":   "query",
				"param_name": "key",
			},
			ServiceConfig: map[string]interface{}{
				"endpoints": map[string]interface{}{
					"list": map[string]interface{}{
						"path": "/api/list",
					},
				},
			},
		}

		operation := Operation{
			Action: "list",
			Target: "keys",
		}

		_, err := adapter.Execute(ctx, operation, config)
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}
	})

	t.Run("basic auth", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if !ok {
				t.Error("Expected basic auth")
			}
			if username != "admin" || password != "secret123" {
				t.Errorf("Expected admin:secret123, got %s:%s", username, password)
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}))
		defer server.Close()

		adapter := NewHTTPAPIAdapter()
		ctx := context.Background()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": server.URL,
			},
			Auth: map[string]string{
				"type":     "basic",
				"username": "admin",
				"value":    "secret123",
			},
			ServiceConfig: map[string]interface{}{
				"endpoints": map[string]interface{}{
					"list": map[string]interface{}{
						"path": "/api/list",
					},
				},
			},
		}

		operation := Operation{
			Action: "list",
			Target: "keys",
		}

		_, err := adapter.Execute(ctx, operation, config)
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}
	})

	t.Run("server error with retry", func(t *testing.T) {
		t.Parallel()

		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("Server error"))
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}))
		defer server.Close()

		adapter := NewHTTPAPIAdapter()
		ctx := context.Background()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": server.URL,
			},
			Retries: 3,
			ServiceConfig: map[string]interface{}{
				"endpoints": map[string]interface{}{
					"list": map[string]interface{}{
						"path": "/api/list",
					},
				},
			},
		}

		operation := Operation{
			Action: "list",
			Target: "keys",
		}

		result, err := adapter.Execute(ctx, operation, config)
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
		if !result.Success {
			t.Errorf("Expected success after retries")
		}
	})

	t.Run("client error no retry", func(t *testing.T) {
		t.Parallel()

		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Bad request"))
		}))
		defer server.Close()

		adapter := NewHTTPAPIAdapter()
		ctx := context.Background()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": server.URL,
			},
			Retries: 5,
			ServiceConfig: map[string]interface{}{
				"endpoints": map[string]interface{}{
					"create": map[string]interface{}{
						"path": "/api/keys",
					},
				},
			},
		}

		operation := Operation{
			Action: "create",
			Target: "keys",
			Parameters: map[string]interface{}{
				"name": "test",
			},
		}

		_, err := adapter.Execute(ctx, operation, config)
		if err == nil {
			t.Error("Expected error for client error response")
		}

		// Should not retry on 4xx errors
		if attempts != 1 {
			t.Errorf("Expected 1 attempt (no retry on 4xx), got %d", attempts)
		}
	})

	t.Run("non-json response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("plain text response"))
		}))
		defer server.Close()

		adapter := NewHTTPAPIAdapter()
		ctx := context.Background()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": server.URL,
			},
			ServiceConfig: map[string]interface{}{
				"endpoints": map[string]interface{}{
					"list": map[string]interface{}{
						"path": "/api/list",
					},
				},
			},
		}

		operation := Operation{
			Action: "list",
			Target: "keys",
		}

		result, err := adapter.Execute(ctx, operation, config)
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		if result.Data["response"] != "plain text response" {
			t.Errorf("Expected plain text in response field, got %v", result.Data["response"])
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(5 * time.Second) // Slow response
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		adapter := NewHTTPAPIAdapter()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": server.URL,
			},
			ServiceConfig: map[string]interface{}{
				"endpoints": map[string]interface{}{
					"list": map[string]interface{}{
						"path": "/api/list",
					},
				},
			},
		}

		operation := Operation{
			Action: "list",
			Target: "keys",
		}

		_, err := adapter.Execute(ctx, operation, config)
		if err == nil {
			t.Error("Expected error due to context timeout")
		}
	})

	t.Run("missing endpoint configuration", func(t *testing.T) {
		t.Parallel()

		adapter := NewHTTPAPIAdapter()
		ctx := context.Background()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": "https://api.example.com",
			},
			ServiceConfig: map[string]interface{}{
				"endpoints": map[string]interface{}{}, // No endpoints
			},
		}

		operation := Operation{
			Action: "create",
			Target: "keys",
		}

		_, err := adapter.Execute(ctx, operation, config)
		if err == nil {
			t.Error("Expected error for missing endpoint configuration")
		}
	})

	t.Run("invalid auth type", func(t *testing.T) {
		t.Parallel()

		adapter := NewHTTPAPIAdapter()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": "https://api.example.com",
			},
			Auth: map[string]string{
				"type":  "unknown",
				"value": "test",
			},
			ServiceConfig: map[string]interface{}{
				"endpoints": map[string]interface{}{
					"list": map[string]interface{}{
						"path": "/api/list",
					},
				},
			},
		}

		err := adapter.Validate(config)
		if err == nil {
			t.Error("Expected validation error for unsupported auth type")
		}
	})

	t.Run("missing auth value", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		adapter := NewHTTPAPIAdapter()
		ctx := context.Background()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": server.URL,
			},
			Auth: map[string]string{
				"type": "bearer",
				// Missing value
			},
			ServiceConfig: map[string]interface{}{
				"endpoints": map[string]interface{}{
					"list": map[string]interface{}{
						"path": "/api/list",
					},
				},
			},
		}

		operation := Operation{
			Action: "list",
			Target: "keys",
		}

		_, err := adapter.Execute(ctx, operation, config)
		if err == nil {
			t.Error("Expected error for missing auth value")
		}
	})
}

func TestHTTPAPIAdapterHelpers(t *testing.T) {
	t.Parallel()

	adapter := NewHTTPAPIAdapter()

	t.Run("getHTTPMethod mapping", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			action   string
			expected string
		}{
			{"create", "POST"},
			{"verify", "GET"},
			{"rotate", "PUT"},
			{"revoke", "DELETE"},
			{"list", "GET"},
			{"unknown", "POST"}, // Default
		}

		for _, tt := range tests {
			t.Run(tt.action, func(t *testing.T) {
				op := Operation{Action: tt.action}
				method := adapter.getHTTPMethod(op)
				if method != tt.expected {
					t.Errorf("getHTTPMethod(%s) = %s, want %s", tt.action, method, tt.expected)
				}
			})
		}
	})

	t.Run("renderTemplate", func(t *testing.T) {
		t.Parallel()

		op := Operation{
			Action: "create",
			Target: "api_key",
			Parameters: map[string]interface{}{
				"name": "my-key",
				"env":  "prod",
			},
			Metadata: map[string]string{
				"user": "admin",
			},
		}

		template := "/api/{{.Target}}/{{.Action}}"
		result, err := adapter.renderTemplate(template, op)
		if err != nil {
			t.Fatalf("renderTemplate() failed: %v", err)
		}

		expected := "/api/api_key/create"
		if result != expected {
			t.Errorf("renderTemplate() = %s, want %s", result, expected)
		}
	})

	t.Run("renderTemplate with parameters", func(t *testing.T) {
		t.Parallel()

		op := Operation{
			Action: "verify",
			Target: "certificate",
			Parameters: map[string]interface{}{
				"id": "cert-123",
			},
		}

		template := "/api/certs/{{index .Parameters \"id\"}}"
		result, err := adapter.renderTemplate(template, op)
		if err != nil {
			t.Fatalf("renderTemplate() failed: %v", err)
		}

		expected := "/api/certs/cert-123"
		if result != expected {
			t.Errorf("renderTemplate() = %s, want %s", result, expected)
		}
	})

	t.Run("renderTemplate invalid syntax", func(t *testing.T) {
		t.Parallel()

		op := Operation{Action: "test"}
		template := "{{.Invalid"

		_, err := adapter.renderTemplate(template, op)
		if err == nil {
			t.Error("Expected error for invalid template syntax")
		}
	})
}

func TestHTTPAPIAdapterValidationEdgeCases(t *testing.T) {
	t.Parallel()

	adapter := NewHTTPAPIAdapter()

	t.Run("empty base_url", func(t *testing.T) {
		t.Parallel()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": "",
			},
		}

		err := adapter.Validate(config)
		if err == nil {
			t.Error("Expected error for empty base_url")
		}
	})

	t.Run("auth without type", func(t *testing.T) {
		t.Parallel()

		config := AdapterConfig{
			Connection: map[string]string{
				"base_url": "https://api.example.com",
			},
			Auth: map[string]string{
				"value": "test-token",
			},
		}

		err := adapter.Validate(config)
		if err == nil {
			t.Error("Expected error for auth without type")
		}
	})

	t.Run("valid auth types", func(t *testing.T) {
		t.Parallel()

		validAuthTypes := []string{"bearer", "api_key", "basic"}

		for _, authType := range validAuthTypes {
			config := AdapterConfig{
				Connection: map[string]string{
					"base_url": "https://api.example.com",
				},
				Auth: map[string]string{
					"type": authType,
				},
			}

			err := adapter.Validate(config)
			if err != nil {
				t.Errorf("Expected no error for auth type %s, got %v", authType, err)
			}
		}
	})
}
