package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
)

// MockVaultClient implements VaultClient for testing
type MockVaultClient struct {
	ReadFunc         func(ctx context.Context, path string) (*VaultSecret, error)
	AuthenticateFunc func(ctx context.Context) error
	CloseFunc        func() error
}

func (m *MockVaultClient) Read(ctx context.Context, path string) (*VaultSecret, error) {
	if m.ReadFunc != nil {
		return m.ReadFunc(ctx, path)
	}
	return nil, nil
}

func (m *MockVaultClient) Authenticate(ctx context.Context) error {
	if m.AuthenticateFunc != nil {
		return m.AuthenticateFunc(ctx)
	}
	return nil
}

func (m *MockVaultClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestVaultProvider_Resolve_Success(t *testing.T) {
	t.Parallel()

	mockClient := &MockVaultClient{
		AuthenticateFunc: func(ctx context.Context) error {
			return nil
		},
		ReadFunc: func(ctx context.Context, path string) (*VaultSecret, error) {
			return &VaultSecret{
				Data: map[string]interface{}{
					"password": "secret123",
					"username": "admin",
				},
			}, nil
		},
	}

	p := &VaultProvider{
		name:   "test-vault",
		config: Config{Address: "http://localhost:8200"},
		client: mockClient,
		logger: logging.New(false, false),
	}

	ctx := context.Background()
	ref := provider.Reference{Key: "secret/data/myapp#password"}

	value, err := p.Resolve(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, "secret123", value.Value)
	assert.Equal(t, "secret/data/myapp", value.Metadata["path"])
}

func TestVaultProvider_Resolve_EntireSecret(t *testing.T) {
	t.Parallel()

	mockClient := &MockVaultClient{
		AuthenticateFunc: func(ctx context.Context) error { return nil },
		ReadFunc: func(ctx context.Context, path string) (*VaultSecret, error) {
			return &VaultSecret{
				Data: map[string]interface{}{
					"password": "secret123",
					"username": "admin",
				},
			}, nil
		},
	}

	p := &VaultProvider{
		name:   "test-vault",
		config: Config{Address: "http://localhost:8200"},
		client: mockClient,
		logger: logging.New(false, false),
	}

	ctx := context.Background()
	ref := provider.Reference{Key: "secret/data/myapp"} // No field specified

	value, err := p.Resolve(ctx, ref)
	require.NoError(t, err)

	// Should return JSON of entire secret
	var data map[string]interface{}
	err = json.Unmarshal([]byte(value.Value), &data)
	require.NoError(t, err)
	assert.Equal(t, "secret123", data["password"])
	assert.Equal(t, "admin", data["username"])
}

func TestVaultProvider_Resolve_FieldNotFound(t *testing.T) {
	t.Parallel()

	mockClient := &MockVaultClient{
		AuthenticateFunc: func(ctx context.Context) error { return nil },
		ReadFunc: func(ctx context.Context, path string) (*VaultSecret, error) {
			return &VaultSecret{
				Data: map[string]interface{}{
					"password": "secret123",
				},
			}, nil
		},
	}

	p := &VaultProvider{
		name:   "test-vault",
		config: Config{Address: "http://localhost:8200"},
		client: mockClient,
		logger: logging.New(false, false),
	}

	ctx := context.Background()
	ref := provider.Reference{Key: "secret/data/myapp#nonexistent"}

	_, err := p.Resolve(ctx, ref)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Field 'nonexistent' not found")
}

func TestVaultProvider_Resolve_SecretNotFound(t *testing.T) {
	t.Parallel()

	mockClient := &MockVaultClient{
		AuthenticateFunc: func(ctx context.Context) error { return nil },
		ReadFunc: func(ctx context.Context, path string) (*VaultSecret, error) {
			return nil, nil // Secret not found
		},
	}

	p := &VaultProvider{
		name:   "test-vault",
		config: Config{Address: "http://localhost:8200"},
		client: mockClient,
		logger: logging.New(false, false),
	}

	ctx := context.Background()
	ref := provider.Reference{Key: "secret/data/nonexistent"}

	_, err := p.Resolve(ctx, ref)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Secret not found")
}

func TestVaultProvider_Resolve_TypeConversions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		data     map[string]interface{}
		field    string
		expected string
	}{
		{
			name:     "string value",
			data:     map[string]interface{}{"field": "test"},
			field:    "field",
			expected: "test",
		},
		{
			name:     "integer value",
			data:     map[string]interface{}{"port": 5432},
			field:    "port",
			expected: "5432",
		},
		{
			name:     "float value",
			data:     map[string]interface{}{"rate": 3.14},
			field:    "rate",
			expected: "3.14",
		},
		{
			name:     "boolean value",
			data:     map[string]interface{}{"enabled": true},
			field:    "enabled",
			expected: "true",
		},
		{
			name:     "byte array",
			data:     map[string]interface{}{"data": []byte("binary")},
			field:    "data",
			expected: "binary",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockClient := &MockVaultClient{
				AuthenticateFunc: func(ctx context.Context) error { return nil },
				ReadFunc: func(ctx context.Context, path string) (*VaultSecret, error) {
					return &VaultSecret{Data: tc.data}, nil
				},
			}

			p := &VaultProvider{
				name:   "test-vault",
				config: Config{Address: "http://localhost:8200"},
				client: mockClient,
				logger: logging.New(false, false),
			}

			ctx := context.Background()
			ref := provider.Reference{Key: "secret/data/test#" + tc.field}

			value, err := p.Resolve(ctx, ref)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, value.Value)
		})
	}
}

func TestVaultProvider_Describe(t *testing.T) {
	t.Parallel()

	p := &VaultProvider{
		name: "test-vault",
		config: Config{
			Address:    "http://localhost:8200",
			Namespace:  "test-ns",
			AuthMethod: "token",
		},
	}

	ctx := context.Background()
	ref := provider.Reference{Key: "secret/data/myapp#password"}

	metadata, err := p.Describe(ctx, ref)
	require.NoError(t, err)

	assert.Equal(t, "vault-secret", metadata.Type)
	assert.Equal(t, "secret/data/myapp", metadata.Tags["path"])
	assert.Equal(t, "password", metadata.Tags["field"])
	assert.Equal(t, "http://localhost:8200", metadata.Tags["address"])
	assert.Equal(t, "test-ns", metadata.Tags["namespace"])
	assert.Equal(t, "token", metadata.Tags["auth_method"])
}

func TestVaultProvider_ParseReference(t *testing.T) {
	t.Parallel()

	p := &VaultProvider{}

	testCases := []struct {
		name          string
		key           string
		expectedPath  string
		expectedField string
		expectError   bool
	}{
		{
			name:          "path only",
			key:           "secret/data/myapp",
			expectedPath:  "secret/data/myapp",
			expectedField: "",
		},
		{
			name:          "path with field",
			key:           "secret/data/myapp#password",
			expectedPath:  "secret/data/myapp",
			expectedField: "password",
		},
		{
			name:        "empty key",
			key:         "",
			expectError: true,
		},
		{
			name:        "multiple hash signs",
			key:         "secret/data#field#extra",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path, field, err := p.parseReference(tc.key)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedPath, path)
				assert.Equal(t, tc.expectedField, field)
			}
		})
	}
}

func TestVaultProvider_Validate_TokenAuth(t *testing.T) {
	t.Parallel()

	mockClient := &MockVaultClient{
		AuthenticateFunc: func(ctx context.Context) error { return nil },
	}

	p := &VaultProvider{
		name: "test",
		config: Config{
			Address:    "http://localhost:8200",
			AuthMethod: "token",
			Token:      "test-token",
		},
		client: mockClient,
	}

	ctx := context.Background()
	err := p.Validate(ctx)
	assert.NoError(t, err)
}

func TestVaultProvider_Validate_MissingAddress(t *testing.T) {
	t.Parallel()

	p := &VaultProvider{
		name: "test",
		config: Config{
			Address:    "",
			AuthMethod: "token",
		},
	}

	ctx := context.Background()
	err := p.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "address")
}

func TestVaultProvider_Validate_MissingToken(t *testing.T) {
	t.Parallel()

	// Clear environment variable
	oldToken := os.Getenv("VAULT_TOKEN")
	_ = os.Unsetenv("VAULT_TOKEN")
	defer func() {
		if oldToken != "" {
			_ = os.Setenv("VAULT_TOKEN", oldToken)
		}
	}()

	p := &VaultProvider{
		name: "test",
		config: Config{
			Address:    "http://localhost:8200",
			AuthMethod: "token",
			Token:      "",
		},
	}

	ctx := context.Background()
	err := p.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token")
}

func TestVaultProvider_Validate_UserpassAuth(t *testing.T) {
	t.Parallel()

	mockClient := &MockVaultClient{
		AuthenticateFunc: func(ctx context.Context) error { return nil },
	}

	p := &VaultProvider{
		name: "test",
		config: Config{
			Address:          "http://localhost:8200",
			AuthMethod:       "userpass",
			UserpassUsername: "admin",
		},
		client: mockClient,
	}

	ctx := context.Background()
	err := p.Validate(ctx)
	assert.NoError(t, err)
}

func TestVaultProvider_Validate_UserpassMissingUsername(t *testing.T) {
	t.Parallel()

	p := &VaultProvider{
		name: "test",
		config: Config{
			Address:    "http://localhost:8200",
			AuthMethod: "userpass",
		},
	}

	ctx := context.Background()
	err := p.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "userpass_username")
}

func TestVaultProvider_Validate_LDAPAuth(t *testing.T) {
	t.Parallel()

	mockClient := &MockVaultClient{
		AuthenticateFunc: func(ctx context.Context) error { return nil },
	}

	p := &VaultProvider{
		name: "test",
		config: Config{
			Address:      "http://localhost:8200",
			AuthMethod:   "ldap",
			LDAPUsername: "admin",
		},
		client: mockClient,
	}

	ctx := context.Background()
	err := p.Validate(ctx)
	assert.NoError(t, err)
}

func TestVaultProvider_Validate_AWSAuth(t *testing.T) {
	t.Parallel()

	mockClient := &MockVaultClient{
		AuthenticateFunc: func(ctx context.Context) error { return nil },
	}

	p := &VaultProvider{
		name: "test",
		config: Config{
			Address:    "http://localhost:8200",
			AuthMethod: "aws",
			AWSRole:    "my-role",
		},
		client: mockClient,
	}

	ctx := context.Background()
	err := p.Validate(ctx)
	assert.NoError(t, err)
}

func TestVaultProvider_Validate_K8SAuth(t *testing.T) {
	t.Parallel()

	mockClient := &MockVaultClient{
		AuthenticateFunc: func(ctx context.Context) error { return nil },
	}

	p := &VaultProvider{
		name: "test",
		config: Config{
			Address:    "http://localhost:8200",
			AuthMethod: "k8s",
			K8SRole:    "my-role",
		},
		client: mockClient,
	}

	ctx := context.Background()
	err := p.Validate(ctx)
	assert.NoError(t, err)
}

func TestVaultProvider_Validate_UnsupportedAuthMethod(t *testing.T) {
	t.Parallel()

	p := &VaultProvider{
		name: "test",
		config: Config{
			Address:    "http://localhost:8200",
			AuthMethod: "unsupported",
		},
	}

	ctx := context.Background()
	err := p.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestVaultProvider_GetVaultErrorSuggestion(t *testing.T) {
	t.Parallel()

	p := &VaultProvider{
		config: Config{Address: "http://localhost:8200"},
	}

	testCases := []struct {
		errMsg   string
		contains string
	}{
		{"connection refused", "server is running"},
		{"permission denied", "token permissions"},
		{"invalid token", "expired or invalid"},
		{"namespace not found", "namespace configuration"},
		{"tls handshake failed", "TLS configuration"},
		{"auth failed", "Authentication failed"},
		{"unknown error", "dsops doctor"},
	}

	for _, tc := range testCases {
		t.Run(tc.errMsg, func(t *testing.T) {
			t.Parallel()

			suggestion := p.getVaultErrorSuggestion(assert.AnError)
			assert.NotEmpty(t, suggestion)
		})
	}
}

func TestHTTPVaultClient_AuthenticateToken(t *testing.T) {
	t.Parallel()

	client := &HTTPVaultClient{
		config: Config{Token: "test-token"},
	}

	err := client.authenticateToken()
	require.NoError(t, err)
	assert.Equal(t, "test-token", client.token)
}

func TestHTTPVaultClient_AuthenticateToken_FromEnv(t *testing.T) {
	t.Parallel()

	_ = os.Setenv("VAULT_TOKEN", "env-token")
	defer func() { _ = os.Unsetenv("VAULT_TOKEN") }()

	client := &HTTPVaultClient{
		config: Config{Token: ""},
	}

	err := client.authenticateToken()
	require.NoError(t, err)
	assert.Equal(t, "env-token", client.token)
}

func TestHTTPVaultClient_AuthenticateToken_NoToken(t *testing.T) {
	t.Parallel()

	oldToken := os.Getenv("VAULT_TOKEN")
	_ = os.Unsetenv("VAULT_TOKEN")
	defer func() {
		if oldToken != "" {
			_ = os.Setenv("VAULT_TOKEN", oldToken)
		}
	}()

	client := &HTTPVaultClient{
		config: Config{Token: ""},
	}

	err := client.authenticateToken()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no vault token")
}

func TestHTTPVaultClient_Read_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "test-token", r.Header.Get("X-Vault-Token"))
		assert.Contains(t, r.URL.Path, "secret/data/myapp")

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"data": map[string]interface{}{
					"password": "secret123",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &HTTPVaultClient{
		config: Config{Address: server.URL},
		token:  "test-token",
	}

	ctx := context.Background()
	secret, err := client.Read(ctx, "secret/data/myapp")
	require.NoError(t, err)
	assert.NotNil(t, secret)
}

func TestHTTPVaultClient_Read_NotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &HTTPVaultClient{
		config: Config{Address: server.URL},
		token:  "test-token",
	}

	ctx := context.Background()
	secret, err := client.Read(ctx, "secret/data/nonexistent")
	require.NoError(t, err)
	assert.Nil(t, secret)
}

func TestHTTPVaultClient_Read_Error(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("permission denied"))
	}))
	defer server.Close()

	client := &HTTPVaultClient{
		config: Config{Address: server.URL},
		token:  "test-token",
	}

	ctx := context.Background()
	_, err := client.Read(ctx, "secret/data/forbidden")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestHTTPVaultClient_Read_NotAuthenticated(t *testing.T) {
	t.Parallel()

	client := &HTTPVaultClient{
		config: Config{Address: "http://localhost:8200"},
		token:  "", // Not authenticated
	}

	ctx := context.Background()
	_, err := client.Read(ctx, "secret/data/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")
}

func TestHTTPVaultClient_Read_WithNamespace(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-namespace", r.Header.Get("X-Vault-Namespace"))

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"data": map[string]interface{}{
					"key": "value",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &HTTPVaultClient{
		config: Config{
			Address:   server.URL,
			Namespace: "test-namespace",
		},
		token: "test-token",
	}

	ctx := context.Background()
	_, err := client.Read(ctx, "secret/data/test")
	require.NoError(t, err)
}

func TestHTTPVaultClient_Close(t *testing.T) {
	t.Parallel()

	client := &HTTPVaultClient{
		token: "test-token",
	}

	err := client.Close()
	require.NoError(t, err)
	assert.Empty(t, client.token)
}

func TestHTTPVaultClient_PerformLogin(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		response := map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token": "new-token",
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &HTTPVaultClient{
		config: Config{Address: server.URL},
	}

	ctx := context.Background()
	authData := map[string]interface{}{"password": "secret"}
	err := client.performLogin(ctx, "auth/userpass/login/admin", authData)
	require.NoError(t, err)
	assert.Equal(t, "new-token", client.token)
}

func TestHTTPVaultClient_PerformLogin_Failure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid credentials"))
	}))
	defer server.Close()

	client := &HTTPVaultClient{
		config: Config{Address: server.URL},
	}

	ctx := context.Background()
	err := client.performLogin(ctx, "auth/userpass/login/admin", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestHTTPVaultClient_ValidateToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/token/lookup-self" {
			assert.Equal(t, "valid-token", r.Header.Get("X-Vault-Token"))
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer server.Close()

	client := &HTTPVaultClient{
		config: Config{Address: server.URL},
		token:  "valid-token",
	}

	ctx := context.Background()
	err := client.validateToken(ctx)
	require.NoError(t, err)
}

func TestHTTPVaultClient_ValidateToken_Invalid(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := &HTTPVaultClient{
		config: Config{Address: server.URL},
		token:  "invalid-token",
	}

	ctx := context.Background()
	err := client.validateToken(ctx)
	assert.Error(t, err)
}

func TestHTTPVaultClient_Authenticate_WithValidToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/token/lookup-self" {
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer server.Close()

	client := &HTTPVaultClient{
		config: Config{
			Address:    server.URL,
			AuthMethod: "token",
		},
		token: "existing-token",
	}

	ctx := context.Background()
	err := client.Authenticate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "existing-token", client.token)
}

func TestHTTPVaultClient_Authenticate_UnsupportedMethod(t *testing.T) {
	t.Parallel()

	client := &HTTPVaultClient{
		config: Config{
			AuthMethod: "unsupported",
		},
	}

	ctx := context.Background()
	err := client.Authenticate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported auth method")
}

func TestHTTPVaultClient_GetHTTPClient_WithTLSSkip(t *testing.T) {
	t.Parallel()

	client := &HTTPVaultClient{
		config: Config{TLSSkip: true},
	}

	httpClient := client.getHTTPClient()
	assert.NotNil(t, httpClient)
	assert.NotNil(t, httpClient.Transport)
}

func TestHTTPVaultClient_GetHTTPClient_WithCACert(t *testing.T) {
	t.Parallel()

	client := &HTTPVaultClient{
		config: Config{CACert: "/path/to/ca.pem"},
	}

	httpClient := client.getHTTPClient()
	assert.NotNil(t, httpClient)
}

func TestNewVaultProvider_EnvironmentOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("VAULT_ADDR", "http://env-vault:8200")
	os.Setenv("VAULT_TOKEN", "env-token")
	os.Setenv("VAULT_NAMESPACE", "env-namespace")
	os.Setenv("VAULT_SKIP_VERIFY", "true")
	defer func() {
		os.Unsetenv("VAULT_ADDR")
		os.Unsetenv("VAULT_TOKEN")
		os.Unsetenv("VAULT_NAMESPACE")
		os.Unsetenv("VAULT_SKIP_VERIFY")
	}()

	config := map[string]interface{}{
		"address": "http://config-vault:8200", // Should be overridden
	}

	p, err := NewVaultProvider("test", config)
	require.NoError(t, err)

	vaultProvider := p.(*VaultProvider)
	assert.Equal(t, "http://env-vault:8200", vaultProvider.config.Address)
	assert.Equal(t, "env-token", vaultProvider.config.Token)
	assert.Equal(t, "env-namespace", vaultProvider.config.Namespace)
	assert.True(t, vaultProvider.config.TLSSkip)
}
