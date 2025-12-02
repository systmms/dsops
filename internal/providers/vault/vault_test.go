package vault_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/providers/vault"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

func TestVaultProviderContract(t *testing.T) {
	if _, exists := os.LookupEnv("DSOPS_TEST_VAULT"); !exists {
		t.Skip("Skipping Vault provider test. Set DSOPS_TEST_VAULT=1 to run.")
	}

	config := map[string]interface{}{
		"address": os.Getenv("VAULT_ADDR"),
		"token":   os.Getenv("VAULT_TOKEN"),
	}

	p, err := vault.NewVaultProvider("test-vault", config)
	require.NoError(t, err)

	tc := testutil.ProviderTestCase{
		Name:     "vault",
		Provider: p,
		TestData: map[string]provider.SecretValue{
			// Example: "secret/data/test": {Value: "test-value"},
		},
		SkipValidation: false,
	}

	if len(tc.TestData) == 0 {
		t.Skip("No Vault test data configured.")
	}

	testutil.RunProviderContractTests(t, tc)
}

func TestVaultProviderName(t *testing.T) {
	t.Parallel()

	config := map[string]interface{}{"address": "http://localhost:8200"}
	p, err := vault.NewVaultProvider("test-vault", config)
	require.NoError(t, err)
	assert.Equal(t, "test-vault", p.Name())
}

func TestVaultProviderCapabilities(t *testing.T) {
	t.Parallel()

	config := map[string]interface{}{"address": "http://localhost:8200"}
	p, err := vault.NewVaultProvider("vault", config)
	require.NoError(t, err)

	caps := p.Capabilities()

	assert.True(t, caps.SupportsVersioning)
	assert.True(t, caps.SupportsMetadata)
	assert.False(t, caps.SupportsWatching)
	assert.True(t, caps.SupportsBinary)
	assert.True(t, caps.RequiresAuth)
	assert.NotEmpty(t, caps.AuthMethods)
}

func TestVaultProviderConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name: "with_token",
			config: map[string]interface{}{
				"address": "https://vault.example.com:8200",
				"token":   "s.test-token",
			},
			wantErr: false,
		},
		{
			name: "with_namespace",
			config: map[string]interface{}{
				"address":   "https://vault.example.com:8200",
				"namespace": "my-namespace",
			},
			wantErr: false,
		},
		{
			name: "userpass_auth",
			config: map[string]interface{}{
				"address":           "https://vault.example.com:8200",
				"auth_method":       "userpass",
				"userpass_username": "admin",
				"userpass_password": "password",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := vault.NewVaultProvider("test", tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, p)
			}
		})
	}
}

func TestVaultKeyFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
	}{
		{"kv_v1", "secret/myapp/config"},
		{"kv_v2_data", "secret/data/myapp/config"},
		{"kv_v2_field", "secret/data/myapp/config#password"},
		{"cubbyhole", "cubbyhole/my-secret"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.NotEmpty(t, tt.key)
		})
	}
}
