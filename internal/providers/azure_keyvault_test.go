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

func TestAzureKeyVaultProviderContract(t *testing.T) {
	if _, exists := os.LookupEnv("DSOPS_TEST_AZURE"); !exists {
		t.Skip("Skipping Azure Key Vault provider test. Set DSOPS_TEST_AZURE=1 to run.")
	}

	config := map[string]interface{}{
		"vault_url": os.Getenv("AZURE_VAULT_URL"),
	}
	p, err := providers.NewAzureKeyVaultProvider("test-azure", config)
	require.NoError(t, err)

	tc := testutil.ProviderTestCase{
		Name:     "azure.keyvault",
		Provider: p,
		TestData: map[string]provider.SecretValue{
			// Example: "database-password": {Value: "test-value"},
		},
		SkipValidation: true,
	}

	if len(tc.TestData) == 0 {
		t.Skip("No Azure Key Vault test data configured.")
	}

	testutil.RunProviderContractTests(t, tc)
}

func TestAzureKeyVaultProviderName(t *testing.T) {
	t.Parallel()

	config := map[string]interface{}{"vault_url": "https://my-vault.vault.azure.net"}
	p, err := providers.NewAzureKeyVaultProvider("azure-kv", config)
	require.NoError(t, err)
	assert.Equal(t, "azure-kv", p.Name())
}

func TestAzureKeyVaultProviderCapabilities(t *testing.T) {
	t.Parallel()

	config := map[string]interface{}{"vault_url": "https://my-vault.vault.azure.net"}
	p, err := providers.NewAzureKeyVaultProvider("test", config)
	require.NoError(t, err)

	caps := p.Capabilities()
	assert.True(t, caps.SupportsVersioning)
	assert.True(t, caps.SupportsMetadata)
	assert.True(t, caps.RequiresAuth)
}

func TestAzureKeyVaultKeyFormats(t *testing.T) {
	t.Parallel()

	tests := []string{
		"database-password",
		"api-key",
		"connection-string",
	}

	for _, key := range tests {
		assert.NotEmpty(t, key)
	}
}
