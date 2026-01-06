package providers_test

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/fakes"
)

// TestKeychainProviderName validates provider name consistency
func TestKeychainProviderName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerName string
		want         string
	}{
		{
			name:         "default_name",
			providerName: "keychain",
			want:         "keychain",
		},
		{
			name:         "custom_name",
			providerName: "my-keychain",
			want:         "my-keychain",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fakes.NewFakeKeychainClient()
			p := providers.NewKeychainProviderWithClient(tt.providerName, nil, fakeClient)
			assert.Equal(t, tt.want, p.Name())
		})
	}
}

// TestKeychainProviderResolve tests secret resolution
func TestKeychainProviderResolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFake func(*fakes.FakeKeychainClient)
		ref       provider.Reference
		want      string
		wantErr   bool
		errMsg    string
	}{
		{
			name: "success_simple_key",
			setupFake: func(f *fakes.FakeKeychainClient) {
				f.SetSecret("myapp", "api-key", []byte("secret123"))
			},
			ref:  provider.Reference{Key: "myapp/api-key"},
			want: "secret123",
		},
		{
			name: "success_with_service_prefix",
			setupFake: func(f *fakes.FakeKeychainClient) {
				f.SetSecret("com.company.myapp", "password", []byte("secretpass"))
			},
			ref:  provider.Reference{Key: "com.company.myapp/password"},
			want: "secretpass",
		},
		{
			name: "not_found",
			setupFake: func(f *fakes.FakeKeychainClient) {
				// Don't add any secrets
			},
			ref:     provider.Reference{Key: "nonexistent/secret"},
			wantErr: true,
			errMsg:  "not found",
		},
		{
			name: "invalid_reference_no_separator",
			setupFake: func(f *fakes.FakeKeychainClient) {
				f.SetSecret("myapp", "key", []byte("value"))
			},
			ref:     provider.Reference{Key: "invalid-key"},
			wantErr: true,
			errMsg:  "must be service/account",
		},
		{
			name: "access_denied",
			setupFake: func(f *fakes.FakeKeychainClient) {
				f.QueryErr = fakes.ErrFakeKeychainAccessDenied
			},
			ref:     provider.Reference{Key: "myapp/secret"},
			wantErr: true,
			errMsg:  "access denied",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fakes.NewFakeKeychainClient()
			if tt.setupFake != nil {
				tt.setupFake(fakeClient)
			}

			p := providers.NewKeychainProviderWithClient("keychain", nil, fakeClient)

			result, err := p.Resolve(context.Background(), tt.ref)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, result.Value)
		})
	}
}

// TestKeychainProviderDescribe tests metadata retrieval
func TestKeychainProviderDescribe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupFake  func(*fakes.FakeKeychainClient)
		ref        provider.Reference
		wantExists bool
	}{
		{
			name: "exists",
			setupFake: func(f *fakes.FakeKeychainClient) {
				f.SetSecret("myapp", "api-key", []byte("secret123"))
			},
			ref:        provider.Reference{Key: "myapp/api-key"},
			wantExists: true,
		},
		{
			name: "not_exists",
			setupFake: func(f *fakes.FakeKeychainClient) {
				// Don't add any secrets
			},
			ref:        provider.Reference{Key: "nonexistent/secret"},
			wantExists: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fakes.NewFakeKeychainClient()
			if tt.setupFake != nil {
				tt.setupFake(fakeClient)
			}

			p := providers.NewKeychainProviderWithClient("keychain", nil, fakeClient)

			meta, err := p.Describe(context.Background(), tt.ref)
			require.NoError(t, err)
			assert.Equal(t, tt.wantExists, meta.Exists)
		})
	}
}

// TestKeychainProviderCapabilities tests capability reporting
func TestKeychainProviderCapabilities(t *testing.T) {
	t.Parallel()

	fakeClient := fakes.NewFakeKeychainClient()
	p := providers.NewKeychainProviderWithClient("keychain", nil, fakeClient)

	caps := p.Capabilities()

	assert.False(t, caps.SupportsVersioning, "keychain does not support versioning")
	assert.True(t, caps.SupportsMetadata, "keychain supports metadata")
	assert.False(t, caps.SupportsWatching, "keychain does not support watching")
	assert.True(t, caps.SupportsBinary, "keychain supports binary data")
	assert.False(t, caps.RequiresAuth, "keychain uses OS-level auth")
	assert.Contains(t, caps.AuthMethods, "os")
}

// TestKeychainProviderValidate tests provider validation
func TestKeychainProviderValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFake func(*fakes.FakeKeychainClient)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid",
			setupFake: func(f *fakes.FakeKeychainClient) {
				f.Available = true
				f.Headless = false
			},
			wantErr: false,
		},
		{
			name: "not_available",
			setupFake: func(f *fakes.FakeKeychainClient) {
				f.Available = false
			},
			wantErr: true,
			errMsg:  "not supported on this platform",
		},
		{
			name: "headless_environment",
			setupFake: func(f *fakes.FakeKeychainClient) {
				f.Available = true
				f.Headless = true
			},
			wantErr: true,
			errMsg:  "headless",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fakes.NewFakeKeychainClient()
			if tt.setupFake != nil {
				tt.setupFake(fakeClient)
			}

			p := providers.NewKeychainProviderWithClient("keychain", nil, fakeClient)

			err := p.Validate(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
		})
	}
}

// TestKeychainReferenceParser tests reference parsing
func TestKeychainReferenceParser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		key         string
		wantService string
		wantAccount string
		wantErr     bool
	}{
		{
			name:        "simple",
			key:         "myapp/api-key",
			wantService: "myapp",
			wantAccount: "api-key",
		},
		{
			name:        "with_dots",
			key:         "com.company.app/password",
			wantService: "com.company.app",
			wantAccount: "password",
		},
		{
			name:        "complex_account",
			key:         "service/user@example.com",
			wantService: "service",
			wantAccount: "user@example.com",
		},
		{
			name:    "no_separator",
			key:     "invalid",
			wantErr: true,
		},
		{
			name:    "empty_service",
			key:     "/account",
			wantErr: true,
		},
		{
			name:    "empty_account",
			key:     "service/",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ref, err := providers.ParseKeychainReference(tt.key)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantService, ref.Service)
			assert.Equal(t, tt.wantAccount, ref.Account)
		})
	}
}

// TestKeychainProviderServicePrefix tests service prefix configuration
func TestKeychainProviderServicePrefix(t *testing.T) {
	t.Parallel()

	fakeClient := fakes.NewFakeKeychainClient()
	fakeClient.SetSecret("com.mycompany.myapp", "api-key", []byte("secret123"))

	config := map[string]interface{}{
		"service_prefix": "com.mycompany",
	}
	p := providers.NewKeychainProviderWithClient("keychain", config, fakeClient)

	// Reference without prefix - should be combined with config prefix
	result, err := p.Resolve(context.Background(), provider.Reference{Key: "myapp/api-key"})
	require.NoError(t, err)
	assert.Equal(t, "secret123", result.Value)
}

// TestKeychainProviderPlatformCheck validates platform detection
func TestKeychainProviderPlatformCheck(t *testing.T) {
	fakeClient := fakes.NewFakeKeychainClient()
	p := providers.NewKeychainProviderWithClient("keychain", nil, fakeClient)

	platform := p.Platform()
	// Should return the actual platform
	assert.Equal(t, runtime.GOOS, platform)
}

// TestKeychainProviderImplementsInterface verifies interface compliance
func TestKeychainProviderImplementsInterface(t *testing.T) {
	fakeClient := fakes.NewFakeKeychainClient()
	p := providers.NewKeychainProviderWithClient("keychain", nil, fakeClient)

	// Compile-time check
	var _ provider.Provider = p
}
