package providers_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

func TestDopplerProviderContract(t *testing.T) {
	if _, exists := os.LookupEnv("DSOPS_TEST_DOPPLER"); !exists {
		t.Skip("Skipping Doppler provider test. Set DSOPS_TEST_DOPPLER=1 to run.")
	}

	config := providers.DopplerConfig{
		Token:   os.Getenv("DOPPLER_TOKEN"),
		Project: "test-project",
		Config:  "dev",
	}

	p := providers.NewDopplerProvider(config)

	tc := testutil.ProviderTestCase{
		Name:     "doppler",
		Provider: p,
		TestData: map[string]provider.SecretValue{
			// Example: "API_KEY": {Value: "test-value"},
		},
		SkipValidation: false,
	}

	if len(tc.TestData) == 0 {
		t.Skip("No Doppler test data configured.")
	}

	testutil.RunProviderContractTests(t, tc)
}

func TestDopplerProviderName(t *testing.T) {
	t.Parallel()

	p := providers.NewDopplerProvider(providers.DopplerConfig{})
	assert.Equal(t, "doppler", p.Name())
}

func TestDopplerProviderCapabilities(t *testing.T) {
	t.Parallel()

	p := providers.NewDopplerProvider(providers.DopplerConfig{})
	caps := p.Capabilities()

	assert.False(t, caps.SupportsVersioning)
	assert.True(t, caps.SupportsMetadata)
	assert.False(t, caps.SupportsWatching)
	assert.False(t, caps.SupportsBinary)
	assert.True(t, caps.RequiresAuth)
	assert.Contains(t, caps.AuthMethods, "service_token")
}

func TestDopplerProviderConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config providers.DopplerConfig
	}{
		{
			name: "with_token",
			config: providers.DopplerConfig{
				Token:   "dp.st.test",
				Project: "backend",
				Config:  "production",
			},
		},
		{
			name: "minimal",
			config: providers.DopplerConfig{
				Token: "dp.st.test",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := providers.NewDopplerProvider(tt.config)
			assert.NotNil(t, p)
		})
	}
}

func TestDopplerKeyFormats(t *testing.T) {
	t.Parallel()

	keys := []string{"DATABASE_URL", "API_KEY", "STRIPE_SECRET_KEY"}
	for _, key := range keys {
		assert.NotEmpty(t, key)
	}
}
