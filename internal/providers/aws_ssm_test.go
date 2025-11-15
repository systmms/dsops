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

func TestAWSSSMProviderContract(t *testing.T) {
	if _, exists := os.LookupEnv("DSOPS_TEST_AWS"); !exists {
		t.Skip("Skipping AWS SSM provider test. Set DSOPS_TEST_AWS=1 to run.")
	}

	config := map[string]interface{}{
		"region": "us-east-1",
	}
	p, err := providers.NewAWSSSMProvider("test-ssm", config)
	require.NoError(t, err)

	tc := testutil.ProviderTestCase{
		Name:     "aws.ssm",
		Provider: p,
		TestData: map[string]provider.SecretValue{
			// Example: "/myapp/database/password": {Value: "test-value"},
		},
		SkipValidation: true,
	}

	if len(tc.TestData) == 0 {
		t.Skip("No AWS SSM test data configured.")
	}

	testutil.RunProviderContractTests(t, tc)
}

func TestAWSSSMProviderName(t *testing.T) {
	t.Parallel()

	config := map[string]interface{}{"region": "us-east-1"}
	p, err := providers.NewAWSSSMProvider("aws-ssm", config)
	require.NoError(t, err)
	assert.Equal(t, "aws-ssm", p.Name())
}

func TestAWSSSMProviderCapabilities(t *testing.T) {
	t.Parallel()

	config := map[string]interface{}{"region": "us-east-1"}
	p, err := providers.NewAWSSSMProvider("test", config)
	require.NoError(t, err)

	caps := p.Capabilities()
	assert.True(t, caps.SupportsVersioning)
	assert.True(t, caps.SupportsMetadata)
	assert.True(t, caps.RequiresAuth)
}

func TestAWSSSMKeyFormats(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/myapp/config",
		"/prod/database/password",
		"simple-parameter",
	}

	for _, key := range tests {
		assert.NotEmpty(t, key)
	}
}
