package discovery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubernetesProvider_Name(t *testing.T) {
	t.Parallel()

	provider := NewKubernetesProvider()
	assert.Equal(t, "kubernetes", provider.Name())
}

func TestKubernetesProvider_Discover(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "invalid config type",
			config:  "invalid",
			wantErr: true,
			errMsg:  "invalid config type for kubernetes discovery",
		},
		{
			name: "missing selectors",
			config: Config{
				Type:      "kubernetes",
				Selectors: map[string]string{},
			},
			wantErr: true,
			errMsg:  "requires at least one selector",
		},
		{
			name: "nil selectors",
			config: Config{
				Type: "kubernetes",
			},
			wantErr: true,
			errMsg:  "requires at least one selector",
		},
		{
			name: "valid config - not yet implemented",
			config: Config{
				Type:      "kubernetes",
				Selectors: map[string]string{"app": "myapp"},
			},
			wantErr: true,
			errMsg:  "kubernetes discovery not yet implemented",
		},
		{
			name: "multiple selectors - not yet implemented",
			config: Config{
				Type: "kubernetes",
				Selectors: map[string]string{
					"app":     "myapp",
					"version": "v1",
					"tier":    "backend",
				},
			},
			wantErr: true,
			errMsg:  "kubernetes discovery not yet implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := NewKubernetesProvider()
			ctx := context.Background()

			_, err := provider.Discover(ctx, tt.config)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestKubernetesProvider_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "invalid config type",
			config:  "invalid",
			wantErr: true,
			errMsg:  "invalid config type for kubernetes discovery",
		},
		{
			name: "valid kubernetes config",
			config: Config{
				Type:      "kubernetes",
				Selectors: map[string]string{"app": "myapp"},
			},
			wantErr: false,
		},
		{
			name: "empty type is valid",
			config: Config{
				Type:      "",
				Selectors: map[string]string{"app": "myapp"},
			},
			wantErr: false,
		},
		{
			name: "multiple selectors valid",
			config: Config{
				Type: "kubernetes",
				Selectors: map[string]string{
					"app":     "myapp",
					"version": "v1",
					"env":     "prod",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid discovery type",
			config: Config{
				Type:      "cloud",
				Selectors: map[string]string{"app": "myapp"},
			},
			wantErr: true,
			errMsg:  "invalid discovery type for kubernetes provider",
		},
		{
			name: "missing selectors",
			config: Config{
				Type:      "kubernetes",
				Selectors: map[string]string{},
			},
			wantErr: true,
			errMsg:  "requires at least one selector",
		},
		{
			name: "nil selectors",
			config: Config{
				Type: "kubernetes",
			},
			wantErr: true,
			errMsg:  "requires at least one selector",
		},
		{
			name: "empty selector key",
			config: Config{
				Type:      "kubernetes",
				Selectors: map[string]string{"": "value"},
			},
			wantErr: true,
			errMsg:  "selector key cannot be empty",
		},
		{
			name: "empty selector value",
			config: Config{
				Type:      "kubernetes",
				Selectors: map[string]string{"app": ""},
			},
			wantErr: true,
			errMsg:  "selector value cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := NewKubernetesProvider()
			err := provider.Validate(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
