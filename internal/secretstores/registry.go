package secretstores

import (
	"fmt"

	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/adapter"
	"github.com/systmms/dsops/pkg/secretstore"
)

// Registry manages secret store creation and registration
type Registry struct {
	providerRegistry *providers.Registry
	supportedTypes   map[string]bool
}

// NewRegistry creates a new secret store registry with built-in secret stores
func NewRegistry() *Registry {
	registry := &Registry{
		providerRegistry: providers.NewRegistry(),
		supportedTypes:   make(map[string]bool),
	}

	// Register secret store types (storage systems only)
	secretStoreTypes := []string{
		"literal",
		"mock", 
		"json",
		"bitwarden",
		"onepassword",
		"vault",
		"aws.secretsmanager",
		"aws.ssm",
		"aws.sts", 
		"aws.sso",
		"aws",
		"gcp.secretmanager",
		"gcp",
		"azure.keyvault",
		"azure.identity", 
		"azure",
		"doppler",
		"pass",
	}

	for _, storeType := range secretStoreTypes {
		registry.supportedTypes[storeType] = true
	}

	return registry
}

// CreateSecretStore creates a secret store instance from configuration
func (r *Registry) CreateSecretStore(name string, cfg config.SecretStoreConfig) (secretstore.SecretStore, error) {
	if !r.IsSupported(cfg.Type) {
		return nil, fmt.Errorf("unknown secret store type: %s", cfg.Type)
	}

	// Convert SecretStoreConfig to ProviderConfig for delegation
	providerConfig := config.ProviderConfig(cfg)

	// Delegate to provider registry
	provider, err := r.providerRegistry.CreateProvider(name, providerConfig)
	if err != nil {
		return nil, err
	}

	// Wrap with adapter to implement SecretStore interface
	return adapter.NewProviderToSecretStoreAdapter(provider), nil
}

// GetSupportedTypes returns a list of supported secret store types
func (r *Registry) GetSupportedTypes() []string {
	types := make([]string, 0, len(r.supportedTypes))
	for storeType := range r.supportedTypes {
		types = append(types, storeType)
	}
	return types
}

// IsSupported checks if a secret store type is supported
func (r *Registry) IsSupported(storeType string) bool {
	return r.supportedTypes[storeType]
}

