package providers_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

// TestBitwardenSMProviderContract runs the contract suite against the
// Bitwarden Secrets Manager provider. Requires:
//   - bws CLI installed
//   - BWS_ACCESS_TOKEN set to a token for an org with at least one secret
//   - DSOPS_TEST_BITWARDEN_SM=1 to enable
func TestBitwardenSMProviderContract(t *testing.T) {
	if _, exists := os.LookupEnv("DSOPS_TEST_BITWARDEN_SM"); !exists {
		t.Skip("Skipping Bitwarden Secrets Manager integration test. Set DSOPS_TEST_BITWARDEN_SM=1 to run.")
	}

	if os.Getenv("BWS_ACCESS_TOKEN") == "" {
		t.Skip("BWS_ACCESS_TOKEN not set; skipping integration test.")
	}

	bwsmProvider := providers.NewBitwardenSecretsManagerProvider("test-bw-sm", nil)

	tc := testutil.ProviderTestCase{
		Name:     "bitwarden.secretsmanager",
		Provider: bwsmProvider,
		TestData: map[string]provider.SecretValue{
			// To run, populate with real secret references from your org:
			// "11111111-1111-4111-8111-111111111111": {Value: "expected-value"},
		},
		SkipValidation: false,
	}

	if len(tc.TestData) == 0 {
		t.Skip("No Bitwarden Secrets Manager test data configured. Add UUIDs to enable contract tests.")
	}

	testutil.RunProviderContractTests(t, tc)
}

func TestBitwardenSMProviderName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerName string
		want         string
	}{
		{"default_name", "bitwarden.secretsmanager", "bitwarden.secretsmanager"},
		{"custom_name", "bw-sm-prod", "bw-sm-prod"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := providers.NewBitwardenSecretsManagerProvider(tt.providerName, nil)
			assert.Equal(t, tt.want, p.Name())
		})
	}
}

func TestBitwardenSMProviderResolveNotFoundIntegration(t *testing.T) {
	if _, exists := os.LookupEnv("DSOPS_TEST_BITWARDEN_SM"); !exists {
		t.Skip("Skipping Bitwarden Secrets Manager test. Set DSOPS_TEST_BITWARDEN_SM=1 to run.")
	}
	if os.Getenv("BWS_ACCESS_TOKEN") == "" {
		t.Skip("BWS_ACCESS_TOKEN not set; skipping integration test.")
	}

	ctx := context.Background()
	p := providers.NewBitwardenSecretsManagerProvider("test-bw-sm", nil)

	ref := provider.Reference{
		Provider: "test-bw-sm",
		Key:      "00000000-0000-4000-8000-000000000000",
	}

	_, err := p.Resolve(ctx, ref)
	assert.Error(t, err, "Should return error for non-existent secret")
}
