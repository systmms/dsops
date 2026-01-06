package providers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
)

// mockAkeylessServer creates a mock Akeyless API server for integration testing.
// This tests the real SDK client code without requiring a real Akeyless account.
// The Akeyless SDK communicates via REST API, which we mock here.
func mockAkeylessServer(t *testing.T, secrets map[string]string) *httptest.Server {
	var authCallCount int32

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Read body for all POST requests
		var body map[string]interface{}
		if r.Method == "POST" {
			bodyBytes, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(bodyBytes, &body)
		}

		switch {
		// Authentication endpoint
		case r.Method == "POST" && r.URL.Path == "/auth":
			atomic.AddInt32(&authCallCount, 1)

			accessID, _ := body["access-id"].(string)
			accessKey, _ := body["access-key"].(string)

			// Verify credentials
			if accessID == "invalid" || accessKey == "invalid" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error": "Authentication failed"}`))
				return
			}

			response := map[string]interface{}{
				"token": "akeyless-token-12345",
			}
			_ = json.NewEncoder(w).Encode(response)

		// Get secret value endpoint
		case r.Method == "POST" && r.URL.Path == "/get-secret-value":
			token, _ := body["token"].(string)
			if token == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error": "Missing token"}`))
				return
			}

			// Get names from body - SDK sends as array
			names, ok := body["names"].([]interface{})
			if !ok || len(names) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error": "Missing names"}`))
				return
			}

			// Build response with all requested secrets
			response := make(map[string]string)
			for _, nameInterface := range names {
				name, _ := nameInterface.(string)
				value, exists := secrets[name]
				if exists {
					response[name] = value
				}
			}

			if len(response) == 0 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error": "Secret not found"}`))
				return
			}

			_ = json.NewEncoder(w).Encode(response)

		// Describe item endpoint
		case r.Method == "POST" && r.URL.Path == "/describe-item":
			token, _ := body["token"].(string)
			if token == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error": "Missing token"}`))
				return
			}

			name, _ := body["name"].(string)
			_, exists := secrets[name]

			if !exists {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error": "Item not found"}`))
				return
			}

			modTime := time.Now().Format(time.RFC3339)
			response := map[string]interface{}{
				"item_name":         name,
				"item_type":         "STATIC_SECRET",
				"last_version":      1,
				"modification_date": modTime,
				"item_tags":         []string{},
			}
			_ = json.NewEncoder(w).Encode(response)

		// List items endpoint
		case r.Method == "POST" && r.URL.Path == "/list-items":
			token, _ := body["token"].(string)
			if token == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error": "Missing token"}`))
				return
			}

			items := make([]map[string]interface{}, 0, len(secrets))
			for name := range secrets {
				items = append(items, map[string]interface{}{
					"item_name": name,
					"item_type": "STATIC_SECRET",
				})
			}

			response := map[string]interface{}{
				"items": items,
			}
			_ = json.NewEncoder(w).Encode(response)

		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "Endpoint not found"}`))
		}
	}))
}

func TestAkeylessIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup mock secrets (Akeyless uses paths starting with /)
	secrets := map[string]string{
		"/prod/database/password":  "super-secret-password",
		"/prod/api/key":            "api-key-12345",
		"/staging/jwt/secret":      "jwt-secret-key",
		"/secrets/special-chars":   "p@$$w0rd!#&*()[]{}|\\<>?",
		"/secrets/unicode":         "Hello 世界 🌍",
		"/secrets/json":            `{"nested": "value", "number": 42}`,
	}

	mockServer := mockAkeylessServer(t, secrets)
	defer mockServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create provider config pointing to mock server
	config := map[string]interface{}{
		"access_id":   "p-test123",
		"gateway_url": mockServer.URL,
		"timeout":     "30s",
		"auth": map[string]interface{}{
			"method":     "api_key",
			"access_key": "test-access-key",
		},
	}

	t.Run("basic_secret_retrieval", func(t *testing.T) {
		p, err := providers.NewAkeylessProvider("akeyless-test", config)
		require.NoError(t, err, "Failed to create Akeyless provider")

		ref := provider.Reference{Key: "/prod/database/password"}
		secret, err := p.Resolve(ctx, ref)
		require.NoError(t, err, "Failed to resolve secret")

		assert.Equal(t, "super-secret-password", secret.Value)
	})

	t.Run("secret_without_leading_slash", func(t *testing.T) {
		p, err := providers.NewAkeylessProvider("akeyless-test", config)
		require.NoError(t, err)

		// Provider should normalize path to include leading slash
		ref := provider.Reference{Key: "prod/api/key"}
		secret, err := p.Resolve(ctx, ref)
		require.NoError(t, err)

		assert.Equal(t, "api-key-12345", secret.Value)
	})

	t.Run("secret_not_found", func(t *testing.T) {
		p, err := providers.NewAkeylessProvider("akeyless-test", config)
		require.NoError(t, err)

		ref := provider.Reference{Key: "/nonexistent/path"}
		_, err = p.Resolve(ctx, ref)

		assert.Error(t, err, "Expected error for nonexistent secret")
		// Check for either "not found" or SDK error message
		errStr := strings.ToLower(err.Error())
		hasExpectedError := strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "404") ||
			strings.Contains(errStr, "secret")
		assert.True(t, hasExpectedError, "Error should indicate not found: %s", err.Error())
	})

	t.Run("special_characters_in_secret", func(t *testing.T) {
		p, err := providers.NewAkeylessProvider("akeyless-test", config)
		require.NoError(t, err)

		ref := provider.Reference{Key: "/secrets/special-chars"}
		secret, err := p.Resolve(ctx, ref)
		require.NoError(t, err)

		assert.Equal(t, "p@$$w0rd!#&*()[]{}|\\<>?", secret.Value)
	})

	t.Run("unicode_secret", func(t *testing.T) {
		p, err := providers.NewAkeylessProvider("akeyless-test", config)
		require.NoError(t, err)

		ref := provider.Reference{Key: "/secrets/unicode"}
		secret, err := p.Resolve(ctx, ref)
		require.NoError(t, err)

		assert.Equal(t, "Hello 世界 🌍", secret.Value)
	})

	t.Run("json_value_secret", func(t *testing.T) {
		p, err := providers.NewAkeylessProvider("akeyless-test", config)
		require.NoError(t, err)

		ref := provider.Reference{Key: "/secrets/json"}
		secret, err := p.Resolve(ctx, ref)
		require.NoError(t, err)

		assert.Equal(t, `{"nested": "value", "number": 42}`, secret.Value)
	})

	t.Run("provider_validate", func(t *testing.T) {
		p, err := providers.NewAkeylessProvider("akeyless-test", config)
		require.NoError(t, err)

		err = p.Validate(ctx)
		assert.NoError(t, err, "Validate should succeed with valid credentials")
	})

	t.Run("provider_capabilities", func(t *testing.T) {
		p, err := providers.NewAkeylessProvider("akeyless-test", config)
		require.NoError(t, err)

		caps := p.Capabilities()

		assert.True(t, caps.SupportsVersioning, "Akeyless should support versioning")
		assert.True(t, caps.SupportsMetadata, "Akeyless should support metadata")
		assert.True(t, caps.RequiresAuth, "Akeyless should require auth")
		assert.Contains(t, caps.AuthMethods, "api_key")
		assert.Contains(t, caps.AuthMethods, "aws_iam")
	})

	t.Run("multiple_secrets_parallel", func(t *testing.T) {
		p, err := providers.NewAkeylessProvider("akeyless-test", config)
		require.NoError(t, err)

		secretPaths := []string{
			"/prod/database/password",
			"/prod/api/key",
			"/staging/jwt/secret",
		}

		type result struct {
			path string
			val  string
			err  error
		}

		results := make(chan result, len(secretPaths))

		for _, path := range secretPaths {
			path := path
			go func() {
				ref := provider.Reference{Key: path}
				secret, err := p.Resolve(ctx, ref)
				if err != nil {
					results <- result{path: path, err: err}
					return
				}
				results <- result{path: path, val: secret.Value}
			}()
		}

		for i := 0; i < len(secretPaths); i++ {
			res := <-results
			assert.NoError(t, res.err, "Parallel resolution failed for %s", res.path)
			assert.NotEmpty(t, res.val, "Secret value should not be empty for %s", res.path)
		}
	})
}

func TestAkeylessIntegrationAuthFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockServer := mockAkeylessServer(t, map[string]string{})
	defer mockServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("invalid_credentials", func(t *testing.T) {
		config := map[string]interface{}{
			"access_id":   "invalid",
			"gateway_url": mockServer.URL,
			"auth": map[string]interface{}{
				"method":     "api_key",
				"access_key": "invalid",
			},
		}

		p, err := providers.NewAkeylessProvider("akeyless-test", config)
		require.NoError(t, err, "Provider creation should succeed")

		// Validate should fail with invalid credentials
		err = p.Validate(ctx)
		assert.Error(t, err, "Validate should fail with invalid credentials")
	})
}

func TestAkeylessIntegrationConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	secrets := map[string]string{
		"/concurrent/secret": "concurrent-test-value",
	}

	mockServer := mockAkeylessServer(t, secrets)
	defer mockServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	config := map[string]interface{}{
		"access_id":   "p-test123",
		"gateway_url": mockServer.URL,
		"auth": map[string]interface{}{
			"method":     "api_key",
			"access_key": "test-key",
		},
	}

	p, err := providers.NewAkeylessProvider("akeyless-test", config)
	require.NoError(t, err)

	// Run 50 concurrent resolutions
	numGoroutines := 50
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			ref := provider.Reference{Key: "/concurrent/secret"}
			secret, err := p.Resolve(ctx, ref)
			if err != nil {
				results <- err
				return
			}

			if secret.Value != "concurrent-test-value" {
				results <- assert.AnError
				return
			}

			results <- nil
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		assert.NoError(t, err, "Concurrent resolution should succeed")
	}
}

func TestAkeylessIntegrationDescribe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	secrets := map[string]string{
		"/describe/test": "test-value",
	}

	mockServer := mockAkeylessServer(t, secrets)
	defer mockServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := map[string]interface{}{
		"access_id":   "p-test123",
		"gateway_url": mockServer.URL,
		"auth": map[string]interface{}{
			"method":     "api_key",
			"access_key": "test-key",
		},
	}

	t.Run("describe_existing_secret", func(t *testing.T) {
		p, err := providers.NewAkeylessProvider("akeyless-test", config)
		require.NoError(t, err)

		ref := provider.Reference{Key: "/describe/test"}
		metadata, err := p.Describe(ctx, ref)
		require.NoError(t, err)

		assert.True(t, metadata.Exists, "Secret should exist")
	})

	t.Run("describe_nonexistent_secret", func(t *testing.T) {
		p, err := providers.NewAkeylessProvider("akeyless-test", config)
		require.NoError(t, err)

		ref := provider.Reference{Key: "/nonexistent"}
		metadata, err := p.Describe(ctx, ref)
		require.NoError(t, err)

		assert.False(t, metadata.Exists, "Secret should not exist")
	})
}
