package discovery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExplicitProvider_Name(t *testing.T) {
	t.Parallel()

	provider := NewExplicitProvider()
	assert.Equal(t, "explicit", provider.Name())
}

func TestExplicitProvider_Discover(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    Config
		wantCount int
		wantErr   bool
		errMsg    string
	}{
		{
			name: "single instance",
			config: Config{
				Instances: []InstanceConfig{
					{
						ID:       "instance-1",
						Labels:   map[string]string{"env": "prod"},
						Endpoint: "https://api1.example.com",
					},
				},
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "multiple instances",
			config: Config{
				Instances: []InstanceConfig{
					{ID: "instance-1", Labels: map[string]string{"env": "prod"}},
					{ID: "instance-2", Labels: map[string]string{"env": "prod"}},
					{ID: "instance-3", Labels: map[string]string{"env": "prod", "canary": "true"}},
				},
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name: "empty instances",
			config: Config{
				Instances: []InstanceConfig{},
			},
			wantErr: true,
			errMsg:  "explicit discovery requires at least one instance",
		},
		{
			name: "instance with no ID",
			config: Config{
				Instances: []InstanceConfig{
					{ID: ""},
				},
			},
			wantErr: true,
			errMsg:  "instance ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := NewExplicitProvider()
			ctx := context.Background()

			instances, err := provider.Discover(ctx, tt.config)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Len(t, instances, tt.wantCount)

			// Verify instance data
			if tt.wantCount > 0 {
				for i, inst := range instances {
					assert.NotEmpty(t, inst.ID, "instance[%d] should have ID", i)
					assert.Equal(t, tt.config.Instances[i].ID, inst.ID)
					assert.Equal(t, tt.config.Instances[i].Labels, inst.Labels)
					assert.Equal(t, tt.config.Instances[i].Endpoint, inst.Endpoint)
				}
			}
		})
	}
}

func TestExplicitProvider_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				Type: "explicit",
				Instances: []InstanceConfig{
					{ID: "instance-1"},
					{ID: "instance-2"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty type is valid (defaults to explicit)",
			config: Config{
				Type: "",
				Instances: []InstanceConfig{
					{ID: "instance-1"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid type",
			config: Config{
				Type: "kubernetes",
				Instances: []InstanceConfig{
					{ID: "instance-1"},
				},
			},
			wantErr: true,
			errMsg:  "invalid discovery type",
		},
		{
			name: "no instances",
			config: Config{
				Type:      "explicit",
				Instances: []InstanceConfig{},
			},
			wantErr: true,
			errMsg:  "explicit discovery requires at least one instance",
		},
		{
			name: "missing instance ID",
			config: Config{
				Type: "explicit",
				Instances: []InstanceConfig{
					{ID: "instance-1"},
					{ID: ""},
				},
			},
			wantErr: true,
			errMsg:  "ID is required",
		},
		{
			name: "duplicate instance IDs",
			config: Config{
				Type: "explicit",
				Instances: []InstanceConfig{
					{ID: "instance-1"},
					{ID: "instance-1"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := NewExplicitProvider()
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
