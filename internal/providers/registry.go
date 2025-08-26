package providers

import (
	"fmt"

	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/providers/vault"
	"github.com/systmms/dsops/pkg/provider"
)

// Registry manages provider creation and registration
type Registry struct {
	factories map[string]ProviderFactory
}

// ProviderFactory creates a provider instance from configuration
type ProviderFactory func(name string, config map[string]interface{}) (provider.Provider, error)

// NewRegistry creates a new provider registry with built-in providers
func NewRegistry() *Registry {
	registry := &Registry{
		factories: make(map[string]ProviderFactory),
	}

	// Register built-in providers
	registry.RegisterFactory("literal", NewLiteralProviderFactory)
	registry.RegisterFactory("mock", NewMockProviderFactory)
	registry.RegisterFactory("json", NewJSONProviderFactory)
	registry.RegisterFactory("bitwarden", NewBitwardenProviderFactory)
	registry.RegisterFactory("aws.secretsmanager", NewAWSSecretsManagerProviderFactory)
	registry.RegisterFactory("aws.ssm", NewAWSSSMProviderFactory)
	registry.RegisterFactory("aws.sts", NewAWSSTSProviderFactory)
	registry.RegisterFactory("aws.sso", NewAWSSSOProviderFactory)
	registry.RegisterFactory("aws", NewAWSUnifiedProviderFactory)
	registry.RegisterFactory("gcp.secretmanager", NewGCPSecretManagerProviderFactory)
	registry.RegisterFactory("gcp", NewGCPUnifiedProviderFactory)
	registry.RegisterFactory("azure.keyvault", NewAzureKeyVaultProviderFactory)
	registry.RegisterFactory("azure.identity", NewAzureIdentityProviderFactory)
	registry.RegisterFactory("azure", NewAzureUnifiedProviderFactory)
	registry.RegisterFactory("onepassword", NewOnePasswordProviderFactory)
	registry.RegisterFactory("vault", NewVaultProviderFactory)
	registry.RegisterFactory("doppler", NewDopplerProviderFactory)
	registry.RegisterFactory("pass", NewPassProviderFactory)

	return registry
}

// RegisterFactory registers a provider factory for a given type
func (r *Registry) RegisterFactory(providerType string, factory ProviderFactory) {
	r.factories[providerType] = factory
}

// CreateProvider creates a provider instance from configuration
func (r *Registry) CreateProvider(name string, cfg config.ProviderConfig) (provider.Provider, error) {
	factory, exists := r.factories[cfg.Type]
	if !exists {
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}

	return factory(name, cfg.Config)
}

// GetSupportedTypes returns a list of supported provider types
func (r *Registry) GetSupportedTypes() []string {
	types := make([]string, 0, len(r.factories))
	for providerType := range r.factories {
		types = append(types, providerType)
	}
	return types
}

// IsSupported checks if a provider type is supported
func (r *Registry) IsSupported(providerType string) bool {
	_, exists := r.factories[providerType]
	return exists
}

// Factory functions for built-in providers

// NewLiteralProviderFactory creates a literal provider factory
func NewLiteralProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	values := make(map[string]string)
	if configMap, ok := config["values"].(map[string]interface{}); ok {
		for k, v := range configMap {
			if str, ok := v.(string); ok {
				values[k] = str
			}
		}
	}
	return NewLiteralProvider(name, values), nil
}

// NewMockProviderFactory creates a mock provider factory
func NewMockProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	mockProvider := NewMockProvider(name)
	
	// Add default test values
	mockProvider.SetValue("test-secret", "mock-value")
	mockProvider.SetValue("api-key", "mock-api-key-123")
	
	// Add any configured values
	if values, ok := config["values"].(map[string]interface{}); ok {
		for k, v := range values {
			if str, ok := v.(string); ok {
				mockProvider.SetValue(k, str)
			}
		}
	}

	return mockProvider, nil
}

// NewJSONProviderFactory creates a JSON provider factory
func NewJSONProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewJSONProvider(name), nil
}

// NewBitwardenProviderFactory creates a Bitwarden provider factory
func NewBitwardenProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewBitwardenProvider(name, config), nil
}

// NewAWSSecretsManagerProviderFactory creates an AWS Secrets Manager provider factory
func NewAWSSecretsManagerProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewAWSSecretsManagerProvider(name, config)
}

// NewOnePasswordProviderFactory creates a 1Password provider factory
func NewOnePasswordProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewOnePasswordProvider(config)
}

// NewVaultProviderFactory creates a HashiCorp Vault provider factory
func NewVaultProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return vault.NewVaultProvider(name, config)
}

// NewDopplerProviderFactory creates a Doppler provider factory
func NewDopplerProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	var dopplerConfig DopplerConfig

	if token, ok := config["token"].(string); ok {
		dopplerConfig.Token = token
	}

	if project, ok := config["project"].(string); ok {
		dopplerConfig.Project = project
	}

	if configName, ok := config["config"].(string); ok {
		dopplerConfig.Config = configName
	}

	// Validate required fields
	if dopplerConfig.Token == "" {
		return nil, fmt.Errorf("missing required 'token' field for Doppler provider")
	}

	if dopplerConfig.Project == "" {
		return nil, fmt.Errorf("missing required 'project' field for Doppler provider")
	}

	if dopplerConfig.Config == "" {
		return nil, fmt.Errorf("missing required 'config' field for Doppler provider")
	}

	return NewDopplerProvider(dopplerConfig), nil
}

// NewPassProviderFactory creates a pass provider factory
func NewPassProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	var passConfig PassConfig

	if passwordStore, ok := config["password_store"].(string); ok {
		passConfig.PasswordStore = passwordStore
	}

	if gpgKey, ok := config["gpg_key"].(string); ok {
		passConfig.GpgKey = gpgKey
	}

	return NewPassProvider(passConfig), nil
}