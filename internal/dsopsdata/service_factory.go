package dsopsdata

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/pkg/protocol"
	"github.com/systmms/dsops/pkg/service"
)

// DataDrivenServiceFactory creates services using dsops-data definitions
type DataDrivenServiceFactory struct {
	repository *Repository
	registry   *protocol.Registry
}

// NewDataDrivenServiceFactory creates a new data-driven service factory
func NewDataDrivenServiceFactory(repository *Repository) *DataDrivenServiceFactory {
	// Create and populate protocol registry
	registry := protocol.NewRegistry()
	
	// Register all protocol adapters
	_ = registry.Register(protocol.NewSQLAdapter())
	_ = registry.Register(protocol.NewHTTPAPIAdapter())
	_ = registry.Register(protocol.NewNoSQLAdapter())
	_ = registry.Register(protocol.NewCertificateAdapter())
	
	return &DataDrivenServiceFactory{
		repository: repository,
		registry:   registry,
	}
}

// CreateService creates a service instance from configuration using dsops-data
func (f *DataDrivenServiceFactory) CreateService(name string, cfg config.ServiceConfig) (service.Service, error) {
	serviceType, exists := f.repository.GetServiceType(cfg.Type)
	if !exists {
		return nil, fmt.Errorf("unknown service type: %s", cfg.Type)
	}

	return &DataDrivenService{
		name:         name,
		serviceType:  serviceType,
		config:       cfg,
		repository:   f.repository,
		registry:     f.registry,
	}, nil
}

// GetSupportedTypes returns all supported service types from dsops-data
func (f *DataDrivenServiceFactory) GetSupportedTypes() []string {
	return f.repository.ListServiceTypes()
}

// IsSupported checks if a service type is supported
func (f *DataDrivenServiceFactory) IsSupported(serviceType string) bool {
	_, exists := f.repository.GetServiceType(serviceType)
	return exists
}

// DataDrivenService implements the service.Service interface using dsops-data definitions
type DataDrivenService struct {
	name        string
	serviceType *ServiceType
	config      config.ServiceConfig
	repository  *Repository
	registry    *protocol.Registry
}

func (s *DataDrivenService) Name() string {
	return s.name
}

func (s *DataDrivenService) Plan(ctx context.Context, req service.RotationRequest) (service.RotationPlan, error) {
	// Find the credential kind being rotated
	var credentialKind *CredentialKind
	for _, kind := range s.serviceType.Spec.CredentialKinds {
		if kind.Name == req.ServiceRef.Kind {
			credentialKind = &kind
			break
		}
	}

	if credentialKind == nil {
		return service.RotationPlan{}, fmt.Errorf("credential kind %s not supported by service type %s", req.ServiceRef.Kind, s.serviceType.Metadata.Name)
	}

	// Determine rotation strategy from service type defaults or request
	strategy := req.Strategy
	if strategy == "" {
		strategy = s.serviceType.Spec.Defaults.RotationStrategy
	}
	if strategy == "" {
		strategy = "immediate" // Default fallback
	}

	// Build rotation plan based on strategy
	plan := service.RotationPlan{
		ServiceRef:    req.ServiceRef,
		Strategy:      strategy,
		Steps:         []service.RotationStep{},
		EstimatedTime: 60 * time.Second, // Default estimation
		Fingerprint:   service.GenerateFingerprint(req),
		CreatedAt:     time.Now(),
		Metadata:      req.Metadata,
	}

	// Add steps based on strategy and capabilities
	switch strategy {
	case "two-key":
		if contains(credentialKind.Capabilities, "create") {
			plan.Steps = append(plan.Steps, service.RotationStep{
				Name:        "create_new",
				Description: fmt.Sprintf("Create new %s credential", req.ServiceRef.Kind),
				Action:      "create",
				Target:      fmt.Sprintf("%s:new", req.ServiceRef.Kind),
			})
		}
		if contains(credentialKind.Capabilities, "verify") {
			plan.Steps = append(plan.Steps, service.RotationStep{
				Name:        "verify_new",
				Description: fmt.Sprintf("Verify new %s credential works", req.ServiceRef.Kind),
				Action:      "verify",
				Target:      fmt.Sprintf("%s:new", req.ServiceRef.Kind),
			})
		}
		plan.Steps = append(plan.Steps, service.RotationStep{
			Name:        "promote_new",
			Description: fmt.Sprintf("Activate new %s credential", req.ServiceRef.Kind),
			Action:      "promote",
			Target:      req.ServiceRef.Kind,
		})
		if contains(credentialKind.Capabilities, "revoke") {
			plan.Steps = append(plan.Steps, service.RotationStep{
				Name:        "revoke_old",
				Description: fmt.Sprintf("Revoke old %s credential", req.ServiceRef.Kind),
				Action:      "delete",
				Target:      fmt.Sprintf("%s:old", req.ServiceRef.Kind),
			})
		}

	case "immediate":
		if contains(credentialKind.Capabilities, "rotate") {
			plan.Steps = append(plan.Steps, service.RotationStep{
				Name:        "rotate_immediate",
				Description: fmt.Sprintf("Rotate %s credential immediately", req.ServiceRef.Kind),
				Action:      "create",
				Target:      req.ServiceRef.Kind,
			})
		}
		if contains(credentialKind.Capabilities, "verify") {
			plan.Steps = append(plan.Steps, service.RotationStep{
				Name:        "verify_rotated",
				Description: fmt.Sprintf("Verify rotated %s credential", req.ServiceRef.Kind),
				Action:      "verify",
				Target:      req.ServiceRef.Kind,
			})
		}

	case "overlap":
		if contains(credentialKind.Capabilities, "create") {
			plan.Steps = append(plan.Steps, service.RotationStep{
				Name:        "create_overlapping",
				Description: fmt.Sprintf("Create new %s credential", req.ServiceRef.Kind),
				Action:      "create",
				Target:      fmt.Sprintf("%s:new", req.ServiceRef.Kind),
			})
		}
		if contains(credentialKind.Capabilities, "verify") {
			plan.Steps = append(plan.Steps, service.RotationStep{
				Name:        "verify_overlapping",
				Description: fmt.Sprintf("Verify new %s credential works", req.ServiceRef.Kind),
				Action:      "verify",
				Target:      fmt.Sprintf("%s:new", req.ServiceRef.Kind),
			})
		}
		plan.Steps = append(plan.Steps, service.RotationStep{
			Name:        "activate_overlapping",
			Description: fmt.Sprintf("Activate new %s credential", req.ServiceRef.Kind),
			Action:      "promote",
			Target:      req.ServiceRef.Kind,
		})
		// Note: In overlap strategy, old credential is NOT immediately revoked

	default:
		return service.RotationPlan{}, fmt.Errorf("unsupported rotation strategy: %s", strategy)
	}

	return plan, nil
}

func (s *DataDrivenService) Execute(ctx context.Context, plan service.RotationPlan) (service.RotationResult, error) {
	// Determine protocol adapter based on service category
	protocolType := s.getProtocolType()
	adapter, err := s.registry.GetByProtocol(protocolType)
	if err != nil {
		return service.RotationResult{
			ServiceRef:  plan.ServiceRef,
			Plan:        plan,
			Status:      "failed",
			Error:       fmt.Sprintf("No protocol adapter found for %s: %v", protocolType, err),
		}, err
	}
	
	// Build adapter configuration
	adapterConfig := s.buildAdapterConfig()
	
	// Initialize result
	result := service.RotationResult{
		ServiceRef:    plan.ServiceRef,
		Plan:          plan,
		Status:        "in_progress",
		StartedAt:     time.Now(),
		ExecutedSteps: []service.ExecutedStep{},
		Metadata:      make(map[string]string),
	}
	
	// Execute each step using the protocol adapter
	for _, step := range plan.Steps {
		executedStep := service.ExecutedStep{
			Step:      step,
			StartedAt: time.Now(),
			Status:    "in_progress",
		}
		
		// Build protocol operation from step
		operation := s.buildProtocolOperation(step, plan)
		
		// Execute via protocol adapter
		adapterResult, err := adapter.Execute(ctx, operation, adapterConfig)
		
		executedStep.CompletedAt = time.Now()
		
		if err != nil {
			executedStep.Status = "failed"
			executedStep.Error = err.Error()
			result.Status = "failed"
			result.Error = fmt.Sprintf("Step %s failed: %v", step.Name, err)
		} else if !adapterResult.Success {
			executedStep.Status = "failed"
			executedStep.Error = adapterResult.Error
			result.Status = "failed"
			result.Error = fmt.Sprintf("Step %s failed: %s", step.Name, adapterResult.Error)
		} else {
			executedStep.Status = "success"
			executedStep.Output = fmt.Sprintf("%v", adapterResult.Data)
			
			// Store important results in metadata
			if step.Action == "create" {
				if newValue, ok := adapterResult.Data["value"]; ok {
					result.Metadata["new_value"] = fmt.Sprintf("%v", newValue)
				}
				if serial, ok := adapterResult.Data["serial_number"]; ok {
					result.Metadata["serial_number"] = fmt.Sprintf("%v", serial)
				}
			}
		}
		
		result.ExecutedSteps = append(result.ExecutedSteps, executedStep)
		
		// Stop on failure
		if result.Status == "failed" {
			break
		}
	}
	
	// Set final status
	result.CompletedAt = time.Now()
	if result.Status != "failed" {
		result.Status = "success"
	}
	
	return result, nil
}

func (s *DataDrivenService) Verify(ctx context.Context, result service.RotationResult) error {
	// This would implement verification logic based on the service type definition
	return fmt.Errorf("data-driven service verification not yet implemented for service type %s", s.serviceType.Metadata.Name)
}

func (s *DataDrivenService) Rollback(ctx context.Context, result service.RotationResult) error {
	// This would implement rollback logic based on the service type capabilities
	return fmt.Errorf("data-driven service rollback not yet implemented for service type %s", s.serviceType.Metadata.Name)
}

func (s *DataDrivenService) GetStatus(ctx context.Context, ref service.ServiceRef) (service.RotationStatus, error) {
	// This would query the actual service for credential status
	return service.RotationStatus{
		ServiceRef: ref,
		Status:     "unknown",
		Warnings:   []string{fmt.Sprintf("Status checking not yet implemented for service type %s", s.serviceType.Metadata.Name)},
	}, fmt.Errorf("data-driven service status not yet implemented for service type %s", s.serviceType.Metadata.Name)
}

func (s *DataDrivenService) Capabilities() service.ServiceCapabilities {
	capabilities := service.ServiceCapabilities{
		MaxActiveKeys:       1,
		SupportsVersioning:  false,
		SupportsExpiration:  false,
		SupportedStrategies: []string{},
	}

	// Determine max active keys from credential kinds
	maxActive := 1
	for _, credKind := range s.serviceType.Spec.CredentialKinds {
		if credKind.Constraints.MaxActive != nil {
			switch v := credKind.Constraints.MaxActive.(type) {
			case int:
				if v > maxActive {
					maxActive = v
				}
			case string:
				if v == "unlimited" {
					maxActive = -1 // Unlimited
					break
				} else if num, err := strconv.Atoi(v); err == nil && num > maxActive {
					maxActive = num
				}
			}
		}
	}
	capabilities.MaxActiveKeys = maxActive

	// Check if any credential kind supports expiration (has TTL)
	for _, credKind := range s.serviceType.Spec.CredentialKinds {
		if credKind.Constraints.TTL != "" {
			capabilities.SupportsExpiration = true
			break
		}
	}

	// Determine supported strategies based on capabilities
	hasCreate := false
	hasRotate := false
	hasRevoke := false

	for _, credKind := range s.serviceType.Spec.CredentialKinds {
		if contains(credKind.Capabilities, "create") {
			hasCreate = true
		}
		if contains(credKind.Capabilities, "rotate") {
			hasRotate = true
		}
		if contains(credKind.Capabilities, "revoke") {
			hasRevoke = true
		}
	}

	// Add supported strategies based on capabilities
	if hasRotate {
		capabilities.SupportedStrategies = append(capabilities.SupportedStrategies, "immediate")
	}
	if hasCreate {
		capabilities.SupportedStrategies = append(capabilities.SupportedStrategies, "overlap")
		if hasRevoke {
			capabilities.SupportedStrategies = append(capabilities.SupportedStrategies, "two-key")
		}
	}

	// Add default strategy if specified
	if s.serviceType.Spec.Defaults.RotationStrategy != "" {
		strategy := s.serviceType.Spec.Defaults.RotationStrategy
		if !contains(capabilities.SupportedStrategies, strategy) {
			capabilities.SupportedStrategies = append(capabilities.SupportedStrategies, strategy)
		}
	}

	return capabilities
}

func (s *DataDrivenService) Validate(ctx context.Context) error {
	// Validate that the service configuration is valid
	if s.serviceType == nil {
		return fmt.Errorf("service type definition is nil")
	}

	if s.serviceType.Metadata.Name == "" {
		return fmt.Errorf("service type name is empty")
	}

	if len(s.serviceType.Spec.CredentialKinds) == 0 {
		return fmt.Errorf("service type %s has no credential kinds defined", s.serviceType.Metadata.Name)
	}

	// Validate credential kinds have required capabilities
	for _, credKind := range s.serviceType.Spec.CredentialKinds {
		if len(credKind.Capabilities) == 0 {
			return fmt.Errorf("credential kind %s has no capabilities defined", credKind.Name)
		}
	}

	return nil
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// getProtocolType determines the protocol type based on service category
func (s *DataDrivenService) getProtocolType() string {
	// Use the service category from metadata
	category := s.serviceType.Metadata.Category
	
	// Map categories to protocol types
	switch category {
	case "database":
		// Determine if SQL or NoSQL based on service type
		if strings.Contains(s.serviceType.Metadata.Name, "mongo") ||
		   strings.Contains(s.serviceType.Metadata.Name, "redis") ||
		   strings.Contains(s.serviceType.Metadata.Name, "dynamo") {
			return "nosql"
		}
		return "sql"
	case "api-service", "api":
		return "http-api"
	case "certificate", "certificates":
		return "certificate"
	default:
		// Default to HTTP API for unknown categories
		return "http-api"
	}
}

// buildAdapterConfig builds the configuration for the protocol adapter
func (s *DataDrivenService) buildAdapterConfig() protocol.AdapterConfig {
	config := protocol.AdapterConfig{
		Connection:    make(map[string]string),
		Auth:          make(map[string]string),
		ServiceConfig: make(map[string]interface{}),
	}
	
	// Extract connection details from service config
	if s.config.Config != nil {
		// Connection parameters
		if host, ok := s.config.Config["host"].(string); ok {
			config.Connection["host"] = host
		}
		if port, ok := s.config.Config["port"].(string); ok {
			config.Connection["port"] = port
		} else if port, ok := s.config.Config["port"].(float64); ok {
			config.Connection["port"] = fmt.Sprintf("%d", int(port))
		}
		if database, ok := s.config.Config["database"].(string); ok {
			config.Connection["database"] = database
		}
		if baseURL, ok := s.config.Config["base_url"].(string); ok {
			config.Connection["base_url"] = baseURL
		}
		
		// Auth parameters
		if username, ok := s.config.Config["username"].(string); ok {
			config.Auth["username"] = username
		}
		if password, ok := s.config.Config["password"].(string); ok {
			config.Auth["password"] = password
		}
		if apiKey, ok := s.config.Config["api_key"].(string); ok {
			config.Auth["type"] = "api_key"
			config.Auth["value"] = apiKey
		}
		if token, ok := s.config.Config["token"].(string); ok {
			config.Auth["type"] = "bearer"
			config.Auth["value"] = token
		}
		
		// Timeout
		if timeout, ok := s.config.Config["timeout"].(float64); ok {
			config.Timeout = int(timeout)
		}
		if timeout, ok := s.config.Config["timeout"].(int); ok {
			config.Timeout = timeout
		}
	}
	
	// Set service type
	config.Connection["type"] = s.serviceType.Metadata.Name
	
	// Add service-specific configuration from dsops-data
	config.ServiceConfig = s.extractServiceConfig()
	
	return config
}

// buildProtocolOperation builds a protocol operation from a rotation step
func (s *DataDrivenService) buildProtocolOperation(step service.RotationStep, plan service.RotationPlan) protocol.Operation {
	operation := protocol.Operation{
		Action:     step.Action,
		Target:     step.Target,
		Parameters: make(map[string]interface{}),
		Metadata:   make(map[string]string),
	}
	
	// Extract credential kind from target (e.g., "password:new" -> "password")
	targetParts := strings.Split(step.Target, ":")
	credentialKind := targetParts[0]
	
	// Add parameters from plan metadata
	if plan.Metadata != nil {
		for k, v := range plan.Metadata {
			operation.Parameters[k] = v
		}
	}
	
	// Add specific parameters based on action
	switch step.Action {
	case "create", "rotate":
		// Add new value if provided
		if newValue, ok := plan.Metadata["new_value"]; ok {
			operation.Parameters["value"] = newValue
		} else {
			// Generate based on credential kind constraints
			operation.Parameters["generate"] = true
			operation.Parameters["credential_kind"] = credentialKind
		}
		
	case "verify":
		// Add credential to verify
		if value, ok := plan.Metadata["verify_value"]; ok {
			operation.Parameters["value"] = value
		}
		
	case "revoke", "delete":
		// Add identifier for what to revoke
		if oldValue, ok := plan.Metadata["old_value"]; ok {
			operation.Parameters["value"] = oldValue
		}
		if serial, ok := plan.Metadata["serial_number"]; ok {
			operation.Parameters["serial_number"] = serial
		}
	}
	
	// Add service reference info
	operation.Metadata["service_type"] = s.serviceType.Metadata.Name
	operation.Metadata["service_instance"] = plan.ServiceRef.Instance
	operation.Metadata["credential_kind"] = credentialKind
	
	return operation
}

// extractServiceConfig extracts service-specific configuration for protocol adapters
func (s *DataDrivenService) extractServiceConfig() map[string]interface{} {
	config := make(map[string]interface{})
	
	// Extract commands based on credential kinds and capabilities
	// Since dsops-data doesn't define commands directly, we'll build them
	// from the service type metadata and credential capabilities
	commands := make(map[string]interface{})
	for _, credKind := range s.serviceType.Spec.CredentialKinds {
		for _, capability := range credKind.Capabilities {
			key := fmt.Sprintf("%s_%s", capability, credKind.Name)
			// Protocol adapters will use service-specific templates
			// This is a placeholder for where commands would come from
			commands[key] = fmt.Sprintf("{{.%s}}", capability)
		}
	}
	if len(commands) > 0 {
		config["commands"] = commands
	}
	
	// Extract rate limiting
	if s.serviceType.Spec.Defaults.RateLimit != "" {
		config["rate_limit"] = s.serviceType.Spec.Defaults.RateLimit
	}
	
	// Add metadata that might be useful
	config["service_type"] = s.serviceType.Metadata.Name
	config["category"] = s.serviceType.Metadata.Category
	
	return config
}