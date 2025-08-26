package services

import (
	"fmt"

	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/pkg/service"
)

// Registry manages service creation and registration for rotation targets
type Registry struct {
	factories          map[string]ServiceFactory
	supportedTypes     map[string]bool
	dataDrivenFactory  *dsopsdata.DataDrivenServiceFactory
}

// ServiceFactory creates a service instance from configuration
type ServiceFactory func(name string, config map[string]interface{}) (service.Service, error)

// NewRegistry creates a new service registry with built-in services
func NewRegistry() *Registry {
	registry := &Registry{
		factories:      make(map[string]ServiceFactory),
		supportedTypes: make(map[string]bool),
	}

	// Legacy registry for hardcoded services (no longer used)

	return registry
}

// NewRegistryWithDataDriven creates a new service registry with dsops-data integration
func NewRegistryWithDataDriven(repository *dsopsdata.Repository) *Registry {
	registry := &Registry{
		factories:         make(map[string]ServiceFactory),
		supportedTypes:    make(map[string]bool),
		dataDrivenFactory: dsopsdata.NewDataDrivenServiceFactory(repository),
	}

	// Register all service types from dsops-data
	for _, serviceType := range repository.ListServiceTypes() {
		registry.supportedTypes[serviceType] = true
	}

	// All services now come from dsops-data - no hardcoded factories needed

	return registry
}

// RegisterFactory registers a service factory for a given type
func (r *Registry) RegisterFactory(serviceType string, factory ServiceFactory) {
	r.factories[serviceType] = factory
	r.supportedTypes[serviceType] = true
}

// CreateService creates a service instance from configuration
func (r *Registry) CreateService(name string, cfg config.ServiceConfig) (service.Service, error) {
	// Use data-driven factory if available
	if r.dataDrivenFactory != nil && r.dataDrivenFactory.IsSupported(cfg.Type) {
		return r.dataDrivenFactory.CreateService(name, cfg)
	}

	// Fall back to hardcoded factory if exists (should be rare)
	if factory, exists := r.factories[cfg.Type]; exists {
		return factory(name, cfg.Config)
	}

	return nil, fmt.Errorf("unknown service type: %s", cfg.Type)
}

// GetSupportedTypes returns a list of supported service types
func (r *Registry) GetSupportedTypes() []string {
	typeSet := make(map[string]bool)
	
	// Add hardcoded service types
	for serviceType := range r.supportedTypes {
		typeSet[serviceType] = true
	}
	
	// Add data-driven service types
	if r.dataDrivenFactory != nil {
		for _, serviceType := range r.dataDrivenFactory.GetSupportedTypes() {
			typeSet[serviceType] = true
		}
	}
	
	// Convert to slice
	types := make([]string, 0, len(typeSet))
	for serviceType := range typeSet {
		types = append(types, serviceType)
	}
	return types
}

// IsSupported checks if a service type is supported
func (r *Registry) IsSupported(serviceType string) bool {
	// Check hardcoded types first
	if r.supportedTypes[serviceType] {
		return true
	}
	
	// Check data-driven types
	if r.dataDrivenFactory != nil {
		return r.dataDrivenFactory.IsSupported(serviceType)
	}
	
	return false
}

// HasImplementation checks if a service type has an actual implementation
func (r *Registry) HasImplementation(serviceType string) bool {
	// Check data-driven factory first (primary implementation)
	if r.dataDrivenFactory != nil && r.dataDrivenFactory.IsSupported(serviceType) {
		return true
	}
	
	// Check hardcoded factories as fallback
	_, exists := r.factories[serviceType]
	return exists
}

// All service implementations now come from dsops-data via DataDrivenServiceFactory