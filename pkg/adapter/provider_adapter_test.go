package adapter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/pkg/secretstore"
	"github.com/systmms/dsops/pkg/service"
)

// Mock provider for testing
type mockProvider struct {
	name string
	supportsRotation bool
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	return provider.SecretValue{
		Value:     "test-secret-value",
		Version:   "v1",
		UpdatedAt: time.Now(),
		Metadata:  map[string]string{"source": "mock"},
	}, nil
}

func (m *mockProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	return provider.Metadata{
		Exists:      true,
		Version:     "v1",
		UpdatedAt:   time.Now(),
		Size:        100,
		Type:        "text",
		Permissions: []string{"read-only"},
		Tags:        map[string]string{"env": "test"},
	}, nil
}

func (m *mockProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: true,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     false,
		RequiresAuth:       true,
		AuthMethods:        []string{"token"},
	}
}

func (m *mockProvider) Validate(ctx context.Context) error {
	return nil
}

// Mock provider with rotation support
type mockRotator struct {
	mockProvider
}

func (m *mockRotator) CreateNewVersion(ctx context.Context, ref provider.Reference, value []byte, metadata map[string]string) (string, error) {
	return "v2", nil
}

func (m *mockRotator) DeprecateVersion(ctx context.Context, ref provider.Reference, version string) error {
	return nil
}

func (m *mockRotator) GetRotationMetadata(ctx context.Context, ref provider.Reference) (provider.RotationMetadata, error) {
	lastRotated := time.Now().Add(-30 * 24 * time.Hour)
	nextRotation := time.Now().Add(24 * time.Hour)
	return provider.RotationMetadata{
		SupportsRotation:   true,
		SupportsVersioning: true,
		RotationInterval:   "30d",
		LastRotated:        &lastRotated,
		NextRotation:       &nextRotation,
		Constraints:        map[string]string{"strategy": "immediate"},
	}, nil
}

// Mock secret store for testing
type mockSecretStore struct {
	name string
}

func (m *mockSecretStore) Name() string {
	return m.name
}

func (m *mockSecretStore) Resolve(ctx context.Context, ref secretstore.SecretRef) (secretstore.SecretValue, error) {
	return secretstore.SecretValue{
		Value:     "secret-from-store",
		Version:   "v2",
		UpdatedAt: time.Now(),
		Metadata:  map[string]string{"store": "mock"},
	}, nil
}

func (m *mockSecretStore) Describe(ctx context.Context, ref secretstore.SecretRef) (secretstore.SecretMetadata, error) {
	return secretstore.SecretMetadata{
		Exists:      true,
		Version:     "v2",
		UpdatedAt:   time.Now(),
		Size:        200,
		Type:        "binary",
		Permissions: []string{"read-write"},
		Tags:        map[string]string{"team": "platform"},
	}, nil
}

func (m *mockSecretStore) Capabilities() secretstore.SecretStoreCapabilities {
	return secretstore.SecretStoreCapabilities{
		SupportsVersioning: true,
		SupportsMetadata:   true,
		SupportsWatching:   true,
		SupportsBinary:     true,
		RequiresAuth:       false,
		AuthMethods:        []string{"oauth"},
		Rotation: &secretstore.RotationCapabilities{
			SupportsRotation:   true,
			SupportsVersioning: true,
			MaxVersions:        10,
			MinRotationTime:    24 * time.Hour,
		},
	}
}

func (m *mockSecretStore) Validate(ctx context.Context) error {
	return nil
}

func TestProviderToSecretStoreAdapter(t *testing.T) {
	ctx := context.Background()
	mockProv := &mockProvider{name: "test-provider", supportsRotation: false}
	adapter := NewProviderToSecretStoreAdapter(mockProv)

	t.Run("Name", func(t *testing.T) {
		assert.Equal(t, "test-provider", adapter.Name())
	})

	t.Run("Resolve", func(t *testing.T) {
		ref := secretstore.SecretRef{
			Store:   "test-provider",
			Path:    "/path/to/secret",
			Field:   "password",
			Version: "v1",
			Options: map[string]string{"key": "test-key"},
		}

		value, err := adapter.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, "test-secret-value", value.Value)
		assert.Equal(t, "v1", value.Version)
		assert.Equal(t, "mock", value.Metadata["source"])
	})

	t.Run("Describe", func(t *testing.T) {
		ref := secretstore.SecretRef{
			Store:   "test-provider",
			Path:    "/path/to/secret",
			Version: "v1",
		}

		metadata, err := adapter.Describe(ctx, ref)
		require.NoError(t, err)
		assert.True(t, metadata.Exists)
		assert.Equal(t, "v1", metadata.Version)
		assert.Equal(t, 100, metadata.Size)
		assert.Equal(t, "text", metadata.Type)
		assert.Equal(t, "test", metadata.Tags["env"])
	})

	t.Run("Capabilities", func(t *testing.T) {
		caps := adapter.Capabilities()
		assert.True(t, caps.SupportsVersioning)
		assert.True(t, caps.SupportsMetadata)
		assert.False(t, caps.SupportsWatching)
		assert.False(t, caps.SupportsBinary)
		assert.True(t, caps.RequiresAuth)
		assert.Contains(t, caps.AuthMethods, "token")
		assert.Nil(t, caps.Rotation) // No rotation support in base provider
	})

	t.Run("Validate", func(t *testing.T) {
		err := adapter.Validate(ctx)
		assert.NoError(t, err)
	})
}

func TestProviderToSecretStoreAdapterWithRotation(t *testing.T) {
	mockRotator := &mockRotator{
		mockProvider: mockProvider{name: "test-rotator", supportsRotation: true},
	}
	adapter := NewProviderToSecretStoreAdapter(mockRotator)

	t.Run("CapabilitiesWithRotation", func(t *testing.T) {
		caps := adapter.Capabilities()
		assert.NotNil(t, caps.Rotation)
		assert.True(t, caps.Rotation.SupportsRotation)
		assert.True(t, caps.Rotation.SupportsVersioning)
	})
}

func TestSecretStoreToProviderAdapter(t *testing.T) {
	ctx := context.Background()
	mockStore := &mockSecretStore{name: "test-store"}
	adapter := NewSecretStoreToProviderAdapter(mockStore)

	t.Run("Name", func(t *testing.T) {
		assert.Equal(t, "test-store", adapter.Name())
	})

	t.Run("Resolve", func(t *testing.T) {
		ref := provider.Reference{
			Provider: "test-store",
			Key:      "test-key",
			Path:     "/path/to/secret",
			Field:    "password",
			Version:  "v2",
		}

		value, err := adapter.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, "secret-from-store", value.Value)
		assert.Equal(t, "v2", value.Version)
		assert.Equal(t, "mock", value.Metadata["store"])
	})

	t.Run("Describe", func(t *testing.T) {
		ref := provider.Reference{
			Provider: "test-store",
			Path:     "/path/to/secret",
			Version:  "v2",
		}

		metadata, err := adapter.Describe(ctx, ref)
		require.NoError(t, err)
		assert.True(t, metadata.Exists)
		assert.Equal(t, "v2", metadata.Version)
		assert.Equal(t, 200, metadata.Size)
		assert.Equal(t, "binary", metadata.Type)
		assert.Equal(t, "platform", metadata.Tags["team"])
	})

	t.Run("Capabilities", func(t *testing.T) {
		caps := adapter.Capabilities()
		assert.True(t, caps.SupportsVersioning)
		assert.True(t, caps.SupportsMetadata)
		assert.True(t, caps.SupportsWatching)
		assert.True(t, caps.SupportsBinary)
		assert.False(t, caps.RequiresAuth)
		assert.Contains(t, caps.AuthMethods, "oauth")
	})

	t.Run("Validate", func(t *testing.T) {
		err := adapter.Validate(ctx)
		assert.NoError(t, err)
	})
}

func TestProviderToServiceAdapter(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateWithNonRotator", func(t *testing.T) {
		mockProv := &mockProvider{name: "test-provider"}
		_, err := NewProviderToServiceAdapter(mockProv)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not implement Rotator interface")
	})

	t.Run("CreateWithRotator", func(t *testing.T) {
		mockRotator := &mockRotator{
			mockProvider: mockProvider{name: "test-rotator"},
		}
		adapter, err := NewProviderToServiceAdapter(mockRotator)
		require.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, "test-rotator", adapter.Name())
	})

	t.Run("Plan", func(t *testing.T) {
		mockRotator := &mockRotator{
			mockProvider: mockProvider{name: "test-rotator"},
		}
		adapter, _ := NewProviderToServiceAdapter(mockRotator)

		req := service.RotationRequest{
			ServiceRef: service.ServiceRef{
				Type:      "github",
				Instance:  "acme-org",
				Kind:      "pat",
				Principal: "ci-bot",
			},
			Strategy: "immediate",
			Metadata: map[string]string{"reason": "test"},
		}

		plan, err := adapter.Plan(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, req.ServiceRef, plan.ServiceRef)
		assert.Equal(t, "immediate", plan.Strategy)
		assert.Len(t, plan.Steps, 3)
		assert.Equal(t, "create", plan.Steps[0].Action)
		assert.Equal(t, "verify", plan.Steps[1].Action)
		assert.Equal(t, "deprecate", plan.Steps[2].Action)
	})

	t.Run("Execute", func(t *testing.T) {
		mockRotator := &mockRotator{
			mockProvider: mockProvider{name: "test-rotator"},
		}
		adapter, _ := NewProviderToServiceAdapter(mockRotator)

		plan := service.RotationPlan{
			ServiceRef: service.ServiceRef{
				Type:      "github",
				Instance:  "acme-org",
				Kind:      "pat",
				Principal: "ci-bot",
			},
			Strategy: "immediate",
			Steps: []service.RotationStep{
				{Name: "create_new_version", Action: "create"},
				{Name: "verify_new_version", Action: "verify"},
				{Name: "deprecate_old_version", Action: "deprecate"},
			},
		}

		result, err := adapter.Execute(ctx, plan)
		require.NoError(t, err)
		assert.Equal(t, "success", result.Status)
		assert.Len(t, result.ExecutedSteps, 3)
		assert.Equal(t, "success", result.ExecutedSteps[0].Status)
		assert.Contains(t, result.ExecutedSteps[0].Output, "Created version: v2")
	})

	t.Run("GetStatus", func(t *testing.T) {
		mockRotator := &mockRotator{
			mockProvider: mockProvider{name: "test-rotator"},
		}
		adapter, _ := NewProviderToServiceAdapter(mockRotator)

		ref := service.ServiceRef{
			Type:      "github",
			Instance:  "acme-org",
			Kind:      "pat",
			Principal: "ci-bot",
		}

		status, err := adapter.GetStatus(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, "needs_rotation", status.Status)
		assert.NotNil(t, status.NextRotation)
	})

	t.Run("Capabilities", func(t *testing.T) {
		mockRotator := &mockRotator{
			mockProvider: mockProvider{name: "test-rotator"},
		}
		adapter, _ := NewProviderToServiceAdapter(mockRotator)

		caps := adapter.Capabilities()
		assert.Contains(t, caps.SupportedStrategies, "immediate")
		assert.Equal(t, 1, caps.MaxActiveKeys)
		assert.True(t, caps.SupportsVersioning)
		assert.True(t, caps.SupportsRevocation)
	})
}

func TestReferenceConversion(t *testing.T) {
	t.Run("ConvertProviderRefToSecretRef", func(t *testing.T) {
		provRef := provider.Reference{
			Provider: "bitwarden",
			Key:      "mykey",
			Path:     "/path/to/secret",
			Field:    "password",
			Version:  "v1",
		}

		secretRef := ConvertProviderRefToSecretRef(provRef)
		assert.Equal(t, "bitwarden", secretRef.Store)
		assert.Equal(t, "/path/to/secret", secretRef.Path)
		assert.Equal(t, "password", secretRef.Field)
		assert.Equal(t, "v1", secretRef.Version)
		assert.Equal(t, "mykey", secretRef.Options["key"])
	})

	t.Run("ConvertSecretRefToProviderRef", func(t *testing.T) {
		secretRef := secretstore.SecretRef{
			Store:   "vault",
			Path:    "/secret/data",
			Field:   "api_key",
			Version: "v2",
			Options: map[string]string{
				"key": "legacy-key",
			},
		}

		provRef := ConvertSecretRefToProviderRef(secretRef)
		assert.Equal(t, "vault", provRef.Provider)
		assert.Equal(t, "legacy-key", provRef.Key)
		assert.Equal(t, "/secret/data", provRef.Path)
		assert.Equal(t, "api_key", provRef.Field)
		assert.Equal(t, "v2", provRef.Version)
	})

	t.Run("BidirectionalConversion", func(t *testing.T) {
		// Test Provider -> Secret -> Provider
		originalProvider := provider.Reference{
			Provider: "aws.secretsmanager",
			Key:      "prod/db/password",
			Path:     "/prod/db/password",
			Field:    "password",
			Version:  "AWSCURRENT",
		}

		secretRef := ConvertProviderRefToSecretRef(originalProvider)
		backToProvider := ConvertSecretRefToProviderRef(secretRef)

		assert.Equal(t, originalProvider.Provider, backToProvider.Provider)
		assert.Equal(t, originalProvider.Key, backToProvider.Key)
		assert.Equal(t, originalProvider.Path, backToProvider.Path)
		assert.Equal(t, originalProvider.Field, backToProvider.Field)
		assert.Equal(t, originalProvider.Version, backToProvider.Version)

		// Test Secret -> Provider -> Secret
		originalSecret := secretstore.SecretRef{
			Store:   "gcp.secretmanager",
			Path:    "projects/123/secrets/api-key",
			Field:   "value",
			Version: "latest",
			Options: map[string]string{
				"key":    "api-key",
				"region": "us-central1",
			},
		}

		provRef := ConvertSecretRefToProviderRef(originalSecret)
		backToSecret := ConvertProviderRefToSecretRef(provRef)

		assert.Equal(t, originalSecret.Store, backToSecret.Store)
		assert.Equal(t, originalSecret.Path, backToSecret.Path)
		assert.Equal(t, originalSecret.Field, backToSecret.Field)
		assert.Equal(t, originalSecret.Version, backToSecret.Version)
		assert.Equal(t, originalSecret.Options["key"], backToSecret.Options["key"])
	})
}

func TestProviderClassification(t *testing.T) {
	t.Run("IsSecretStore", func(t *testing.T) {
		secretStores := []string{
			"bitwarden",
			"onepassword",
			"lastpass",
			"keeper",
			"vault",
			"aws.secretsmanager",
			"gcp.secretmanager", 
			"azure.keyvault",
		}

		for _, name := range secretStores {
			assert.True(t, IsSecretStore(name), "%s should be classified as SecretStore", name)
		}

		notSecretStores := []string{
			"github",
			"postgres",
			"stripe",
		}

		for _, name := range notSecretStores {
			assert.False(t, IsSecretStore(name), "%s should not be classified as SecretStore", name)
		}
	})

	t.Run("IsService", func(t *testing.T) {
		services := []string{
			"github",
			"gitlab",
			"postgres",
			"mysql",
			"redis",
			"stripe",
			"datadog",
			"aws.iam",
		}

		for _, name := range services {
			assert.True(t, IsService(name), "%s should be classified as Service", name)
		}

		notServices := []string{
			"bitwarden",
			"vault",
			"onepassword",
		}

		for _, name := range notServices {
			assert.False(t, IsService(name), "%s should not be classified as Service", name)
		}
	})

	t.Run("CaseInsensitive", func(t *testing.T) {
		assert.True(t, IsSecretStore("BITWARDEN"))
		assert.True(t, IsSecretStore("Vault"))
		assert.True(t, IsService("GITHUB"))
		assert.True(t, IsService("Postgres"))
	})
}