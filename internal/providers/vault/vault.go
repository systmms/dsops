package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
	dserrors "github.com/systmms/dsops/internal/errors"
)

const (
	DefaultVaultAddr = "https://vault.example.com:8200"
	DefaultTimeout   = 30 * time.Second
)

// VaultProvider implements the Provider interface for HashiCorp Vault
type VaultProvider struct {
	name     string
	config   Config
	logger   *logging.Logger
	client   VaultClient
}

// Config holds Vault-specific configuration
type Config struct {
	Address    string `yaml:"address"`     // Vault server address
	Token      string `yaml:"token"`       // Vault token (discouraged, use env var)
	AuthMethod string `yaml:"auth_method"` // Authentication method: token, userpass, ldap, aws, k8s
	Namespace  string `yaml:"namespace"`   // Vault namespace (Vault Enterprise)
	
	// Auth method specific configs
	UserpassUsername string `yaml:"userpass_username"` // For userpass auth
	UserpassPassword string `yaml:"userpass_password"` // For userpass auth (discouraged)
	LDAPUsername     string `yaml:"ldap_username"`     // For LDAP auth
	LDAPPassword     string `yaml:"ldap_password"`     // For LDAP auth (discouraged)
	AWSRole          string `yaml:"aws_role"`          // For AWS auth
	K8SRole          string `yaml:"k8s_role"`          // For Kubernetes auth
	
	// Optional settings
	CACert     string `yaml:"ca_cert"`     // Path to CA certificate
	ClientCert string `yaml:"client_cert"` // Path to client certificate
	ClientKey  string `yaml:"client_key"`  // Path to client key
	TLSSkip    bool   `yaml:"tls_skip"`    // Skip TLS verification (not recommended)
}

// VaultClient interface for testability
type VaultClient interface {
	Read(ctx context.Context, path string) (*VaultSecret, error)
	Authenticate(ctx context.Context) error
	Close() error
}

// VaultSecret represents a secret from Vault
type VaultSecret struct {
	Data     map[string]interface{} `json:"data"`
	Metadata map[string]interface{} `json:"metadata"`
}

// HTTPVaultClient implements VaultClient using HTTP API
type HTTPVaultClient struct {
	config Config
	token  string
	logger *logging.Logger
}

// NewVaultProvider creates a new Vault provider
func NewVaultProvider(name string, configMap map[string]interface{}) (provider.Provider, error) {
	// Create a default logger if none provided
	logger := logging.New(false, false)
	var config Config
	
	// Set defaults
	config.Address = DefaultVaultAddr
	config.AuthMethod = "token"
	
	// Parse configuration
	if addr, ok := configMap["address"].(string); ok {
		config.Address = addr
	}
	if token, ok := configMap["token"].(string); ok {
		config.Token = token
	}
	if authMethod, ok := configMap["auth_method"].(string); ok {
		config.AuthMethod = authMethod
	}
	if namespace, ok := configMap["namespace"].(string); ok {
		config.Namespace = namespace
	}
	if username, ok := configMap["userpass_username"].(string); ok {
		config.UserpassUsername = username
	}
	if password, ok := configMap["userpass_password"].(string); ok {
		config.UserpassPassword = password
	}
	if username, ok := configMap["ldap_username"].(string); ok {
		config.LDAPUsername = username
	}
	if password, ok := configMap["ldap_password"].(string); ok {
		config.LDAPPassword = password
	}
	if role, ok := configMap["aws_role"].(string); ok {
		config.AWSRole = role
	}
	if role, ok := configMap["k8s_role"].(string); ok {
		config.K8SRole = role
	}
	if caCert, ok := configMap["ca_cert"].(string); ok {
		config.CACert = caCert
	}
	if clientCert, ok := configMap["client_cert"].(string); ok {
		config.ClientCert = clientCert
	}
	if clientKey, ok := configMap["client_key"].(string); ok {
		config.ClientKey = clientKey
	}
	if tlsSkip, ok := configMap["tls_skip"].(bool); ok {
		config.TLSSkip = tlsSkip
	}

	// Override with environment variables
	if addr := os.Getenv("VAULT_ADDR"); addr != "" {
		config.Address = addr
	}
	if token := os.Getenv("VAULT_TOKEN"); token != "" {
		config.Token = token
	}
	if namespace := os.Getenv("VAULT_NAMESPACE"); namespace != "" {
		config.Namespace = namespace
	}
	if caCert := os.Getenv("VAULT_CACERT"); caCert != "" {
		config.CACert = caCert
	}
	if clientCert := os.Getenv("VAULT_CLIENT_CERT"); clientCert != "" {
		config.ClientCert = clientCert
	}
	if clientKey := os.Getenv("VAULT_CLIENT_KEY"); clientKey != "" {
		config.ClientKey = clientKey
	}
	if tlsSkip := os.Getenv("VAULT_SKIP_VERIFY"); tlsSkip == "1" || strings.ToLower(tlsSkip) == "true" {
		config.TLSSkip = true
	}

	// Create HTTP client
	client := &HTTPVaultClient{
		config: config,
		logger: logger,
	}

	return &VaultProvider{
		name:   name,
		config: config,
		logger: logger,
		client: client,
	}, nil
}

// Name returns the provider name
func (v *VaultProvider) Name() string {
	return v.name
}

// Resolve fetches a secret from Vault
func (v *VaultProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	// Authenticate if needed
	if err := v.client.Authenticate(ctx); err != nil {
		return provider.SecretValue{}, fmt.Errorf("vault authentication failed: %w", err)
	}

	// Parse the reference
	path, field, err := v.parseReference(ref.Key)
	if err != nil {
		return provider.SecretValue{}, err
	}

	v.logger.Debug("Fetching secret from Vault path: %s, field: %s", logging.Secret(path), logging.Secret(field))

	// Read secret from Vault
	secret, err := v.client.Read(ctx, path)
	if err != nil {
		return provider.SecretValue{}, dserrors.UserError{
			Message:    "Failed to read secret from Vault",
			Details:    err.Error(),
			Suggestion: v.getVaultErrorSuggestion(err),
		}
	}

	if secret == nil || secret.Data == nil {
		return provider.SecretValue{}, dserrors.UserError{
			Message:    fmt.Sprintf("Secret not found at path: %s", path),
			Suggestion: "Check that the secret exists and you have read permissions",
		}
	}

	// Extract the requested field
	var value string
	if field == "" {
		// Return entire secret as JSON if no field specified
		jsonData, err := json.Marshal(secret.Data)
		if err != nil {
			return provider.SecretValue{}, fmt.Errorf("failed to marshal secret data: %w", err)
		}
		value = string(jsonData)
	} else {
		// Extract specific field
		fieldValue, exists := secret.Data[field]
		if !exists {
			// List available fields
			var availableFields []string
			for k := range secret.Data {
				availableFields = append(availableFields, k)
			}
			return provider.SecretValue{}, dserrors.UserError{
				Message:    fmt.Sprintf("Field '%s' not found in secret", field),
				Suggestion: fmt.Sprintf("Available fields: %s", strings.Join(availableFields, ", ")),
			}
		}

		// Convert to string
		switch v := fieldValue.(type) {
		case string:
			value = v
		case []byte:
			value = string(v)
		case int, int32, int64:
			value = fmt.Sprintf("%d", v)
		case float32, float64:
			value = fmt.Sprintf("%g", v)
		case bool:
			value = strconv.FormatBool(v)
		default:
			// Convert to JSON for complex types
			jsonData, err := json.Marshal(v)
			if err != nil {
				return provider.SecretValue{}, fmt.Errorf("failed to convert field value to string: %w", err)
			}
			value = string(jsonData)
		}
	}

	return provider.SecretValue{
		Value: value,
		Metadata: map[string]string{
			"source": fmt.Sprintf("vault:%s", path),
			"path":   path,
		},
	}, nil
}

// Describe returns metadata about a secret without fetching its value
func (v *VaultProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	path, field, err := v.parseReference(ref.Key)
	if err != nil {
		return provider.Metadata{}, err
	}

	return provider.Metadata{
		Type:   "vault-secret",
		Tags: map[string]string{
			"source":      fmt.Sprintf("vault:%s", path),
			"path":        path,
			"field":       field,
			"address":     v.config.Address,
			"namespace":   v.config.Namespace,
			"auth_method": v.config.AuthMethod,
		},
	}, nil
}

// Capabilities returns the provider's capabilities
func (v *VaultProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: true,  // Vault supports versioning in KV v2
		SupportsMetadata:   true,
		SupportsWatching:   false, // Future feature
		SupportsBinary:     true,  // Vault can store binary data
		RequiresAuth:       true,
		AuthMethods:        []string{"token", "userpass", "ldap", "aws", "k8s"},
	}
}

// Validate checks if the provider is properly configured and accessible
func (v *VaultProvider) Validate(ctx context.Context) error {
	// Check required configuration
	if v.config.Address == "" {
		return dserrors.ConfigError{
			Field:      "address",
			Message:    "Vault address is required",
			Suggestion: "Set 'address' in provider config or VAULT_ADDR environment variable",
		}
	}

	// Validate authentication configuration
	switch v.config.AuthMethod {
	case "token":
		if v.config.Token == "" {
			if os.Getenv("VAULT_TOKEN") == "" {
				return dserrors.ConfigError{
					Field:      "token",
					Message:    "Vault token is required for token auth",
					Suggestion: "Set 'token' in provider config or VAULT_TOKEN environment variable",
				}
			}
		}
	case "userpass":
		if v.config.UserpassUsername == "" {
			return dserrors.ConfigError{
				Field:      "userpass_username",
				Message:    "Username is required for userpass auth",
				Suggestion: "Set 'userpass_username' in provider config",
			}
		}
	case "ldap":
		if v.config.LDAPUsername == "" {
			return dserrors.ConfigError{
				Field:      "ldap_username", 
				Message:    "Username is required for LDAP auth",
				Suggestion: "Set 'ldap_username' in provider config",
			}
		}
	case "aws":
		if v.config.AWSRole == "" {
			return dserrors.ConfigError{
				Field:      "aws_role",
				Message:    "AWS role is required for AWS auth",
				Suggestion: "Set 'aws_role' in provider config",
			}
		}
	case "k8s", "kubernetes":
		if v.config.K8SRole == "" {
			return dserrors.ConfigError{
				Field:      "k8s_role",
				Message:    "Kubernetes role is required for k8s auth",
				Suggestion: "Set 'k8s_role' in provider config",
			}
		}
	default:
		return dserrors.ConfigError{
			Field:      "auth_method",
			Value:      v.config.AuthMethod,
			Message:    "unsupported authentication method",
			Suggestion: "Supported methods: token, userpass, ldap, aws, k8s",
		}
	}

	// Test connectivity
	if err := v.client.Authenticate(ctx); err != nil {
		return dserrors.UserError{
			Message:    "Failed to authenticate with Vault",
			Details:    err.Error(),
			Suggestion: v.getVaultErrorSuggestion(err),
		}
	}

	return nil
}

// parseReference parses a Vault reference into path and field
// Supports formats:
// - "secret/data/myapp" (returns entire secret as JSON)
// - "secret/data/myapp#password" (returns specific field)
// - "secret/data/myapp@v2#password" (returns specific version and field)
func (v *VaultProvider) parseReference(key string) (string, string, error) {
	if key == "" {
		return "", "", dserrors.UserError{
			Message:    "Empty vault key",
			Suggestion: "Provide a vault path like 'secret/data/myapp' or 'secret/data/myapp#field'",
		}
	}

	// Split on # to separate path from field
	parts := strings.Split(key, "#")
	path := parts[0]
	field := ""
	
	if len(parts) > 2 {
		return "", "", dserrors.UserError{
			Message:    "Invalid vault key format",
			Suggestion: "Use format: 'path' or 'path#field'",
		}
	}
	
	if len(parts) == 2 {
		field = parts[1]
	}

	// TODO: Handle versioning (@v2 syntax) in future version
	
	if path == "" {
		return "", "", dserrors.UserError{
			Message:    "Empty vault path",
			Suggestion: "Provide a vault path like 'secret/data/myapp'",
		}
	}

	return path, field, nil
}

// getVaultErrorSuggestion provides helpful suggestions based on Vault errors
func (v *VaultProvider) getVaultErrorSuggestion(err error) string {
	errStr := strings.ToLower(err.Error())
	
	switch {
	case strings.Contains(errStr, "connection refused"):
		return "Check that Vault server is running and accessible at " + v.config.Address
	case strings.Contains(errStr, "permission denied"):
		return "Check your Vault token permissions for this path"
	case strings.Contains(errStr, "invalid token"):
		return "Your Vault token may be expired or invalid. Try 'vault auth' to refresh"
	case strings.Contains(errStr, "namespace"):
		return "Check your Vault namespace configuration"
	case strings.Contains(errStr, "tls"):
		return "Check TLS configuration or try setting tls_skip: true for testing"
	case strings.Contains(errStr, "auth"):
		return "Authentication failed. Check your credentials and auth method configuration"
	default:
		return "Check your Vault configuration and connectivity. Run 'dsops doctor' for diagnostics"
	}
}