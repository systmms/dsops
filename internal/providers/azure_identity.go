package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
	dserrors "github.com/systmms/dsops/internal/errors"
)

// AzureIdentityProvider implements the Provider interface for Azure Managed Identity and Service Principal authentication
type AzureIdentityProvider struct {
	name       string
	credential azcore.TokenCredential
	logger     *logging.Logger
	config     AzureIdentityConfig
}

// AzureIdentityConfig holds Azure Identity-specific configuration
type AzureIdentityConfig struct {
	TenantID           string
	ClientID           string
	ClientSecret       string
	CertificatePath    string
	UseManagedIdentity bool
	UserAssignedID     string
	Scope              string // Default scope for token requests
}

// NewAzureIdentityProvider creates a new Azure Identity provider
func NewAzureIdentityProvider(name string, configMap map[string]interface{}) (*AzureIdentityProvider, error) {
	logger := logging.New(false, false)
	
	config := AzureIdentityConfig{
		UseManagedIdentity: true,                                    // Default to managed identity
		Scope:              "https://management.azure.com/.default", // Default Azure Management scope
	}

	// Parse configuration
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
	if scope, ok := configMap["scope"].(string); ok {
		config.Scope = scope
	}

	// Create Azure credential
	credential, err := createAzureCredential(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	return &AzureIdentityProvider{
		name:       name,
		credential: credential,
		logger:     logger,
		config:     config,
	}, nil
}

// createAzureCredential creates an Azure credential based on configuration
func createAzureCredential(config AzureIdentityConfig) (azcore.TokenCredential, error) {
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
		if config.TenantID == "" || config.ClientID == "" {
			return nil, fmt.Errorf("tenant_id and client_id are required for service principal authentication")
		}
		cred, err = azidentity.NewClientSecretCredential(config.TenantID, config.ClientID, config.ClientSecret, nil)
	} else if config.CertificatePath != "" {
		// Service Principal with certificate
		if config.TenantID == "" || config.ClientID == "" {
			return nil, fmt.Errorf("tenant_id and client_id are required for certificate authentication")
		}
		// For now, certificate auth is not fully implemented
		return nil, fmt.Errorf("certificate authentication not yet implemented")
	} else {
		// Azure CLI or Default Azure Credential
		cred, err = azidentity.NewDefaultAzureCredential(nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	return cred, nil
}

// Name returns the provider name
func (p *AzureIdentityProvider) Name() string {
	return p.name
}

// Resolve fetches an access token or credential information from Azure Identity
func (p *AzureIdentityProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	scope, requestedField := p.parseReference(ref.Key)
	
	// Use configured scope if not specified in reference
	if scope == "" {
		scope = p.config.Scope
	}

	p.logger.Debug("Requesting Azure access token for scope: %s", logging.Secret(scope))

	// Get access token
	tokenReq := policy.TokenRequestOptions{
		Scopes: []string{scope},
	}

	token, err := p.credential.GetToken(ctx, tokenReq)
	if err != nil {
		return provider.SecretValue{}, dserrors.UserError{
			Message:    "Failed to get Azure access token",
			Details:    err.Error(),
			Suggestion: getAzureIdentityErrorSuggestion(err),
		}
	}

	// Extract requested field
	value, err := p.getTokenValue(token, requestedField)
	if err != nil {
		return provider.SecretValue{}, err
	}

	// Build metadata
	metadata := map[string]string{
		"source":     "azure-identity",
		"scope":      scope,
		"token_type": "Bearer",
		"expires_at": token.ExpiresOn.Format(time.RFC3339),
	}

	return provider.SecretValue{
		Value:    value,
		Metadata: metadata,
	}, nil
}

// parseReference parses Azure Identity references
func (p *AzureIdentityProvider) parseReference(ref string) (scope, field string) {
	// Default field is the access token itself
	field = "access_token"
	
	// Check for field specification (scope:field)
	if strings.Contains(ref, ":") {
		parts := strings.SplitN(ref, ":", 2)
		scope = parts[0]
		field = parts[1]
	} else {
		scope = ref
	}
	
	return scope, field
}

// getTokenValue extracts the requested field from the access token
func (p *AzureIdentityProvider) getTokenValue(token azcore.AccessToken, field string) (string, error) {
	switch field {
	case "access_token", "token":
		return token.Token, nil
	case "expires_at", "expiration":
		return token.ExpiresOn.Format(time.RFC3339), nil
	case "expires_in":
		expiresIn := int64(time.Until(token.ExpiresOn).Seconds())
		return fmt.Sprintf("%d", expiresIn), nil
	case "token_info", "all":
		// Return all token information as JSON
		tokenInfo := map[string]interface{}{
			"access_token": token.Token,
			"token_type":   "Bearer",
			"expires_at":   token.ExpiresOn.Format(time.RFC3339),
			"expires_in":   int64(time.Until(token.ExpiresOn).Seconds()),
		}
		jsonData, err := json.Marshal(tokenInfo)
		if err != nil {
			return "", fmt.Errorf("failed to marshal token info: %w", err)
		}
		return string(jsonData), nil
	default:
		return "", dserrors.UserError{
			Message:    fmt.Sprintf("Unknown token field: %s", field),
			Suggestion: "Use one of: access_token, expires_at, expires_in, token_info",
		}
	}
}

// Describe returns metadata about the identity provider
func (p *AzureIdentityProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	scope, _ := p.parseReference(ref.Key)
	if scope == "" {
		scope = p.config.Scope
	}

	return provider.Metadata{
		Exists: true,
		Type:   "azure-identity",
		Tags: map[string]string{
			"source": "azure-identity",
			"scope":  scope,
		},
	}, nil
}

// Capabilities returns the provider's capabilities
func (p *AzureIdentityProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: false, // Tokens are not versioned
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     false, // Tokens are text-based
		RequiresAuth:       true,
		AuthMethods:        []string{"managed_identity", "service_principal", "azure_cli", "default_credential"},
	}
}

// Validate checks if the provider is properly configured and accessible
func (p *AzureIdentityProvider) Validate(ctx context.Context) error {
	// Test by getting a token for the default scope
	tokenReq := policy.TokenRequestOptions{
		Scopes: []string{p.config.Scope},
	}

	_, err := p.credential.GetToken(ctx, tokenReq)
	if err != nil {
		return dserrors.UserError{
			Message:    "Failed to validate Azure identity",
			Details:    err.Error(),
			Suggestion: getAzureIdentityErrorSuggestion(err),
		}
	}

	return nil
}

// getAzureIdentityErrorSuggestion provides helpful suggestions based on Azure Identity errors
func getAzureIdentityErrorSuggestion(err error) string {
	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "managed identity"):
		return "Check that Managed Identity is enabled and assigned appropriate roles"
	case strings.Contains(errStr, "invalid_client") || strings.Contains(errStr, "unauthorized_client"):
		return "Check service principal client ID and ensure it's registered in the correct tenant"
	case strings.Contains(errStr, "invalid_client_secret"):
		return "Check that the client secret is correct and not expired"
	case strings.Contains(errStr, "invalid_scope"):
		return "Check that the requested scope is valid (e.g., https://management.azure.com/.default)"
	case strings.Contains(errStr, "tenant"):
		return "Check that the tenant ID is correct"
	case strings.Contains(errStr, "login"):
		return "Try running 'az login' to authenticate with Azure CLI"
	case strings.Contains(errStr, "certificate"):
		return "Check certificate path and format. Ensure certificate is valid and not expired"
	case strings.Contains(errStr, "timeout"):
		return "Network timeout - check connectivity to Azure endpoints"
	default:
		return "Check Azure credentials and network connectivity. Try 'az login' or verify managed identity configuration"
	}
}

// NewAzureIdentityProviderFactory creates an Azure Identity provider factory
func NewAzureIdentityProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewAzureIdentityProvider(name, config)
}