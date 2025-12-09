// Package discovery provides instance discovery for gradual rollout.
package discovery

import (
	"context"

	"github.com/systmms/dsops/internal/rotation/gradual"
)

// Provider defines the interface for discovering service instances.
// It implements gradual.DiscoveryProvider to avoid import cycles.
type Provider interface {
	// Name returns the provider name (e.g., "explicit", "kubernetes", "cloud", "endpoint").
	Name() string

	// Discover returns the list of instances for the given configuration.
	// The config parameter should be a Config struct but is interface{} to match the parent interface.
	Discover(ctx context.Context, config interface{}) ([]gradual.Instance, error)

	// Validate checks if the configuration is valid for this provider.
	// The config parameter should be a Config struct but is interface{} to match the parent interface.
	Validate(config interface{}) error
}

// Config holds discovery configuration.
type Config struct {
	// Type is the discovery provider type.
	Type string `yaml:"type"`

	// Instances lists explicit instances (for explicit discovery).
	Instances []InstanceConfig `yaml:"instances,omitempty"`

	// Selectors are key-value labels for filtering (for kubernetes/cloud).
	Selectors map[string]string `yaml:"selectors,omitempty"`

	// Endpoint is the HTTP endpoint URL (for endpoint discovery).
	Endpoint string `yaml:"endpoint,omitempty"`

	// CloudProvider specifies the cloud provider (for cloud discovery).
	// Valid values: "aws", "gcp", "azure".
	CloudProvider string `yaml:"cloud_provider,omitempty"`

	// Region specifies the cloud region (for cloud discovery).
	Region string `yaml:"region,omitempty"`
}

// InstanceConfig holds explicit instance configuration.
type InstanceConfig struct {
	// ID is the unique instance identifier.
	ID string `yaml:"id"`

	// Labels are key-value labels for instance selection.
	Labels map[string]string `yaml:"labels,omitempty"`

	// Endpoint is the instance endpoint.
	Endpoint string `yaml:"endpoint,omitempty"`

	// Canary indicates if this instance should be used as canary.
	Canary bool `yaml:"canary,omitempty"`
}
