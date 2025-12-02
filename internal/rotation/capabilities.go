package rotation

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed capabilities.yaml
var capabilitiesYAML string

// ProviderCapability represents the rotation capabilities of a provider
type ProviderCapability struct {
	DisplayName           string   `yaml:"display_name"`
	MaxActiveKeys         string   `yaml:"max_active_keys"` // Can be number or "unlimited"
	SupportsExpiration    bool     `yaml:"supports_expiration"`
	SupportsVersioning    bool     `yaml:"supports_versioning"`
	SupportsRevocation    bool     `yaml:"supports_revocation"`
	RotationAPI           bool     `yaml:"rotation_api"`
	RecommendedStrategies []string `yaml:"recommended_strategies"`
	VerifiedDate          string   `yaml:"verified_date"`
	Documentation         string   `yaml:"documentation"`
	Notes                 string   `yaml:"notes"`
}

// StrategyDefinition describes a rotation strategy
type StrategyDefinition struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	DowntimeRisk string   `yaml:"downtime_risk"`
	Complexity   string   `yaml:"complexity"`
	Requirements []string `yaml:"requirements"`
}

// CapabilitiesRegistry holds all provider capabilities and strategy definitions
type CapabilitiesRegistry struct {
	Providers  map[string]ProviderCapability `yaml:"providers"`
	Strategies map[string]StrategyDefinition `yaml:"strategies"`
}

var (
	registry   *CapabilitiesRegistry
	registryMu sync.RWMutex
)

// LoadCapabilities loads the embedded capabilities YAML
func LoadCapabilities() (*CapabilitiesRegistry, error) {
	registryMu.RLock()
	if registry != nil {
		defer registryMu.RUnlock()
		return registry, nil
	}
	registryMu.RUnlock()

	registryMu.Lock()
	defer registryMu.Unlock()

	// Double-check after acquiring write lock
	if registry != nil {
		return registry, nil
	}

	var reg CapabilitiesRegistry
	if err := yaml.Unmarshal([]byte(capabilitiesYAML), &reg); err != nil {
		return nil, fmt.Errorf("failed to parse capabilities: %w", err)
	}

	registry = &reg
	return registry, nil
}

// GetProviderCapability returns the capability for a specific provider
func GetProviderCapability(provider string) (*ProviderCapability, error) {
	reg, err := LoadCapabilities()
	if err != nil {
		return nil, err
	}

	// Normalize provider name
	provider = strings.ToLower(strings.TrimSpace(provider))

	cap, ok := reg.Providers[provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	return &cap, nil
}

// GetRecommendedStrategy returns the best strategy for a provider
func GetRecommendedStrategy(provider string) (string, error) {
	cap, err := GetProviderCapability(provider)
	if err != nil {
		// Default to immediate if provider unknown
		return "immediate", nil
	}

	if len(cap.RecommendedStrategies) > 0 {
		return cap.RecommendedStrategies[0], nil
	}

	// Default fallback
	return "immediate", nil
}

// SupportsStrategy checks if a provider supports a specific strategy
func SupportsStrategy(provider, strategy string) bool {
	cap, err := GetProviderCapability(provider)
	if err != nil {
		// Unknown provider - only immediate is safe
		return strategy == "immediate"
	}

	for _, s := range cap.RecommendedStrategies {
		if s == strategy {
			return true
		}
	}

	return false
}

// GetMaxActiveKeys returns the maximum number of active keys
// Returns -1 for unlimited, 0 for unknown
func GetMaxActiveKeys(provider string) int {
	cap, err := GetProviderCapability(provider)
	if err != nil {
		return 0
	}

	switch cap.MaxActiveKeys {
	case "unlimited":
		return -1
	case "1":
		return 1
	case "2":
		return 2
	case "50":
		return 50
	default:
		// Try to parse as number
		var n int
		if _, err := fmt.Sscanf(cap.MaxActiveKeys, "%d", &n); err == nil {
			return n
		}
		return 0
	}
}

// ListProviders returns all known providers
func ListProviders() []string {
	reg, err := LoadCapabilities()
	if err != nil {
		return []string{}
	}

	providers := make([]string, 0, len(reg.Providers))
	for name := range reg.Providers {
		providers = append(providers, name)
	}

	return providers
}

// GetStrategyDefinition returns information about a strategy
func GetStrategyDefinition(strategy string) (*StrategyDefinition, error) {
	reg, err := LoadCapabilities()
	if err != nil {
		return nil, err
	}

	def, ok := reg.Strategies[strategy]
	if !ok {
		return nil, fmt.Errorf("unknown strategy: %s", strategy)
	}

	return &def, nil
}

// ValidateProviderStrategy checks if a provider can use a specific strategy
func ValidateProviderStrategy(provider, strategy string) error {
	cap, err := GetProviderCapability(provider)
	if err != nil {
		return fmt.Errorf("unknown provider %s: %w", provider, err)
	}

	stratDef, err := GetStrategyDefinition(strategy)
	if err != nil {
		return fmt.Errorf("unknown strategy %s: %w", strategy, err)
	}

	// Check if strategy is recommended
	if !SupportsStrategy(provider, strategy) {
		return fmt.Errorf("strategy %s is not recommended for provider %s. Recommended: %v",
			strategy, provider, cap.RecommendedStrategies)
	}

	// Validate specific requirements
	switch strategy {
	case "two-key":
		maxKeys := GetMaxActiveKeys(provider)
		if maxKeys != -1 && maxKeys < 2 {
			return fmt.Errorf("two-key strategy requires at least 2 active keys, but %s only supports %d",
				provider, maxKeys)
		}
	case "overlap":
		if !cap.SupportsExpiration {
			return fmt.Errorf("overlap strategy requires expiration support, which %s lacks", provider)
		}
	case "versioned":
		if !cap.SupportsVersioning {
			return fmt.Errorf("versioned strategy requires versioning support, which %s lacks", provider)
		}
	}

	// Log if there's downtime risk
	if stratDef.DowntimeRisk == "high" {
		// This would be logged as a warning in real implementation
		return fmt.Errorf("warning: %s strategy has high downtime risk with %s", strategy, provider)
	}

	return nil
}