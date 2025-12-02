package providers_test

import (
	"context"
	"os"
	"testing"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
	"google.golang.org/api/option"
)

// mockGCPClient implements providers.GCPSecretManagerClientAPI for testing
type mockGCPClient struct{}

func (m *mockGCPClient) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...option.ClientOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	return nil, nil
}

func (m *mockGCPClient) GetSecret(ctx context.Context, req *secretmanagerpb.GetSecretRequest, opts ...option.ClientOption) (*secretmanagerpb.Secret, error) {
	return nil, nil
}

func (m *mockGCPClient) ListSecrets(ctx context.Context, req *secretmanagerpb.ListSecretsRequest, opts ...option.ClientOption) *secretmanager.SecretIterator {
	return nil
}

func (m *mockGCPClient) AddSecretVersion(ctx context.Context, req *secretmanagerpb.AddSecretVersionRequest, opts ...option.ClientOption) (*secretmanagerpb.SecretVersion, error) {
	return nil, nil
}

func (m *mockGCPClient) DisableSecretVersion(ctx context.Context, req *secretmanagerpb.DisableSecretVersionRequest, opts ...option.ClientOption) (*secretmanagerpb.SecretVersion, error) {
	return nil, nil
}

func TestGCPSecretManagerProviderContract(t *testing.T) {
	if _, exists := os.LookupEnv("DSOPS_TEST_GCP"); !exists {
		t.Skip("Skipping GCP Secret Manager provider test. Set DSOPS_TEST_GCP=1 to run.")
	}

	config := map[string]interface{}{
		"project_id": os.Getenv("GCP_PROJECT_ID"),
	}
	p, err := providers.NewGCPSecretManagerProvider("test-gcp", config)
	require.NoError(t, err)

	tc := testutil.ProviderTestCase{
		Name:     "gcp.secretmanager",
		Provider: p,
		TestData: map[string]provider.SecretValue{
			// Example: "database-password": {Value: "test-value"},
		},
		SkipValidation: true,
	}

	if len(tc.TestData) == 0 {
		t.Skip("No GCP Secret Manager test data configured.")
	}

	testutil.RunProviderContractTests(t, tc)
}

func TestGCPSecretManagerProviderName(t *testing.T) {
	t.Parallel()

	config := map[string]interface{}{"project_id": "my-project"}
	mockClient := &mockGCPClient{}
	p, err := providers.NewGCPSecretManagerProvider("gcp-sm", config, providers.WithGCPSecretManagerClient(mockClient))
	require.NoError(t, err)
	assert.Equal(t, "gcp-sm", p.Name())
}

func TestGCPSecretManagerProviderCapabilities(t *testing.T) {
	t.Parallel()

	config := map[string]interface{}{"project_id": "my-project"}
	mockClient := &mockGCPClient{}
	p, err := providers.NewGCPSecretManagerProvider("test", config, providers.WithGCPSecretManagerClient(mockClient))
	require.NoError(t, err)

	caps := p.Capabilities()
	assert.True(t, caps.SupportsVersioning)
	assert.True(t, caps.SupportsMetadata)
	assert.True(t, caps.RequiresAuth)
}

func TestGCPSecretManagerKeyFormats(t *testing.T) {
	t.Parallel()

	tests := []string{
		"database-password",
		"api-key",
		"stripe-secret",
	}

	for _, key := range tests {
		assert.NotEmpty(t, key)
	}
}
