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

// TestInfisicalProviderName validates provider name consistency
func TestInfisicalProviderName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerName string
		want         string
	}{
		{
			name:         "default_name",
			providerName: "infisical",
			want:         "infisical",
		},
		{
			name:         "custom_name",
			providerName: "my-infisical",
			want:         "my-infisical",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fakes.NewFakeInfisicalClient()
			p := providers.NewInfisicalProviderWithClient(tt.providerName, nil, fakeClient)
			assert.Equal(t, tt.want, p.Name())
		})
	}
}

// TestInfisicalProviderResolve tests secret resolution
func TestInfisicalProviderResolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFake func(*fakes.FakeInfisicalClient)
		ref       provider.Reference
		want      string
		wantErr   bool
		errMsg    string
	}{
		{
			name: "success_simple_key",
			setupFake: func(f *fakes.FakeInfisicalClient) {
				f.SetSecret("DATABASE_URL", "postgres://localhost/db")
			},
			ref:  provider.Reference{Key: "DATABASE_URL"},
			want: "postgres://localhost/db",
		},
		{
			name: "success_nested_path",
			setupFake: func(f *fakes.FakeInfisicalClient) {
				// The parser extracts just the name, so the secret should be stored by name
				f.SetSecret("DATABASE_URL", "postgres://prod/db")
			},
			ref:  provider.Reference{Key: "production/DATABASE_URL"},
			want: "postgres://prod/db",
		},
		{
			name: "not_found",
			setupFake: func(f *fakes.FakeInfisicalClient) {
				// Don't add any secrets
			},
			ref:     provider.Reference{Key: "NONEXISTENT"},
			wantErr: true,
			errMsg:  "not found",
		},
		{
			name: "auth_failure",
			setupFake: func(f *fakes.FakeInfisicalClient) {
				f.AuthErr = fakes.ErrFakeInfisicalUnauthorized
			},
			ref:     provider.Reference{Key: "SECRET"},
			wantErr: true,
			errMsg:  "unauthorized",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fakes.NewFakeInfisicalClient()
			if tt.setupFake != nil {
				tt.setupFake(fakeClient)
			}

			p := providers.NewInfisicalProviderWithClient("infisical", nil, fakeClient)

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

// TestInfisicalProviderDescribe tests metadata retrieval
func TestInfisicalProviderDescribe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupFake  func(*fakes.FakeInfisicalClient)
		ref        provider.Reference
		wantExists bool
	}{
		{
			name: "exists",
			setupFake: func(f *fakes.FakeInfisicalClient) {
				f.SetSecret("DATABASE_URL", "postgres://localhost/db")
			},
			ref:        provider.Reference{Key: "DATABASE_URL"},
			wantExists: true,
		},
		{
			name: "not_exists",
			setupFake: func(f *fakes.FakeInfisicalClient) {
				// Don't add any secrets
			},
			ref:        provider.Reference{Key: "NONEXISTENT"},
			wantExists: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fakes.NewFakeInfisicalClient()
			if tt.setupFake != nil {
				tt.setupFake(fakeClient)
			}

			p := providers.NewInfisicalProviderWithClient("infisical", nil, fakeClient)

			meta, err := p.Describe(context.Background(), tt.ref)
			require.NoError(t, err)
			assert.Equal(t, tt.wantExists, meta.Exists)
		})
	}
}

// TestInfisicalProviderCapabilities tests capability reporting
func TestInfisicalProviderCapabilities(t *testing.T) {
	t.Parallel()

	fakeClient := fakes.NewFakeInfisicalClient()
	p := providers.NewInfisicalProviderWithClient("infisical", nil, fakeClient)

	caps := p.Capabilities()

	assert.True(t, caps.SupportsVersioning, "infisical supports versioning")
	assert.True(t, caps.SupportsMetadata, "infisical supports metadata")
	assert.False(t, caps.SupportsWatching, "infisical does not support watching")
	assert.True(t, caps.SupportsBinary, "infisical supports binary data")
	assert.True(t, caps.RequiresAuth, "infisical requires auth")
	assert.Contains(t, caps.AuthMethods, "machine_identity")
	assert.Contains(t, caps.AuthMethods, "service_token")
	assert.Contains(t, caps.AuthMethods, "api_key")
}

// TestInfisicalProviderValidate tests provider validation
func TestInfisicalProviderValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFake func(*fakes.FakeInfisicalClient)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid",
			setupFake: func(f *fakes.FakeInfisicalClient) {
				// Default setup is valid
			},
			wantErr: false,
		},
		{
			name: "auth_failure",
			setupFake: func(f *fakes.FakeInfisicalClient) {
				f.AuthErr = fakes.ErrFakeInfisicalUnauthorized
			},
			wantErr: true,
			errMsg:  "unauthorized",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fakes.NewFakeInfisicalClient()
			if tt.setupFake != nil {
				tt.setupFake(fakeClient)
			}

			p := providers.NewInfisicalProviderWithClient("infisical", nil, fakeClient)

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

// TestInfisicalReferenceParser tests reference parsing
func TestInfisicalReferenceParser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		key         string
		wantName    string
		wantPath    string
		wantVersion *int
		wantErr     bool
	}{
		{
			name:     "simple",
			key:      "DATABASE_URL",
			wantName: "DATABASE_URL",
			wantPath: "",
		},
		{
			name:     "with_path",
			key:      "production/DATABASE_URL",
			wantName: "DATABASE_URL",
			wantPath: "production",
		},
		{
			name:        "with_version",
			key:         "DATABASE_URL@v2",
			wantName:    "DATABASE_URL",
			wantPath:    "",
			wantVersion: intPtr(2),
		},
		{
			name:        "with_path_and_version",
			key:         "production/secrets/DATABASE_URL@v3",
			wantName:    "DATABASE_URL",
			wantPath:    "production/secrets",
			wantVersion: intPtr(3),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ref, err := providers.ParseInfisicalReference(tt.key)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantName, ref.Name)
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

// TestInfisicalProviderTokenCaching tests token caching behavior
func TestInfisicalProviderTokenCaching(t *testing.T) {
	t.Parallel()

	fakeClient := fakes.NewFakeInfisicalClient()
	fakeClient.SetSecret("SECRET1", "value1")
	fakeClient.SetSecret("SECRET2", "value2")
	fakeClient.TokenTTL = 30 * time.Second

	p := providers.NewInfisicalProviderWithClient("infisical", nil, fakeClient)

	// First resolve should authenticate
	_, err := p.Resolve(context.Background(), provider.Reference{Key: "SECRET1"})
	require.NoError(t, err)
	assert.Equal(t, 1, fakeClient.AuthCallCount)

	// Second resolve should use cached token
	_, err = p.Resolve(context.Background(), provider.Reference{Key: "SECRET2"})
	require.NoError(t, err)
	assert.Equal(t, 1, fakeClient.AuthCallCount, "should reuse cached token")
}

// TestInfisicalProviderImplementsInterface verifies interface compliance
func TestInfisicalProviderImplementsInterface(t *testing.T) {
	fakeClient := fakes.NewFakeInfisicalClient()
	p := providers.NewInfisicalProviderWithClient("infisical", nil, fakeClient)

	// Compile-time check
	var _ provider.Provider = p
}

func intPtr(v int) *int {
	return &v
}
