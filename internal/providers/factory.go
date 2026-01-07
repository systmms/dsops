package providers

import (
	"time"

	"github.com/systmms/dsops/pkg/provider"
)

// KeychainConfig holds configuration for the keychain provider
type KeychainConfig struct {
	// ServicePrefix is prepended to service names in references
	// Example: "com.mycompany" + "/myapp" → service="com.mycompany.myapp"
	ServicePrefix string `mapstructure:"service_prefix"`

	// AccessGroup (macOS only) specifies the keychain access group
	// for shared keychain items between applications
	AccessGroup string `mapstructure:"access_group"`
}

// InfisicalConfig holds configuration for the Infisical provider
type InfisicalConfig struct {
	// Host is the Infisical instance URL
	// Defaults to "https://app.infisical.com"
	Host string `mapstructure:"host"`

	// ProjectID is the Infisical project identifier (required)
	ProjectID string `mapstructure:"project_id"`

	// Environment is the environment slug (required)
	// Examples: "dev", "staging", "prod"
	Environment string `mapstructure:"environment"`

	// Auth contains authentication configuration
	Auth InfisicalAuth `mapstructure:"auth"`

	// Timeout for API requests (default: 30s)
	Timeout time.Duration `mapstructure:"timeout"`

	// CACert is path to custom CA certificate for self-hosted instances
	CACert string `mapstructure:"ca_cert"`

	// InsecureSkipVerify disables TLS verification (use with caution)
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}

// InfisicalAuth defines authentication method for Infisical
type InfisicalAuth struct {
	// Method is the authentication method
	// Values: "machine_identity", "service_token", "api_key"
	Method string `mapstructure:"method"`

	// ClientID for machine identity auth
	ClientID string `mapstructure:"client_id"`

	// ClientSecret for machine identity auth
	ClientSecret string `mapstructure:"client_secret"`

	// ServiceToken for service token auth (legacy)
	ServiceToken string `mapstructure:"service_token"`

	// APIKey for API key auth (development)
	APIKey string `mapstructure:"api_key"`
}

// AkeylessConfig holds configuration for the Akeyless provider
type AkeylessConfig struct {
	// AccessID is the Akeyless access ID (required)
	AccessID string `mapstructure:"access_id"`

	// GatewayURL is the custom gateway URL for enterprise deployments
	// Defaults to "https://api.akeyless.io"
	GatewayURL string `mapstructure:"gateway_url"`

	// Auth contains authentication configuration
	Auth AkeylessAuth `mapstructure:"auth"`

	// Timeout for API requests (default: 30s)
	Timeout time.Duration `mapstructure:"timeout"`
}

// AkeylessAuth defines authentication method for Akeyless
type AkeylessAuth struct {
	// Method is the authentication method
	// Values: "api_key", "aws_iam", "azure_ad", "gcp", "oidc", "saml"
	Method string `mapstructure:"method"`

	// AccessKey for API key auth
	AccessKey string `mapstructure:"access_key"`

	// AzureADObjectID for Azure AD auth
	AzureADObjectID string `mapstructure:"azure_ad_object_id"`

	// GCPAudience for GCP auth
	GCPAudience string `mapstructure:"gcp_audience"`
}

// Default values for provider configurations
const (
	DefaultInfisicalHost   = "https://app.infisical.com"
	DefaultAkeylessGateway = "https://api.akeyless.io"
	DefaultTimeout         = 30 * time.Second
)

// NewKeychainProviderFunc is the factory function signature for keychain
type NewKeychainProviderFunc func(name string, config map[string]interface{}) (provider.Provider, error)

// NewInfisicalProviderFunc is the factory function signature for Infisical
type NewInfisicalProviderFunc func(name string, config map[string]interface{}) (provider.Provider, error)

// NewAkeylessProviderFunc is the factory function signature for Akeyless
type NewAkeylessProviderFunc func(name string, config map[string]interface{}) (provider.Provider, error)
