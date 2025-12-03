package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
)

func TestNewLoginCommand(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewLoginCommand(cfg)

	assert.Equal(t, "login [provider]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify flags exist
	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("list"))
	assert.NotNil(t, flags.Lookup("interactive"))
}

func TestShowAvailableProviders(t *testing.T) {
	t.Parallel()

	// Should execute without error
	err := showAvailableProviders()
	assert.NoError(t, err)
}

func TestAuthenticateProvider_UnknownProvider(t *testing.T) {
	t.Parallel()

	err := authenticateProvider("unknown-provider-xyz", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown provider")
}

func TestAuthenticateProvider_KnownProviders(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		provider string
	}{
		{"bitwarden", "bitwarden"},
		{"bitwarden_alias", "bw"},
		{"1password", "1password"},
		{"1password_alias", "op"},
		{"aws", "aws"},
		{"aws_ssm", "aws.ssm"},
		{"aws_sts", "aws.sts"},
		{"aws_sso", "aws.sso"},
		{"gcp", "gcp"},
		{"gcp_alias", "google"},
		{"azure", "azure"},
		{"azure_alias", "az"},
		{"vault", "vault"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Should not error (may print CLI not found messages but that's expected)
			err := authenticateProvider(tc.provider, false)
			assert.NoError(t, err)
		})
	}
}

func TestAuthenticateBitwarden_NonInteractive(t *testing.T) {
	t.Parallel()

	// Should not error even if CLI is not installed
	err := authenticateBitwarden(false)
	assert.NoError(t, err)
}

func TestAuthenticateOnePassword_NonInteractive(t *testing.T) {
	t.Parallel()

	err := authenticateOnePassword(false)
	assert.NoError(t, err)
}

func TestAuthenticateAWS_NonInteractive(t *testing.T) {
	t.Parallel()

	err := authenticateAWS(false)
	assert.NoError(t, err)
}

func TestAuthenticateGCP_NonInteractive(t *testing.T) {
	t.Parallel()

	err := authenticateGCP(false)
	assert.NoError(t, err)
}

func TestAuthenticateAzure_NonInteractive(t *testing.T) {
	t.Parallel()

	err := authenticateAzure(false)
	assert.NoError(t, err)
}

func TestAuthenticateVault_NonInteractive(t *testing.T) {
	t.Parallel()

	err := authenticateVault(false)
	assert.NoError(t, err)
}

func TestAuthenticateAWSSSM_NonInteractive(t *testing.T) {
	t.Parallel()

	err := authenticateAWSSSM(false)
	assert.NoError(t, err)
}

func TestAuthenticateAWSSTS_NonInteractive(t *testing.T) {
	t.Parallel()

	err := authenticateAWSSTS(false)
	assert.NoError(t, err)
}

func TestAuthenticateAWSSSO_NonInteractive(t *testing.T) {
	t.Parallel()

	err := authenticateAWSSSO(false)
	assert.NoError(t, err)
}
