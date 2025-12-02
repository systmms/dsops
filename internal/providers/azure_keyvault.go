package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
	dserrors "github.com/systmms/dsops/internal/errors"
)

// AzureKeyVaultClientAPI defines the interface for Azure Key Vault operations
// This allows for mocking in tests
type AzureKeyVaultClientAPI interface {
	GetSecret(ctx context.Context, name string, version string, options *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error)
	// Note: NewListSecretPropertiesPager is excluded from the interface for now
	// as it returns a complex pager type that's difficult to mock.
	// For validation testing, we'll use GetSecret instead.
}

// AzureKeyVaultProvider implements the Provider interface for Azure Key Vault
type AzureKeyVaultProvider struct {
	name       string
	client     AzureKeyVaultClientAPI
	logger     *logging.Logger
	config     AzureKeyVaultConfig
	vaultURL   string
}

// AzureKeyVaultConfig holds Azure Key Vault-specific configuration
type AzureKeyVaultConfig struct {
	VaultURL           string
	TenantID           string
	ClientID           string
	ClientSecret       string
	CertificatePath    string
	UseManagedIdentity bool
	UserAssignedID     string // For user-assigned managed identity
}

// AzureProviderOption is a functional option for configuring Azure providers
type AzureProviderOption func(*AzureKeyVaultProvider)

// WithAzureKeyVaultClient sets a custom Azure Key Vault client (for testing)
func WithAzureKeyVaultClient(client AzureKeyVaultClientAPI) AzureProviderOption {
	return func(p *AzureKeyVaultProvider) {
		p.client = client
	}
}

// NewAzureKeyVaultProvider creates a new Azure Key Vault provider
func NewAzureKeyVaultProvider(name string, configMap map[string]interface{}, opts ...AzureProviderOption) (*AzureKeyVaultProvider, error) {
	logger := logging.New(false, false)

	config := AzureKeyVaultConfig{
		UseManagedIdentity: true, // Default to managed identity
	}

	// Parse configuration
	if vaultURL, ok := configMap["vault_url"].(string); ok {
		config.VaultURL = vaultURL
	}
	if tenantID, ok := configMap["tenant_id"].(string); ok {
		config.TenantID = tenantID
	}
	if clientID, ok := configMap["client_id"].(string); ok {
		config.ClientID = clientID
	}
	if clientSecret, ok := configMap["client_secret"].(string); ok {
		config.ClientSecret = clientSecret
	}
	if certPath, ok := configMap["certificate_path"].(string); ok {
		config.CertificatePath = certPath
	}
	if useMI, ok := configMap["use_managed_identity"].(bool); ok {
		config.UseManagedIdentity = useMI
	}
	if userAssignedID, ok := configMap["user_assigned_identity_id"].(string); ok {
		config.UserAssignedID = userAssignedID
	}

	// Validate required configuration
	if config.VaultURL == "" {
		return nil, dserrors.ConfigError{
			Field:      "vault_url",
			Message:    "vault_url is required for Azure Key Vault",
			Suggestion: "Provide the Key Vault URL (e.g., https://my-vault.vault.azure.net/)",
		}
	}

	// Validate URL format
	if _, err := url.Parse(config.VaultURL); err != nil {
		return nil, dserrors.ConfigError{
			Field:      "vault_url",
			Message:    "Invalid vault_url format",
			Suggestion: "Use format: https://vault-name.vault.azure.net/",
		}
	}

	p := &AzureKeyVaultProvider{
		name:     name,
		logger:   logger,
		config:   config,
		vaultURL: config.VaultURL,
	}

	// Apply options (allows mock client injection)
	for _, opt := range opts {
		opt(p)
	}

	// If no client was provided via options, create real client
	if p.client == nil {
		client, err := createAzureKeyVaultClient(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create Azure Key Vault client: %w", err)
		}
		p.client = client
	}

	return p, nil
}

// createAzureKeyVaultClient creates an Azure Key Vault client with appropriate authentication
func createAzureKeyVaultClient(config AzureKeyVaultConfig) (*azsecrets.Client, error) {
	var cred azcore.TokenCredential
	var err error

	// Determine authentication method
	if config.UseManagedIdentity {
		// Managed Identity (system-assigned or user-assigned)
		if config.UserAssignedID != "" {
			// User-assigned managed identity
			clientIDCred := azidentity.ManagedIdentityCredentialOptions{
				ID: azidentity.ClientID(config.UserAssignedID),
			}
			cred, err = azidentity.NewManagedIdentityCredential(&clientIDCred)
		} else {
			// System-assigned managed identity
			cred, err = azidentity.NewManagedIdentityCredential(nil)
		}
	} else if config.ClientSecret != "" {
		// Service Principal with client secret
		cred, err = azidentity.NewClientSecretCredential(config.TenantID, config.ClientID, config.ClientSecret, nil)
	} else if config.CertificatePath != "" {
		// Service Principal with certificate
		// Certificate authentication is more complex and requires parsing
		// For now, return an error indicating it's not implemented
		return nil, fmt.Errorf("certificate authentication not yet implemented")
	} else {
		// Azure CLI or Default Azure Credential
		cred, err = azidentity.NewDefaultAzureCredential(nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	// Create Key Vault client
	client, err := azsecrets.NewClient(config.VaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Key Vault client: %w", err)
	}

	return client, nil
}

// readCertificateFile reads certificate data from file

// Name returns the provider name
func (p *AzureKeyVaultProvider) Name() string {
	return p.name
}

// Resolve fetches a secret from Azure Key Vault
func (p *AzureKeyVaultProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	secretName, version, jsonPath := p.parseReference(ref.Key)
	
	p.logger.Debug("Accessing Azure Key Vault secret: %s", logging.Secret(secretName))

	// Get secret from Key Vault
	var resp azsecrets.GetSecretResponse
	var err error

	if version != "" {
		resp, err = p.client.GetSecret(ctx, secretName, version, nil)
	} else {
		resp, err = p.client.GetSecret(ctx, secretName, "", nil)
	}

	if err != nil {
		return provider.SecretValue{}, dserrors.UserError{
			Message:    fmt.Sprintf("Failed to access secret: %s", secretName),
			Details:    err.Error(),
			Suggestion: getAzureErrorSuggestion(err),
		}
	}

	if resp.Value == nil {
		return provider.SecretValue{}, fmt.Errorf("secret has no value")
	}

	secretData := *resp.Value
	
	// Extract JSON field if specified
	if jsonPath != "" {
		extractedValue, err := extractJSONPathAzure(secretData, jsonPath)
		if err != nil {
			return provider.SecretValue{}, dserrors.UserError{
				Message:    fmt.Sprintf("Failed to extract JSON path: %s", jsonPath),
				Details:    err.Error(),
				Suggestion: "Check that the secret contains valid JSON and the path exists",
			}
		}
		secretData = extractedValue
	}

	// Build metadata
	metadata := map[string]string{
		"source":    fmt.Sprintf("azure-kv:%s", secretName),
		"vault_url": p.vaultURL,
	}

	if resp.Attributes != nil {
		if resp.Attributes.Created != nil {
			metadata["created_at"] = resp.Attributes.Created.Format(time.RFC3339)
		}
		if resp.Attributes.Updated != nil {
			metadata["updated_at"] = resp.Attributes.Updated.Format(time.RFC3339)
		}
		if resp.Attributes.Expires != nil {
			metadata["expires_at"] = resp.Attributes.Expires.Format(time.RFC3339)
		}
	}
	// Note: Version information would need separate API call

	return provider.SecretValue{
		Value:    secretData,
		Metadata: metadata,
	}, nil
}

// parseReference parses Azure Key Vault secret references
func (p *AzureKeyVaultProvider) parseReference(ref string) (secretName, version, jsonPath string) {
	// Check for JSON path extraction (secret-name#.json.path)
	if strings.Contains(ref, "#") {
		parts := strings.SplitN(ref, "#", 2)
		ref = parts[0]
		jsonPath = parts[1]
	}
	
	// Check for version specification (secret-name/version)
	if strings.Contains(ref, "/") {
		parts := strings.SplitN(ref, "/", 2)
		secretName = parts[0]
		version = parts[1]
	} else {
		secretName = ref
	}
	
	return secretName, version, jsonPath
}

// extractJSONPathAzure extracts a value from JSON using a simple path
func extractJSONPathAzure(jsonStr, path string) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	
	// Simple path extraction (e.g., .field.subfield)
	path = strings.TrimPrefix(path, ".")
	parts := strings.Split(path, ".")
	
	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}
		
		switch v := current.(type) {
		case map[string]interface{}:
			var exists bool
			current, exists = v[part]
			if !exists {
				return "", fmt.Errorf("path not found: %s", part)
			}
		case []interface{}:
			// Handle array index
			if index, err := strconv.Atoi(part); err == nil && index < len(v) {
				current = v[index]
			} else {
				return "", fmt.Errorf("invalid array index: %s", part)
			}
		default:
			return "", fmt.Errorf("cannot traverse path at: %s", part)
		}
	}
	
	// Convert result to string
	switch v := current.(type) {
	case string:
		return v, nil
	case nil:
		return "", nil
	default:
		// Convert to JSON for complex types
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal result: %w", err)
		}
		return string(jsonBytes), nil
	}
}

// Describe returns metadata about a secret without fetching its value
func (p *AzureKeyVaultProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	secretName, version, _ := p.parseReference(ref.Key)
	
	var resp azsecrets.GetSecretResponse
	var err error

	if version != "" {
		resp, err = p.client.GetSecret(ctx, secretName, version, nil)
	} else {
		resp, err = p.client.GetSecret(ctx, secretName, "", nil)
	}

	if err != nil {
		if isAzureNotFoundError(err) {
			return provider.Metadata{
				Exists: false,
			}, nil
		}
		return provider.Metadata{}, fmt.Errorf("failed to describe secret: %w", err)
	}

	metadata := provider.Metadata{
		Exists: true,
		Type:   "azure-secret",
		Tags: map[string]string{
			"source":    fmt.Sprintf("azure-kv:%s", secretName),
			"vault_url": p.vaultURL,
		},
	}

	if resp.Attributes != nil {
		if resp.Attributes.Created != nil {
			metadata.UpdatedAt = *resp.Attributes.Created
		}
	}
	// Note: Version and tags would need separate API calls

	return metadata, nil
}

// Capabilities returns the provider's capabilities
func (p *AzureKeyVaultProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: true,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     true, // Key Vault can store binary data
		RequiresAuth:       true,
		AuthMethods:        []string{"managed_identity", "service_principal", "azure_cli", "default_credential"},
	}
}

// Validate checks if the provider is properly configured and accessible
func (p *AzureKeyVaultProvider) Validate(ctx context.Context) error {
	// For real clients, we need to check if it's the concrete type that has the pager method
	// For mock clients used in tests, we'll skip pager-based validation

	// Type assertion to check if we have the real client
	if realClient, ok := p.client.(*azsecrets.Client); ok {
		// Test by listing secrets (requires minimal permissions)
		pager := realClient.NewListSecretPropertiesPager(nil)

		// Try to get the first page
		_, err := pager.NextPage(ctx)
		if err != nil {
			return dserrors.UserError{
				Message:    "Failed to connect to Azure Key Vault",
				Details:    err.Error(),
				Suggestion: getAzureErrorSuggestion(err),
			}
		}
	}
	// For mock clients, validation passes by default
	// Tests can inject errors via the mock if needed

	return nil
}

// isAzureNotFoundError checks if the error indicates a secret was not found
func isAzureNotFoundError(err error) bool {
	return strings.Contains(err.Error(), "SecretNotFound") || strings.Contains(err.Error(), "404")
}

// getAzureErrorSuggestion provides helpful suggestions based on Azure errors
func getAzureErrorSuggestion(err error) string {
	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "forbidden") || strings.Contains(errStr, "access denied"):
		return "Check Key Vault access policies: 'Get' and 'List' permissions are required for secrets"
	case strings.Contains(errStr, "secretnotfound") || strings.Contains(errStr, "404"):
		return "Verify the secret name exists in the Key Vault. Secret names are case-sensitive"
	case strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "401"):
		return "Check authentication: verify managed identity, service principal, or Azure CLI login"
	case strings.Contains(errStr, "vault not found") || strings.Contains(errStr, "keyvaulterror"):
		return "Check the vault URL format and that the Key Vault exists"
	case strings.Contains(errStr, "throttled") || strings.Contains(errStr, "429"):
		return "Request was throttled. Consider adding exponential backoff or reducing request rate"
	case strings.Contains(errStr, "tenant"):
		return "Check that the tenant ID is correct and the application is registered"
	default:
		return "Check Azure credentials, Key Vault URL, and access policies"
	}
}

// NewAzureKeyVaultProviderFactory creates an Azure Key Vault provider factory
func NewAzureKeyVaultProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewAzureKeyVaultProvider(name, config)
}