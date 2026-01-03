package discovery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudProvider_Name(t *testing.T) {
	t.Parallel()

	provider := NewCloudProvider()
	assert.Equal(t, "cloud", provider.Name())
}

func TestCloudProvider_Discover(t *testing.T) {
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
			errMsg:  "invalid config type for cloud discovery",
		},
		{
			name: "missing cloud_provider",
			config: Config{
				Selectors: map[string]string{"app": "myapp"},
			},
			wantErr: true,
			errMsg:  "cloud_provider is required",
		},
		{
			name: "missing selectors",
			config: Config{
				CloudProvider: "aws",
			},
			wantErr: true,
			errMsg:  "requires at least one selector",
		},
		{
			name: "unsupported cloud provider",
			config: Config{
				CloudProvider: "digitalocean",
				Selectors:     map[string]string{"app": "myapp"},
			},
			wantErr: true,
			errMsg:  "unsupported cloud provider: digitalocean",
		},
		{
			name: "aws not yet implemented",
			config: Config{
				CloudProvider: "aws",
				Region:        "us-east-1",
				Selectors:     map[string]string{"app": "myapp"},
			},
			wantErr: true,
			errMsg:  "aws cloud discovery not yet implemented",
		},
		{
			name: "gcp not yet implemented",
			config: Config{
				CloudProvider: "gcp",
				Selectors:     map[string]string{"app": "myapp"},
			},
			wantErr: true,
			errMsg:  "gcp cloud discovery not yet implemented",
		},
		{
			name: "azure not yet implemented",
			config: Config{
				CloudProvider: "azure",
				Region:        "eastus",
				Selectors:     map[string]string{"app": "myapp"},
			},
			wantErr: true,
			errMsg:  "azure cloud discovery not yet implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := NewCloudProvider()
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

func TestCloudProvider_Validate(t *testing.T) {
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
			errMsg:  "invalid config type for cloud discovery",
		},
		{
			name: "valid aws config",
			config: Config{
				Type:          "cloud",
				CloudProvider: "aws",
				Region:        "us-east-1",
				Selectors:     map[string]string{"app": "myapp"},
			},
			wantErr: false,
		},
		{
			name: "valid gcp config (no region required)",
			config: Config{
				Type:          "cloud",
				CloudProvider: "gcp",
				Selectors:     map[string]string{"app": "myapp"},
			},
			wantErr: false,
		},
		{
			name: "valid azure config",
			config: Config{
				Type:          "cloud",
				CloudProvider: "azure",
				Region:        "eastus",
				Selectors:     map[string]string{"app": "myapp"},
			},
			wantErr: false,
		},
		{
			name: "empty type is valid",
			config: Config{
				Type:          "",
				CloudProvider: "aws",
				Region:        "us-west-2",
				Selectors:     map[string]string{"env": "prod"},
			},
			wantErr: false,
		},
		{
			name: "invalid discovery type",
			config: Config{
				Type:          "kubernetes",
				CloudProvider: "aws",
				Region:        "us-east-1",
				Selectors:     map[string]string{"app": "myapp"},
			},
			wantErr: true,
			errMsg:  "invalid discovery type for cloud provider",
		},
		{
			name: "missing cloud_provider",
			config: Config{
				Type:      "cloud",
				Selectors: map[string]string{"app": "myapp"},
			},
			wantErr: true,
			errMsg:  "cloud_provider is required",
		},
		{
			name: "unsupported cloud provider",
			config: Config{
				Type:          "cloud",
				CloudProvider: "linode",
				Selectors:     map[string]string{"app": "myapp"},
			},
			wantErr: true,
			errMsg:  "unsupported cloud provider: linode",
		},
		{
			name: "missing selectors",
			config: Config{
				Type:          "cloud",
				CloudProvider: "aws",
				Region:        "us-east-1",
				Selectors:     map[string]string{},
			},
			wantErr: true,
			errMsg:  "requires at least one selector",
		},
		{
			name: "empty selector key",
			config: Config{
				Type:          "cloud",
				CloudProvider: "aws",
				Region:        "us-east-1",
				Selectors:     map[string]string{"": "value"},
			},
			wantErr: true,
			errMsg:  "selector key cannot be empty",
		},
		{
			name: "empty selector value",
			config: Config{
				Type:          "cloud",
				CloudProvider: "aws",
				Region:        "us-east-1",
				Selectors:     map[string]string{"app": ""},
			},
			wantErr: true,
			errMsg:  "selector value cannot be empty",
		},
		{
			name: "aws missing region",
			config: Config{
				Type:          "cloud",
				CloudProvider: "aws",
				Selectors:     map[string]string{"app": "myapp"},
			},
			wantErr: true,
			errMsg:  "region is required for aws",
		},
		{
			name: "azure missing region",
			config: Config{
				Type:          "cloud",
				CloudProvider: "azure",
				Selectors:     map[string]string{"app": "myapp"},
			},
			wantErr: true,
			errMsg:  "region is required for azure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := NewCloudProvider()
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
