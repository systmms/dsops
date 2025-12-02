package providers_test

import (
	"context"
	osExec "os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

func TestDopplerProviderWithMockExecutor_Resolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		secretName  string
		mockOutput  string
		wantValue   string
		wantErr     bool
		errContains string
	}{
		{
			name:       "simple secret",
			secretName: "API_KEY",
			mockOutput: `{"name": "API_KEY", "value": "sk-live-1234567890"}`,
			wantValue:  "sk-live-1234567890",
		},
		{
			name:       "database connection string",
			secretName: "DATABASE_URL",
			mockOutput: `{"name": "DATABASE_URL", "value": "postgres://user:pass@localhost:5432/mydb"}`,
			wantValue:  "postgres://user:pass@localhost:5432/mydb",
		},
		{
			name:        "secret not found",
			secretName:  "NONEXISTENT",
			mockOutput:  "Error: secret not found",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:       "empty value",
			secretName: "EMPTY_SECRET",
			mockOutput: `{"name": "EMPTY_SECRET", "value": ""}`,
			wantValue:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()

			if tt.wantErr {
				mockExec.AddErrorResponse("doppler secrets get "+tt.secretName, tt.mockOutput, 1)
			} else {
				mockExec.AddJSONResponse("doppler secrets get "+tt.secretName, tt.mockOutput)
			}

			p := providers.NewDopplerProviderWithExecutor(providers.DopplerConfig{}, mockExec)
			ref := provider.Reference{Key: tt.secretName}

			secret, err := p.Resolve(context.Background(), ref)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValue, secret.Value)
				assert.Equal(t, tt.secretName, secret.Metadata["name"])
			}

			mockExec.AssertCalled(t, "doppler")
		})
	}
}

func TestDopplerProviderWithMockExecutor_Describe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		secretName string
		mockOutput string
		wantErr    bool
	}{
		{
			name:       "existing secret",
			secretName: "API_KEY",
			mockOutput: `{"API_KEY": {"name": "API_KEY", "value": "secret-value"}, "OTHER": {"name": "OTHER", "value": "other-value"}}`,
			wantErr:    false,
		},
		{
			name:       "nonexistent secret",
			secretName: "MISSING",
			mockOutput: `{"API_KEY": {"name": "API_KEY", "value": "secret-value"}}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()
			// When config is set, it uses sh -c for env vars
			mockExec.AddJSONResponse("sh -c", tt.mockOutput)

			config := providers.DopplerConfig{
				Project: "test-project",
				Config:  "development",
			}
			p := providers.NewDopplerProviderWithExecutor(config, mockExec)
			ref := provider.Reference{Key: tt.secretName}

			meta, err := p.Describe(context.Background(), ref)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, meta.Exists)
				assert.Equal(t, "secret", meta.Type)
				assert.Equal(t, "test-project", meta.Tags["project"])
				assert.Equal(t, "development", meta.Tags["config"])
			}
		})
	}
}

func TestDopplerProviderWithMockExecutor_Validate(t *testing.T) {
	t.Parallel()

	// Skip if doppler CLI is not installed
	if _, err := osExec.LookPath("doppler"); err != nil {
		t.Skip("Skipping Validate tests - doppler CLI not installed")
	}

	tests := []struct {
		name        string
		mockOutput  string
		mockErr     bool
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid authentication",
			mockOutput: `{"API_KEY": {"name": "API_KEY", "value": "test"}}`,
			wantErr:    false,
		},
		{
			name:        "authentication failure",
			mockErr:     true,
			wantErr:     true,
			errContains: "Failed to authenticate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()

			if tt.mockErr {
				mockExec.AddErrorResponse("doppler secrets get --json", "Unauthorized", 1)
			} else {
				mockExec.AddJSONResponse("doppler secrets get --json", tt.mockOutput)
			}

			p := providers.NewDopplerProviderWithExecutor(providers.DopplerConfig{}, mockExec)

			err := p.Validate(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDopplerProviderWithMockExecutor_WithToken(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("sh -c", `{"name": "SECRET", "value": "token-auth"}`)

	config := providers.DopplerConfig{
		Token: "dp.st.test_token_12345",
	}
	p := providers.NewDopplerProviderWithExecutor(config, mockExec)

	ref := provider.Reference{Key: "SECRET"}
	_, err := p.Resolve(context.Background(), ref)

	require.NoError(t, err)

	// Verify sh was called (env var wrapping)
	calls := mockExec.GetCalls("sh")
	require.NotEmpty(t, calls)
	if len(calls) > 0 && len(calls[0].Args) >= 2 {
		assert.Contains(t, calls[0].Args[1], "DOPPLER_TOKEN")
	}
}

func TestDopplerProviderWithMockExecutor_WithProjectAndConfig(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("sh -c", `{"name": "DB_PASSWORD", "value": "project-secret"}`)

	config := providers.DopplerConfig{
		Project: "my-project",
		Config:  "production",
	}
	p := providers.NewDopplerProviderWithExecutor(config, mockExec)

	ref := provider.Reference{Key: "DB_PASSWORD"}
	_, err := p.Resolve(context.Background(), ref)

	require.NoError(t, err)

	// Verify environment variables are set
	calls := mockExec.GetCalls("sh")
	require.NotEmpty(t, calls)
	if len(calls) > 0 && len(calls[0].Args) >= 2 {
		assert.Contains(t, calls[0].Args[1], "DOPPLER_PROJECT")
		assert.Contains(t, calls[0].Args[1], "DOPPLER_CONFIG")
	}
}

func TestDopplerProviderWithMockExecutor_FullConfig(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("sh -c", `{"name": "FULL", "value": "full-config-value"}`)

	config := providers.DopplerConfig{
		Token:   "dp.st.full_token",
		Project: "full-project",
		Config:  "staging",
	}
	p := providers.NewDopplerProviderWithExecutor(config, mockExec)

	ref := provider.Reference{Key: "FULL"}
	_, err := p.Resolve(context.Background(), ref)

	require.NoError(t, err)

	// Verify all environment variables are set
	calls := mockExec.GetCalls("sh")
	require.NotEmpty(t, calls)
	if len(calls) > 0 && len(calls[0].Args) >= 2 {
		assert.Contains(t, calls[0].Args[1], "DOPPLER_TOKEN")
		assert.Contains(t, calls[0].Args[1], "DOPPLER_PROJECT")
		assert.Contains(t, calls[0].Args[1], "DOPPLER_CONFIG")
	}
}

func TestDopplerProviderWithMockExecutor_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("doppler secrets get MALFORMED --json", `{invalid json}`)

	p := providers.NewDopplerProviderWithExecutor(providers.DopplerConfig{}, mockExec)
	ref := provider.Reference{Key: "MALFORMED"}

	_, err := p.Resolve(context.Background(), ref)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid response format")
}

func TestDopplerProviderWithMockExecutor_NetworkError(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddErrorResponse("doppler secrets get NETWORK_ERROR --json", "connection refused", 1)

	p := providers.NewDopplerProviderWithExecutor(providers.DopplerConfig{}, mockExec)
	ref := provider.Reference{Key: "NETWORK_ERROR"}

	_, err := p.Resolve(context.Background(), ref)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to retrieve")
}

func TestDopplerProviderConstructors(t *testing.T) {
	t.Parallel()

	t.Run("default constructor", func(t *testing.T) {
		t.Parallel()
		p := providers.NewDopplerProvider(providers.DopplerConfig{})
		assert.NotNil(t, p)
		assert.Equal(t, "doppler", p.Name())
	})

	t.Run("with executor constructor", func(t *testing.T) {
		t.Parallel()
		mockExec := testutil.NewMockCommandExecutor()
		p := providers.NewDopplerProviderWithExecutor(providers.DopplerConfig{}, mockExec)
		assert.NotNil(t, p)
		assert.Equal(t, "doppler", p.Name())
	})

	t.Run("capabilities", func(t *testing.T) {
		t.Parallel()
		p := providers.NewDopplerProvider(providers.DopplerConfig{})
		caps := p.Capabilities()
		assert.True(t, caps.RequiresAuth)
		assert.Contains(t, caps.AuthMethods, "service_token")
	})
}
