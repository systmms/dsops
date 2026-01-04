package providers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/fakes"
)

// TestAkeylessProviderName validates provider name consistency
func TestAkeylessProviderName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerName string
		want         string
	}{
		{
			name:         "default_name",
			providerName: "akeyless",
			want:         "akeyless",
		},
		{
			name:         "custom_name",
			providerName: "my-akeyless",
			want:         "my-akeyless",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fakes.NewFakeAkeylessClient()
			p := providers.NewAkeylessProviderWithClient(tt.providerName, nil, fakeClient)
			assert.Equal(t, tt.want, p.Name())
		})
	}
}

// TestAkeylessProviderResolve tests secret resolution
func TestAkeylessProviderResolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFake func(*fakes.FakeAkeylessClient)
		ref       provider.Reference
		want      string
		wantErr   bool
		errMsg    string
	}{
		{
			name: "success_simple_path",
			setupFake: func(f *fakes.FakeAkeylessClient) {
				f.SetSecret("/prod/database/password", "supersecret")
			},
			ref:  provider.Reference{Key: "/prod/database/password"},
			want: "supersecret",
		},
		{
			name: "success_without_leading_slash",
			setupFake: func(f *fakes.FakeAkeylessClient) {
				f.SetSecret("/secrets/api-key", "api-key-value")
			},
			ref:  provider.Reference{Key: "secrets/api-key"},
			want: "api-key-value",
		},
		{
			name: "not_found",
			setupFake: func(f *fakes.FakeAkeylessClient) {
				// Don't add any secrets
			},
			ref:     provider.Reference{Key: "/nonexistent/path"},
			wantErr: true,
			errMsg:  "not found",
		},
		{
			name: "auth_failure",
			setupFake: func(f *fakes.FakeAkeylessClient) {
				f.AuthErr = fakes.ErrFakeAkeylessUnauthorized
			},
			ref:     provider.Reference{Key: "/secret"},
			wantErr: true,
			errMsg:  "authentication failed",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fakes.NewFakeAkeylessClient()
			if tt.setupFake != nil {
				tt.setupFake(fakeClient)
			}

			p := providers.NewAkeylessProviderWithClient("akeyless", nil, fakeClient)

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

// TestAkeylessProviderDescribe tests metadata retrieval
func TestAkeylessProviderDescribe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupFake  func(*fakes.FakeAkeylessClient)
		ref        provider.Reference
		wantExists bool
	}{
		{
			name: "exists",
			setupFake: func(f *fakes.FakeAkeylessClient) {
				f.SetSecret("/prod/database/password", "secret")
			},
			ref:        provider.Reference{Key: "/prod/database/password"},
			wantExists: true,
		},
		{
			name: "not_exists",
			setupFake: func(f *fakes.FakeAkeylessClient) {
				// Don't add any secrets
			},
			ref:        provider.Reference{Key: "/nonexistent/path"},
			wantExists: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fakes.NewFakeAkeylessClient()
			if tt.setupFake != nil {
				tt.setupFake(fakeClient)
			}

			p := providers.NewAkeylessProviderWithClient("akeyless", nil, fakeClient)

			meta, err := p.Describe(context.Background(), tt.ref)
			require.NoError(t, err)
			assert.Equal(t, tt.wantExists, meta.Exists)
		})
	}
}

// TestAkeylessProviderCapabilities tests capability reporting
func TestAkeylessProviderCapabilities(t *testing.T) {
	t.Parallel()

	fakeClient := fakes.NewFakeAkeylessClient()
	p := providers.NewAkeylessProviderWithClient("akeyless", nil, fakeClient)

	caps := p.Capabilities()

	assert.True(t, caps.SupportsVersioning, "akeyless supports versioning")
	assert.True(t, caps.SupportsMetadata, "akeyless supports metadata")
	assert.False(t, caps.SupportsWatching, "akeyless does not support watching")
	assert.True(t, caps.SupportsBinary, "akeyless supports binary data")
	assert.True(t, caps.RequiresAuth, "akeyless requires auth")
	assert.Contains(t, caps.AuthMethods, "api_key")
	assert.Contains(t, caps.AuthMethods, "aws_iam")
	assert.Contains(t, caps.AuthMethods, "azure_ad")
	assert.Contains(t, caps.AuthMethods, "gcp")
}

// TestAkeylessProviderValidate tests provider validation
func TestAkeylessProviderValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFake func(*fakes.FakeAkeylessClient)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid",
			setupFake: func(f *fakes.FakeAkeylessClient) {
				// Default setup is valid
			},
			wantErr: false,
		},
		{
			name: "auth_failure",
			setupFake: func(f *fakes.FakeAkeylessClient) {
				f.AuthErr = fakes.ErrFakeAkeylessUnauthorized
			},
			wantErr: true,
			errMsg:  "authentication failed",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fakes.NewFakeAkeylessClient()
			if tt.setupFake != nil {
				tt.setupFake(fakeClient)
			}

			p := providers.NewAkeylessProviderWithClient("akeyless", nil, fakeClient)

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

// TestAkeylessReferenceParser tests reference parsing
func TestAkeylessReferenceParser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		key         string
		wantPath    string
		wantVersion *int
		wantErr     bool
	}{
		{
			name:     "simple_path",
			key:      "/prod/database/password",
			wantPath: "/prod/database/password",
		},
		{
			name:     "path_without_leading_slash",
			key:      "prod/database/password",
			wantPath: "/prod/database/password",
		},
		{
			name:        "with_version",
			key:         "/prod/database/password@v2",
			wantPath:    "/prod/database/password",
			wantVersion: intPtr(2),
		},
		{
			name:     "root_path",
			key:      "/my-secret",
			wantPath: "/my-secret",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ref, err := providers.ParseAkeylessReference(tt.key)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPath, ref.Path)
			if tt.wantVersion != nil {
				require.NotNil(t, ref.Version)
				assert.Equal(t, *tt.wantVersion, *ref.Version)
			} else {
				assert.Nil(t, ref.Version)
			}
		})
	}
}

// TestAkeylessProviderTokenCaching tests token caching behavior
func TestAkeylessProviderTokenCaching(t *testing.T) {
	t.Parallel()

	fakeClient := fakes.NewFakeAkeylessClient()
	fakeClient.SetSecret("/secret1", "value1")
	fakeClient.SetSecret("/secret2", "value2")
	fakeClient.TokenTTL = 30 * time.Second

	p := providers.NewAkeylessProviderWithClient("akeyless", nil, fakeClient)

	// First resolve should authenticate
	_, err := p.Resolve(context.Background(), provider.Reference{Key: "/secret1"})
	require.NoError(t, err)
	assert.Equal(t, 1, fakeClient.AuthCallCount)

	// Second resolve should use cached token
	_, err = p.Resolve(context.Background(), provider.Reference{Key: "/secret2"})
	require.NoError(t, err)
	assert.Equal(t, 1, fakeClient.AuthCallCount, "should reuse cached token")
}

// TestAkeylessProviderImplementsInterface verifies interface compliance
func TestAkeylessProviderImplementsInterface(t *testing.T) {
	fakeClient := fakes.NewFakeAkeylessClient()
	p := providers.NewAkeylessProviderWithClient("akeyless", nil, fakeClient)

	// Compile-time check
	var _ provider.Provider = p
}
