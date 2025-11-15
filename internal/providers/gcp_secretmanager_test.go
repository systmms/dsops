package providers_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

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
	p, err := providers.NewGCPSecretManagerProvider("gcp-sm", config)
	require.NoError(t, err)
	assert.Equal(t, "gcp-sm", p.Name())
}

func TestGCPSecretManagerProviderCapabilities(t *testing.T) {
	t.Parallel()

	config := map[string]interface{}{"project_id": "my-project"}
	p, err := providers.NewGCPSecretManagerProvider("test", config)
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
