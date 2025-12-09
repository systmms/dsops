package discovery

import (
	"context"
	"fmt"

	"github.com/systmms/dsops/internal/rotation/gradual"
)

// ExplicitProvider discovers instances from explicit configuration.
type ExplicitProvider struct{}

// NewExplicitProvider creates a new explicit discovery provider.
func NewExplicitProvider() *ExplicitProvider {
	return &ExplicitProvider{}
}

// Name returns the provider name.
func (p *ExplicitProvider) Name() string {
	return "explicit"
}

// Discover returns the list of instances from the configuration.
func (p *ExplicitProvider) Discover(ctx context.Context, configIface interface{}) ([]gradual.Instance, error) {
	config, ok := configIface.(Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type for explicit discovery: expected Config, got %T", configIface)
	}

	if len(config.Instances) == 0 {
		return nil, fmt.Errorf("explicit discovery requires at least one instance in configuration")
	}

	instances := make([]gradual.Instance, 0, len(config.Instances))
	for _, inst := range config.Instances {
		if inst.ID == "" {
			return nil, fmt.Errorf("instance ID is required for explicit discovery")
		}

		instances = append(instances, gradual.Instance{
			ID:       inst.ID,
			Labels:   inst.Labels,
			Endpoint: inst.Endpoint,
		})
	}

	return instances, nil
}

// Validate checks if the configuration is valid for explicit discovery.
func (p *ExplicitProvider) Validate(configIface interface{}) error {
	config, ok := configIface.(Config)
	if !ok {
		return fmt.Errorf("invalid config type for explicit discovery: expected Config, got %T", configIface)
	}
	if config.Type != "explicit" && config.Type != "" {
		return fmt.Errorf("invalid discovery type for explicit provider: %s", config.Type)
	}

	if len(config.Instances) == 0 {
		return fmt.Errorf("explicit discovery requires at least one instance")
	}

	// Validate each instance
	seenIDs := make(map[string]bool)
	for i, inst := range config.Instances {
		if inst.ID == "" {
			return fmt.Errorf("instance[%d]: ID is required", i)
		}

		// Check for duplicate IDs
		if seenIDs[inst.ID] {
			return fmt.Errorf("instance[%d]: duplicate ID '%s'", i, inst.ID)
		}
		seenIDs[inst.ID] = true
	}

	return nil
}
