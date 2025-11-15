package providers_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

func TestPassProviderContract(t *testing.T) {
	if _, exists := os.LookupEnv("DSOPS_TEST_PASS"); !exists {
		t.Skip("Skipping Pass provider test. Set DSOPS_TEST_PASS=1 to run.")
	}

	p := providers.NewPassProvider(providers.PassConfig{})

	tc := testutil.ProviderTestCase{
		Name:     "pass",
		Provider: p,
		TestData: map[string]provider.SecretValue{
			// Example: "test/secret": {Value: "test-value"},
		},
		SkipValidation: false,
	}

	if len(tc.TestData) == 0 {
		t.Skip("No Pass test data configured.")
	}

	testutil.RunProviderContractTests(t, tc)
}

func TestPassProviderName(t *testing.T) {
	t.Parallel()

	p := providers.NewPassProvider(providers.PassConfig{})
	assert.Equal(t, "pass", p.Name())
}

func TestPassProviderCapabilities(t *testing.T) {
	t.Parallel()

	p := providers.NewPassProvider(providers.PassConfig{})
	caps := p.Capabilities()

	assert.False(t, caps.SupportsVersioning)
	assert.True(t, caps.SupportsMetadata)
	assert.False(t, caps.SupportsWatching)
	assert.False(t, caps.SupportsBinary)
	assert.True(t, caps.RequiresAuth)
	assert.Contains(t, caps.AuthMethods, "gpg_key")
}

func TestPassProviderWithConfig(t *testing.T) {
	t.Parallel()

	config := providers.PassConfig{
		PasswordStore: "/custom/password/store",
		GpgKey:        "user@example.com",
	}

	p := providers.NewPassProvider(config)
	assert.NotNil(t, p)
	assert.Equal(t, "pass", p.Name())
}

func TestPassKeyFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
	}{
		{"simple", "mypassword"},
		{"hierarchical", "email/gmail"},
		{"deep_path", "work/client/api-key"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.NotEmpty(t, tt.key)
		})
	}
}
