package protocol

import (
	"context"
	"fmt"
)

// AdapterType represents the type of protocol adapter
type AdapterType string

const (
	// AdapterTypeSQL handles SQL database operations (PostgreSQL, MySQL, etc.)
	AdapterTypeSQL AdapterType = "sql"
	
	// AdapterTypeHTTPAPI handles REST/HTTP API operations (Stripe, GitHub, etc.)
	AdapterTypeHTTPAPI AdapterType = "http-api"
	
	// AdapterTypeNoSQL handles NoSQL database operations (MongoDB, Redis, etc.)
	AdapterTypeNoSQL AdapterType = "nosql"
	
	// AdapterTypeCertificate handles certificate operations (ACME, Venafi, etc.)
	AdapterTypeCertificate AdapterType = "certificate"
)

// Operation represents a protocol-agnostic operation to perform
type Operation struct {
	// Action is what to do: create, verify, rotate, revoke, list
	Action string
	
	// Target identifies what to operate on (e.g., "password", "api-key")
	Target string
	
	// Parameters contains operation-specific data
	Parameters map[string]interface{}
	
	// Metadata contains additional context
	Metadata map[string]string
}

// AdapterConfig contains configuration for protocol adapters
type AdapterConfig struct {
	// Connection details (host, port, etc.)
	Connection map[string]string
	
	// Authentication credentials
	Auth map[string]string
	
	// Service-specific configuration from dsops-data
	ServiceConfig map[string]interface{}
	
	// Timeout and retry settings
	Timeout int // seconds
	Retries int
}

// Result represents the outcome of a protocol operation
type Result struct {
	// Success indicates if the operation succeeded
	Success bool
	
	// Data contains operation-specific results
	Data map[string]interface{}
	
	// Error message if operation failed
	Error string
	
	// Metadata about the operation
	Metadata map[string]string
}

// Adapter defines the interface for protocol adapters
type Adapter interface {
	// Name returns the adapter name
	Name() string
	
	// Type returns the adapter type (sql, http-api, nosql, certificate)
	Type() AdapterType
	
	// Execute performs a protocol operation
	Execute(ctx context.Context, operation Operation, config AdapterConfig) (*Result, error)
	
	// Validate checks if the adapter configuration is valid
	Validate(config AdapterConfig) error
	
	// Capabilities returns what operations this adapter supports
	Capabilities() Capabilities
}

// Capabilities describes what an adapter can do
type Capabilities struct {
	// SupportedActions lists operations this adapter can perform
	SupportedActions []string
	
	// RequiredConfig lists required configuration fields
	RequiredConfig []string
	
	// OptionalConfig lists optional configuration fields
	OptionalConfig []string
	
	// Features describes special features
	Features map[string]bool
}

// Registry manages protocol adapters
type Registry struct {
	adapters map[AdapterType]Adapter
}

// NewRegistry creates a new protocol adapter registry
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[AdapterType]Adapter),
	}
}

// Register adds an adapter to the registry
func (r *Registry) Register(adapter Adapter) error {
	if adapter == nil {
		return fmt.Errorf("adapter cannot be nil")
	}
	
	adapterType := adapter.Type()
	if _, exists := r.adapters[adapterType]; exists {
		return fmt.Errorf("adapter type %s already registered", adapterType)
	}
	
	r.adapters[adapterType] = adapter
	return nil
}

// Get retrieves an adapter by type
func (r *Registry) Get(adapterType AdapterType) (Adapter, error) {
	adapter, exists := r.adapters[adapterType]
	if !exists {
		return nil, fmt.Errorf("no adapter registered for type %s", adapterType)
	}
	return adapter, nil
}

// GetByProtocol retrieves an adapter by protocol string
func (r *Registry) GetByProtocol(protocol string) (Adapter, error) {
	return r.Get(AdapterType(protocol))
}

// List returns all registered adapter types
func (r *Registry) List() []AdapterType {
	types := make([]AdapterType, 0, len(r.adapters))
	for t := range r.adapters {
		types = append(types, t)
	}
	return types
}

// DefaultRegistry is the global protocol adapter registry
var DefaultRegistry = NewRegistry()

// Register adds an adapter to the default registry
func Register(adapter Adapter) error {
	return DefaultRegistry.Register(adapter)
}

// Get retrieves an adapter from the default registry
func Get(adapterType AdapterType) (Adapter, error) {
	return DefaultRegistry.Get(adapterType)
}

// GetByProtocol retrieves an adapter by protocol string from the default registry
func GetByProtocol(protocol string) (Adapter, error) {
	return DefaultRegistry.GetByProtocol(protocol)
}