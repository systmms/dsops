package fakes

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

// AzureKeyVaultAPI defines the interface for Azure Key Vault operations
// This matches the subset of methods used by AzureKeyVaultProvider
type AzureKeyVaultAPI interface {
	GetSecret(ctx context.Context, name string, version string, options *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error)
}

// FakeAzureKeyVaultClient is a mock implementation of AzureKeyVaultAPI
type FakeAzureKeyVaultClient struct {
	// Secrets maps secret names to their data
	Secrets map[string]*AzureSecretData
	// Errors maps secret names to errors to return
	Errors map[string]error
	// GetSecretFunc allows custom behavior for GetSecret
	GetSecretFunc func(ctx context.Context, name string, version string) (azsecrets.GetSecretResponse, error)
	// ListSecretsFunc allows custom behavior for listing secrets
	ListSecretsFunc func(ctx context.Context) ([]azsecrets.SecretProperties, error)
}

// AzureSecretData holds the data for a mock Azure Key Vault secret
type AzureSecretData struct {
	Value      *string
	ID         *string
	Attributes *azsecrets.SecretAttributes
	Tags       map[string]*string
	ContentType *string
	// Version-specific data
	Versions map[string]*AzureSecretVersion
}

// AzureSecretVersion holds version-specific data for a secret
type AzureSecretVersion struct {
	Value      *string
	Attributes *azsecrets.SecretAttributes
}

// NewFakeAzureKeyVaultClient creates a new mock Azure Key Vault client
func NewFakeAzureKeyVaultClient() *FakeAzureKeyVaultClient {
	return &FakeAzureKeyVaultClient{
		Secrets: make(map[string]*AzureSecretData),
		Errors:  make(map[string]error),
	}
}

// AddSecret adds a secret to the mock client
func (f *FakeAzureKeyVaultClient) AddSecret(name string, data *AzureSecretData) {
	f.Secrets[name] = data
}

// AddSecretString adds a string secret to the mock client
func (f *FakeAzureKeyVaultClient) AddSecretString(name, value string) {
	now := time.Now()
	f.Secrets[name] = &AzureSecretData{
		Value: to.Ptr(value),
		ID:    to.Ptr(fmt.Sprintf("https://test-vault.vault.azure.net/secrets/%s", name)),
		Attributes: &azsecrets.SecretAttributes{
			Enabled:   to.Ptr(true),
			Created:   &now,
			Updated:   &now,
			RecoveryLevel: to.Ptr("Recoverable+Purgeable"),
		},
		Versions: make(map[string]*AzureSecretVersion),
	}
}

// AddSecretWithVersion adds a secret with a specific version
func (f *FakeAzureKeyVaultClient) AddSecretWithVersion(name, value, version string) {
	now := time.Now()

	// If secret doesn't exist, create it
	if _, exists := f.Secrets[name]; !exists {
		f.Secrets[name] = &AzureSecretData{
			Value: to.Ptr(value),
			ID:    to.Ptr(fmt.Sprintf("https://test-vault.vault.azure.net/secrets/%s/%s", name, version)),
			Attributes: &azsecrets.SecretAttributes{
				Enabled:   to.Ptr(true),
				Created:   &now,
				Updated:   &now,
				RecoveryLevel: to.Ptr("Recoverable+Purgeable"),
			},
			Versions: make(map[string]*AzureSecretVersion),
		}
	}

	// Add version
	f.Secrets[name].Versions[version] = &AzureSecretVersion{
		Value: to.Ptr(value),
		Attributes: &azsecrets.SecretAttributes{
			Enabled:   to.Ptr(true),
			Created:   &now,
			Updated:   &now,
			RecoveryLevel: to.Ptr("Recoverable+Purgeable"),
		},
	}
}

// AddSecretWithTags adds a secret with tags
func (f *FakeAzureKeyVaultClient) AddSecretWithTags(name, value string, tags map[string]*string) {
	now := time.Now()
	f.Secrets[name] = &AzureSecretData{
		Value: to.Ptr(value),
		ID:    to.Ptr(fmt.Sprintf("https://test-vault.vault.azure.net/secrets/%s", name)),
		Attributes: &azsecrets.SecretAttributes{
			Enabled:   to.Ptr(true),
			Created:   &now,
			Updated:   &now,
			RecoveryLevel: to.Ptr("Recoverable+Purgeable"),
		},
		Tags:     tags,
		Versions: make(map[string]*AzureSecretVersion),
	}
}

// AddError configures the mock to return an error for a specific secret
func (f *FakeAzureKeyVaultClient) AddError(name string, err error) {
	f.Errors[name] = err
}

// GetSecret mocks the GetSecret operation
func (f *FakeAzureKeyVaultClient) GetSecret(ctx context.Context, name string, version string, options *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
	if f.GetSecretFunc != nil {
		return f.GetSecretFunc(ctx, name, version)
	}

	// Check for configured errors
	if err, exists := f.Errors[name]; exists {
		return azsecrets.GetSecretResponse{}, err
	}

	// Check if secret exists
	data, exists := f.Secrets[name]
	if !exists {
		return azsecrets.GetSecretResponse{}, &azcore.ResponseError{
			StatusCode: 404,
			ErrorCode:  "SecretNotFound",
			RawResponse: nil,
		}
	}

	// Handle version-specific requests
	if version != "" {
		versionData, versionExists := data.Versions[version]
		if !versionExists {
			return azsecrets.GetSecretResponse{}, &azcore.ResponseError{
				StatusCode: 404,
				ErrorCode:  "SecretNotFound",
				RawResponse: nil,
			}
		}

		return azsecrets.GetSecretResponse{
			Secret: azsecrets.Secret{
				ID:         (*azsecrets.ID)(to.Ptr(fmt.Sprintf("https://test-vault.vault.azure.net/secrets/%s/%s", name, version))),
				Value:      versionData.Value,
				Attributes: versionData.Attributes,
			},
		}, nil
	}

	// Return latest version
	return azsecrets.GetSecretResponse{
		Secret: azsecrets.Secret{
			ID:          (*azsecrets.ID)(data.ID),
			Value:       data.Value,
			Attributes:  data.Attributes,
			Tags:        data.Tags,
			ContentType: data.ContentType,
		},
	}, nil
}

// FakeAzureKeyVaultPager is a simplified mock pager for testing
type FakeAzureKeyVaultPager struct {
	secrets []azsecrets.SecretProperties
	index   int
	err     error
}

// NewFakeAzureKeyVaultPager creates a new mock pager
func NewFakeAzureKeyVaultPager(secrets []azsecrets.SecretProperties, err error) *FakeAzureKeyVaultPager {
	return &FakeAzureKeyVaultPager{
		secrets: secrets,
		index:   0,
		err:     err,
	}
}

// NextPage simulates getting the next page of results
func (p *FakeAzureKeyVaultPager) NextPage(ctx context.Context) (azsecrets.ListSecretPropertiesResponse, error) {
	if p.err != nil {
		return azsecrets.ListSecretPropertiesResponse{}, p.err
	}

	if p.index >= len(p.secrets) {
		return azsecrets.ListSecretPropertiesResponse{
			SecretPropertiesListResult: azsecrets.SecretPropertiesListResult{
				Value: []*azsecrets.SecretProperties{},
			},
		}, nil
	}

	// Return all secrets in one page for simplicity
	var secretPtrs []*azsecrets.SecretProperties
	for i := range p.secrets {
		secretPtrs = append(secretPtrs, &p.secrets[i])
	}
	p.index = len(p.secrets)

	return azsecrets.ListSecretPropertiesResponse{
		SecretPropertiesListResult: azsecrets.SecretPropertiesListResult{
			Value: secretPtrs,
		},
	}, nil
}

// More returns true if there are more pages
func (p *FakeAzureKeyVaultPager) More() bool {
	return p.index < len(p.secrets)
}

// AzureNotFoundError creates a mock Azure not found error
func AzureNotFoundError(secretName string) error {
	return &azcore.ResponseError{
		StatusCode: 404,
		ErrorCode:  "SecretNotFound",
		RawResponse: nil,
	}
}

// AzureForbiddenError creates a mock Azure forbidden error
func AzureForbiddenError(message string) error {
	return &azcore.ResponseError{
		StatusCode: 403,
		ErrorCode:  "Forbidden",
		RawResponse: nil,
	}
}

// AzureUnauthorizedError creates a mock Azure unauthorized error
func AzureUnauthorizedError(message string) error {
	return &azcore.ResponseError{
		StatusCode: 401,
		ErrorCode:  "Unauthorized",
		RawResponse: nil,
	}
}

// AzureThrottledError creates a mock Azure throttled error
func AzureThrottledError() error {
	return &azcore.ResponseError{
		StatusCode: 429,
		ErrorCode:  "TooManyRequests",
		RawResponse: nil,
	}
}
