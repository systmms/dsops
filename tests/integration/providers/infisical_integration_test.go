package providers_test

import (
	"context"
	"encoding/json"
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

// mockInfisicalServer creates a mock Infisical API server for integration testing.
// This tests the real HTTP client code without requiring a real Infisical instance.
func mockInfisicalServer(t *testing.T, secrets map[string]string) *httptest.Server {
	var authCallCount int32

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Authentication endpoint
		case r.Method == "POST" && r.URL.Path == "/api/v1/auth/universal-auth/login":
			atomic.AddInt32(&authCallCount, 1)

			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Verify credentials
			if body["clientId"] == "invalid" || body["clientSecret"] == "invalid" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message": "Invalid credentials"}`))
				return
			}

			response := map[string]interface{}{
				"accessToken": "test-token-12345",
				"expiresIn":   3600,
				"tokenType":   "Bearer",
			}
			_ = json.NewEncoder(w).Encode(response)

		// Get single secret endpoint
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v3/secrets/"):
			// Extract secret name from path
			secretName := strings.TrimPrefix(r.URL.Path, "/api/v3/secrets/")

			// Check auth header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message": "Unauthorized"}`))
				return
			}

			// Verify query params
			workspaceID := r.URL.Query().Get("workspaceId")
			environment := r.URL.Query().Get("environment")
			if workspaceID == "" || environment == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message": "Missing required query parameters"}`))
				return
			}

			// Look up secret
			value, exists := secrets[secretName]
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message": "Secret not found"}`))
				return
			}

			response := map[string]interface{}{
				"secret": map[string]interface{}{
					"_id":           "secret-id-123",
					"secretKey":     secretName,
					"secretValue":   value,
					"version":       1,
					"type":          "shared",
					"createdAt":     time.Now().Format(time.RFC3339),
					"updatedAt":     time.Now().Format(time.RFC3339),
					"secretComment": "",
					"tags":          []string{},
				},
			}
			_ = json.NewEncoder(w).Encode(response)

		// List secrets endpoint
		case r.Method == "GET" && r.URL.Path == "/api/v3/secrets":
			// Check auth header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message": "Unauthorized"}`))
				return
			}

			secretsList := make([]map[string]string, 0, len(secrets))
			for name := range secrets {
				secretsList = append(secretsList, map[string]string{"secretKey": name})
			}

			response := map[string]interface{}{
				"secrets": secretsList,
			}
			_ = json.NewEncoder(w).Encode(response)

		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Endpoint not found"}`))
		}
	}))
}

func TestInfisicalIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup mock secrets
	secrets := map[string]string{
		"DATABASE_URL":    "postgres://localhost:5432/testdb",
		"API_KEY":         "sk-test-12345",
		"JWT_SECRET":      "super-secret-jwt-key",
		"SPECIAL_CHARS":   "p@$$w0rd!#&*()[]{}|\\<>?",
		"UNICODE_SECRET":  "Hello 世界 🌍",
		"JSON_VALUE":      `{"nested": "value", "number": 42}`,
	}

	mockServer := mockInfisicalServer(t, secrets)
	defer mockServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create provider config pointing to mock server
	config := map[string]interface{}{
		"host":        mockServer.URL,
		"project_id":  "test-project-123",
		"environment": "development",
		"timeout":     "30s",
		"auth": map[string]interface{}{
			"method":        "machine_identity",
			"client_id":     "test-client-id",
			"client_secret": "test-client-secret",
		},
	}

	t.Run("basic_secret_retrieval", func(t *testing.T) {
		p, err := providers.NewInfisicalProvider("infisical-test", config)
		require.NoError(t, err, "Failed to create Infisical provider")

		ref := provider.Reference{Key: "DATABASE_URL"}
		secret, err := p.Resolve(ctx, ref)
		require.NoError(t, err, "Failed to resolve secret")

		assert.Equal(t, "postgres://localhost:5432/testdb", secret.Value)
	})

	t.Run("secret_not_found", func(t *testing.T) {
		p, err := providers.NewInfisicalProvider("infisical-test", config)
		require.NoError(t, err)

		ref := provider.Reference{Key: "NONEXISTENT_SECRET"}
		_, err = p.Resolve(ctx, ref)

		assert.Error(t, err, "Expected error for nonexistent secret")
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("special_characters_in_secret", func(t *testing.T) {
		p, err := providers.NewInfisicalProvider("infisical-test", config)
		require.NoError(t, err)

		ref := provider.Reference{Key: "SPECIAL_CHARS"}
		secret, err := p.Resolve(ctx, ref)
		require.NoError(t, err)

		assert.Equal(t, "p@$$w0rd!#&*()[]{}|\\<>?", secret.Value)
	})

	t.Run("unicode_secret", func(t *testing.T) {
		p, err := providers.NewInfisicalProvider("infisical-test", config)
		require.NoError(t, err)

		ref := provider.Reference{Key: "UNICODE_SECRET"}
		secret, err := p.Resolve(ctx, ref)
		require.NoError(t, err)

		assert.Equal(t, "Hello 世界 🌍", secret.Value)
	})

	t.Run("json_value_secret", func(t *testing.T) {
		p, err := providers.NewInfisicalProvider("infisical-test", config)
		require.NoError(t, err)

		ref := provider.Reference{Key: "JSON_VALUE"}
		secret, err := p.Resolve(ctx, ref)
		require.NoError(t, err)

		assert.Equal(t, `{"nested": "value", "number": 42}`, secret.Value)
	})

	t.Run("provider_validate", func(t *testing.T) {
		p, err := providers.NewInfisicalProvider("infisical-test", config)
		require.NoError(t, err)

		err = p.Validate(ctx)
		assert.NoError(t, err, "Validate should succeed with valid credentials")
	})

	t.Run("provider_capabilities", func(t *testing.T) {
		p, err := providers.NewInfisicalProvider("infisical-test", config)
		require.NoError(t, err)

		caps := p.Capabilities()

		assert.True(t, caps.SupportsVersioning, "Infisical should support versioning")
		assert.True(t, caps.SupportsMetadata, "Infisical should support metadata")
		assert.True(t, caps.RequiresAuth, "Infisical should require auth")
		assert.Contains(t, caps.AuthMethods, "machine_identity")
	})

	t.Run("multiple_secrets_parallel", func(t *testing.T) {
		p, err := providers.NewInfisicalProvider("infisical-test", config)
		require.NoError(t, err)

		secretKeys := []string{"DATABASE_URL", "API_KEY", "JWT_SECRET"}

		type result struct {
			key string
			val string
			err error
		}

		results := make(chan result, len(secretKeys))

		for _, key := range secretKeys {
			key := key
			go func() {
				ref := provider.Reference{Key: key}
				secret, err := p.Resolve(ctx, ref)
				if err != nil {
					results <- result{key: key, err: err}
					return
				}
				results <- result{key: key, val: secret.Value}
			}()
		}

		for i := 0; i < len(secretKeys); i++ {
			res := <-results
			assert.NoError(t, res.err, "Parallel resolution failed for %s", res.key)
			assert.NotEmpty(t, res.val, "Secret value should not be empty for %s", res.key)
		}
	})
}

func TestInfisicalIntegrationAuthFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockServer := mockInfisicalServer(t, map[string]string{})
	defer mockServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("invalid_credentials", func(t *testing.T) {
		config := map[string]interface{}{
			"host":        mockServer.URL,
			"project_id":  "test-project",
			"environment": "dev",
			"auth": map[string]interface{}{
				"method":        "machine_identity",
				"client_id":     "invalid",
				"client_secret": "invalid",
			},
		}

		p, err := providers.NewInfisicalProvider("infisical-test", config)
		require.NoError(t, err, "Provider creation should succeed")

		// Validate should fail with invalid credentials
		err = p.Validate(ctx)
		assert.Error(t, err, "Validate should fail with invalid credentials")
	})
}

func TestInfisicalIntegrationConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	secrets := map[string]string{
		"CONCURRENT_SECRET": "concurrent-test-value",
	}

	mockServer := mockInfisicalServer(t, secrets)
	defer mockServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	config := map[string]interface{}{
		"host":        mockServer.URL,
		"project_id":  "test-project",
		"environment": "dev",
		"auth": map[string]interface{}{
			"method":        "machine_identity",
			"client_id":     "test-client",
			"client_secret": "test-secret",
		},
	}

	p, err := providers.NewInfisicalProvider("infisical-test", config)
	require.NoError(t, err)

	// Run 50 concurrent resolutions
	numGoroutines := 50
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			ref := provider.Reference{Key: "CONCURRENT_SECRET"}
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

func TestInfisicalIntegrationDescribe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	secrets := map[string]string{
		"DESCRIBE_TEST": "test-value",
	}

	mockServer := mockInfisicalServer(t, secrets)
	defer mockServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := map[string]interface{}{
		"host":        mockServer.URL,
		"project_id":  "test-project",
		"environment": "dev",
		"auth": map[string]interface{}{
			"method":        "machine_identity",
			"client_id":     "test-client",
			"client_secret": "test-secret",
		},
	}

	t.Run("describe_existing_secret", func(t *testing.T) {
		p, err := providers.NewInfisicalProvider("infisical-test", config)
		require.NoError(t, err)

		ref := provider.Reference{Key: "DESCRIBE_TEST"}
		metadata, err := p.Describe(ctx, ref)
		require.NoError(t, err)

		assert.True(t, metadata.Exists, "Secret should exist")
	})

	t.Run("describe_nonexistent_secret", func(t *testing.T) {
		p, err := providers.NewInfisicalProvider("infisical-test", config)
		require.NoError(t, err)

		ref := provider.Reference{Key: "NONEXISTENT"}
		metadata, err := p.Describe(ctx, ref)
		require.NoError(t, err)

		assert.False(t, metadata.Exists, "Secret should not exist")
	})
}
