package providers_test

import (
	"context"
	"fmt"
	osExec "os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

func TestPassProviderWithMockExecutor_Resolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		secretPath  string
		mockOutput  string
		wantValue   string
		wantMetaKey string
		wantErr     bool
		errContains string
	}{
		{
			name:       "simple password",
			secretPath: "mypassword",
			mockOutput: "secretpassword123\n",
			wantValue:  "secretpassword123",
		},
		{
			name:        "password with metadata",
			secretPath:  "email/gmail",
			mockOutput:  "mypassword\nuser: testuser@gmail.com\nurl: https://gmail.com\n",
			wantValue:   "mypassword",
			wantMetaKey: "additional_data",
		},
		{
			name:       "hierarchical path",
			secretPath: "work/client/api-key",
			mockOutput: "sk-live-1234567890abcdef\n",
			wantValue:  "sk-live-1234567890abcdef",
		},
		{
			name:        "secret not found",
			secretPath:  "nonexistent/secret",
			mockOutput:  "Error: nonexistent/secret is not in the password store.\n",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:       "password with special characters",
			secretPath: "special",
			mockOutput: "p@$$w0rd!#$%^&*()\n",
			wantValue:  "p@$$w0rd!#$%^&*()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()

			// Configure mock response
			if tt.wantErr && tt.errContains != "" {
				mockExec.AddErrorResponse("pass show "+tt.secretPath, tt.mockOutput, 1)
			} else {
				mockExec.AddJSONResponse("pass show "+tt.secretPath, tt.mockOutput)
			}

			p := providers.NewPassProviderWithExecutor(providers.PassConfig{}, mockExec)
			ref := provider.Reference{Key: tt.secretPath}

			secret, err := p.Resolve(context.Background(), ref)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValue, secret.Value)
				assert.Equal(t, tt.secretPath, secret.Metadata["path"])

				if tt.wantMetaKey != "" {
					assert.NotEmpty(t, secret.Metadata[tt.wantMetaKey])
				}
			}

			// Verify the command was called
			mockExec.AssertCalled(t, "pass")
		})
	}
}

func TestPassProviderWithMockExecutor_Describe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		secretPath string
		mockOutput string
		wantType   string
		wantTags   map[string]string
		wantErr    bool
	}{
		{
			name:       "simple password",
			secretPath: "simple",
			mockOutput: "password123\n",
			wantType:   "password",
			wantTags:   map[string]string{"path": "simple"},
		},
		{
			name:       "password with metadata",
			secretPath: "email/work",
			mockOutput: "workpass\nuser: work@company.com\nrecovery: 123456\n",
			wantType:   "password_with_metadata",
			wantTags:   map[string]string{"path": "email/work", "folder": "email"},
		},
		{
			name:       "deep path",
			secretPath: "category/subcategory/secret",
			mockOutput: "secret\n",
			wantType:   "password",
			wantTags:   map[string]string{"path": "category/subcategory/secret", "folder": "category/subcategory"},
		},
		{
			name:       "nonexistent secret",
			secretPath: "missing",
			mockOutput: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()

			if tt.wantErr {
				mockExec.AddErrorResponse("pass show "+tt.secretPath, "Error: not in the password store", 1)
			} else {
				mockExec.AddJSONResponse("pass show "+tt.secretPath, tt.mockOutput)
			}

			p := providers.NewPassProviderWithExecutor(providers.PassConfig{}, mockExec)
			ref := provider.Reference{Key: tt.secretPath}

			meta, err := p.Describe(context.Background(), ref)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, meta.Exists)
				assert.Equal(t, tt.wantType, meta.Type)
				for k, v := range tt.wantTags {
					assert.Equal(t, v, meta.Tags[k])
				}
			}
		})
	}
}

func TestPassProviderWithMockExecutor_Validate(t *testing.T) {
	t.Parallel()

	// Skip if pass CLI is not installed since Validate() checks exec.LookPath first
	if _, err := osExec.LookPath("pass"); err != nil {
		t.Skip("Skipping Validate tests - pass CLI not installed")
	}

	tests := []struct {
		name        string
		mockOutput  string
		mockErr     error
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid pass setup",
			mockOutput: "Password Store\n├── email\n│   └── gmail\n└── work\n",
			wantErr:    false,
		},
		{
			name:        "pass store not initialized",
			mockErr:     fmt.Errorf("exit status 1"),
			wantErr:     true,
			errContains: "Failed to access",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()

			if tt.mockErr != nil {
				mockExec.AddResponse("pass list", testutil.MockResponse{
					Stderr: []byte("Error"),
					Err:    tt.mockErr,
				})
			} else {
				mockExec.AddJSONResponse("pass list", tt.mockOutput)
			}

			p := providers.NewPassProviderWithExecutor(providers.PassConfig{}, mockExec)

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

func TestPassProviderWithMockExecutor_CustomPasswordStore(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockCommandExecutor()

	// The command will be wrapped in sh -c with env vars
	mockExec.AddJSONResponse("sh -c", "testpassword\n")

	config := providers.PassConfig{
		PasswordStore: "/custom/store/path",
	}
	p := providers.NewPassProviderWithExecutor(config, mockExec)

	ref := provider.Reference{Key: "test"}
	_, err := p.Resolve(context.Background(), ref)

	require.NoError(t, err)

	// Verify sh was called (indicating env var wrapping)
	calls := mockExec.GetCalls("sh")
	require.NotEmpty(t, calls)

	// Check that the command includes PASSWORD_STORE_DIR
	if len(calls) > 0 && len(calls[0].Args) >= 2 {
		assert.Contains(t, calls[0].Args[1], "PASSWORD_STORE_DIR")
	}
}

func TestPassProviderWithMockExecutor_CustomGPGKey(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("sh -c", "gpgpassword\n")

	config := providers.PassConfig{
		GpgKey: "custom@gpg.key",
	}
	p := providers.NewPassProviderWithExecutor(config, mockExec)

	ref := provider.Reference{Key: "gpgtest"}
	_, err := p.Resolve(context.Background(), ref)

	require.NoError(t, err)

	// Verify sh was called (indicating env var wrapping)
	calls := mockExec.GetCalls("sh")
	require.NotEmpty(t, calls)

	// Check that the command includes PASSWORD_STORE_KEY
	if len(calls) > 0 && len(calls[0].Args) >= 2 {
		assert.Contains(t, calls[0].Args[1], "PASSWORD_STORE_KEY")
	}
}

func TestPassProviderWithMockExecutor_CombinedConfig(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("sh -c", "combinedtest\n")

	config := providers.PassConfig{
		PasswordStore: "/custom/path",
		GpgKey:        "both@options.set",
	}
	p := providers.NewPassProviderWithExecutor(config, mockExec)

	ref := provider.Reference{Key: "both"}
	_, err := p.Resolve(context.Background(), ref)

	require.NoError(t, err)

	// Verify both environment variables are set
	calls := mockExec.GetCalls("sh")
	require.NotEmpty(t, calls)
	if len(calls) > 0 && len(calls[0].Args) >= 2 {
		assert.Contains(t, calls[0].Args[1], "PASSWORD_STORE_DIR")
		assert.Contains(t, calls[0].Args[1], "PASSWORD_STORE_KEY")
	}
}

func TestPassProviderWithMockExecutor_ErrorHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		secretPath  string
		errOutput   string
		errContains string
	}{
		{
			name:        "GPG decryption failure",
			secretPath:  "encrypted",
			errOutput:   "gpg: decryption failed: No secret key",
			errContains: "Failed to retrieve",
		},
		{
			name:        "permission denied",
			secretPath:  "protected",
			errOutput:   "Permission denied",
			errContains: "Failed to retrieve",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()
			mockExec.AddErrorResponse("pass show "+tt.secretPath, tt.errOutput, 1)

			p := providers.NewPassProviderWithExecutor(providers.PassConfig{}, mockExec)
			ref := provider.Reference{Key: tt.secretPath}

			_, err := p.Resolve(context.Background(), ref)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestPassProviderWithMockExecutor_ContextCancellation(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.StrictMode = true
	mockExec.AddErrorResponse("pass show timeout", "context canceled", 1)

	p := providers.NewPassProviderWithExecutor(providers.PassConfig{}, mockExec)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	ref := provider.Reference{Key: "timeout"}
	_, err := p.Resolve(ctx, ref)

	// The mock doesn't actually respect context, but we verify the executor is called
	require.Error(t, err)
}

func TestPassProviderConstructors(t *testing.T) {
	t.Parallel()

	t.Run("default constructor", func(t *testing.T) {
		t.Parallel()
		p := providers.NewPassProvider(providers.PassConfig{})
		assert.NotNil(t, p)
		assert.Equal(t, "pass", p.Name())
	})

	t.Run("with executor constructor", func(t *testing.T) {
		t.Parallel()
		mockExec := testutil.NewMockCommandExecutor()
		p := providers.NewPassProviderWithExecutor(providers.PassConfig{}, mockExec)
		assert.NotNil(t, p)
		assert.Equal(t, "pass", p.Name())
	})

	t.Run("with full config", func(t *testing.T) {
		t.Parallel()
		config := providers.PassConfig{
			PasswordStore: "/custom/store",
			GpgKey:        "key@example.com",
		}
		p := providers.NewPassProvider(config)
		assert.NotNil(t, p)
		assert.Equal(t, "pass", p.Name())
	})
}
