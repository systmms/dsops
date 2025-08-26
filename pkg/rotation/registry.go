package rotation

import (
	"fmt"

	"github.com/systmms/dsops/internal/logging"
)

// StrategyRegistry manages available rotation strategies
type StrategyRegistry struct {
	strategies map[string]func(*logging.Logger) SecretValueRotator
	logger     *logging.Logger
}

// NewStrategyRegistry creates a new strategy registry
func NewStrategyRegistry(logger *logging.Logger) *StrategyRegistry {
	registry := &StrategyRegistry{
		strategies: make(map[string]func(*logging.Logger) SecretValueRotator),
		logger:     logger,
	}

	// Register built-in strategies
	registry.registerBuiltinStrategies()
	
	return registry
}

// registerBuiltinStrategies registers all built-in rotation strategies
func (r *StrategyRegistry) registerBuiltinStrategies() {
	// Random strategy for testing
	r.strategies["random"] = func(logger *logging.Logger) SecretValueRotator {
		return NewRandomRotator(logger)
	}
	
	// Webhook strategy for external rotation
	r.strategies["webhook"] = func(logger *logging.Logger) SecretValueRotator {
		return NewWebhookRotator(logger)
	}
	
	// Custom script strategy
	r.strategies["script"] = func(logger *logging.Logger) SecretValueRotator {
		return NewScriptRotator(logger)
	}

	// NOTE: Database and service-specific rotation strategies are now
	// implemented in the dsops-data repository as data-driven configurations.
	// Only generic, reusable strategies (random, webhook, script) are 
	// implemented here in the core codebase.

	r.logger.Debug("Registered %d built-in rotation strategies", len(r.strategies))
}

// CreateStrategy creates a new instance of the specified strategy
func (r *StrategyRegistry) CreateStrategy(name string) (SecretValueRotator, error) {
	factory, exists := r.strategies[name]
	if !exists {
		return nil, fmt.Errorf("unknown rotation strategy: %s", name)
	}

	return factory(r.logger), nil
}

// ListStrategies returns all available strategy names
func (r *StrategyRegistry) ListStrategies() []string {
	strategies := make([]string, 0, len(r.strategies))
	for name := range r.strategies {
		strategies = append(strategies, name)
	}
	return strategies
}

// RegisterCustomStrategy allows registration of custom strategies
func (r *StrategyRegistry) RegisterCustomStrategy(name string, factory func(*logging.Logger) SecretValueRotator) error {
	if _, exists := r.strategies[name]; exists {
		return fmt.Errorf("strategy '%s' already registered", name)
	}

	r.strategies[name] = factory
	r.logger.Debug("Registered custom rotation strategy: %s", name)
	return nil
}

// HasStrategy checks if a strategy is available
func (r *StrategyRegistry) HasStrategy(name string) bool {
	_, exists := r.strategies[name]
	return exists
}