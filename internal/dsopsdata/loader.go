package dsopsdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

// ServiceType represents a service type definition from dsops-data
type ServiceType struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Kind       string `yaml:"kind" json:"kind"`
	Metadata   struct {
		Name        string `yaml:"name" json:"name"`
		Description string `yaml:"description,omitempty" json:"description,omitempty"`
		Category    string `yaml:"category,omitempty" json:"category,omitempty"`
	} `yaml:"metadata" json:"metadata"`
	Spec struct {
		CredentialKinds []CredentialKind `yaml:"credentialKinds" json:"credentialKinds"`
		Defaults        struct {
			RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
			RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
		} `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	} `yaml:"spec" json:"spec"`
}

// CredentialKind represents a type of credential that can be managed
type CredentialKind struct {
	Name         string   `yaml:"name" json:"name"`
	Description  string   `yaml:"description,omitempty" json:"description,omitempty"`
	Capabilities []string `yaml:"capabilities" json:"capabilities"`
	Constraints  struct {
		MaxActive interface{} `yaml:"maxActive,omitempty" json:"maxActive,omitempty"` // Can be int or "unlimited"
		TTL       string      `yaml:"ttl,omitempty" json:"ttl,omitempty"`
		Format    string      `yaml:"format,omitempty" json:"format,omitempty"`
	} `yaml:"constraints,omitempty" json:"constraints,omitempty"`
}

// ServiceInstance represents a specific deployment of a service
type ServiceInstance struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Kind       string `yaml:"kind" json:"kind"`
	Metadata   struct {
		Type        string   `yaml:"type" json:"type"`
		ID          string   `yaml:"id" json:"id"`
		Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
		Description string   `yaml:"description,omitempty" json:"description,omitempty"`
		Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	} `yaml:"metadata" json:"metadata"`
	Spec struct {
		Endpoint        string                `yaml:"endpoint" json:"endpoint"`
		Auth            string                `yaml:"auth" json:"auth"`
		CredentialKinds []InstanceCredential  `yaml:"credentialKinds" json:"credentialKinds"`
		Config          map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
	} `yaml:"spec" json:"spec"`
}

// InstanceCredential represents a credential configuration for a service instance
type InstanceCredential struct {
	Name       string                 `yaml:"name" json:"name"`
	Policy     string                 `yaml:"policy" json:"policy"`
	Principals []string               `yaml:"principals" json:"principals"`
	Config     map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
}

// RotationPolicy represents a rotation policy definition
type RotationPolicy struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Kind       string `yaml:"kind" json:"kind"`
	Metadata   struct {
		Name        string `yaml:"name" json:"name"`
		Description string `yaml:"description,omitempty" json:"description,omitempty"`
	} `yaml:"metadata" json:"metadata"`
	Spec struct {
		Strategy     string         `yaml:"strategy" json:"strategy"`
		Schedule     string         `yaml:"schedule,omitempty" json:"schedule,omitempty"`
		Verification *Verification  `yaml:"verification,omitempty" json:"verification,omitempty"`
		Cutover      *Cutover       `yaml:"cutover,omitempty" json:"cutover,omitempty"`
		Notifications *Notifications `yaml:"notifications,omitempty" json:"notifications,omitempty"`
		Constraints  *Constraints   `yaml:"constraints,omitempty" json:"constraints,omitempty"`
	} `yaml:"spec" json:"spec"`
}

// Verification defines how to verify credentials work after creation
type Verification struct {
	Method   string `yaml:"method" json:"method"`
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Timeout  string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Retries  int    `yaml:"retries,omitempty" json:"retries,omitempty"`
}

// Cutover defines how to handle the cutover from old to new credentials
type Cutover struct {
	RequireCheck    bool   `yaml:"requireCheck,omitempty" json:"requireCheck,omitempty"`
	GracePeriod     string `yaml:"gracePeriod,omitempty" json:"gracePeriod,omitempty"`
	RollbackWindow  string `yaml:"rollbackWindow,omitempty" json:"rollbackWindow,omitempty"`
}

// Notifications defines notification settings for rotation events
type Notifications struct {
	OnSuccess     []string         `yaml:"onSuccess,omitempty" json:"onSuccess,omitempty"`
	OnFailure     []string         `yaml:"onFailure,omitempty" json:"onFailure,omitempty"`
	BeforeExpiry  *BeforeExpiry    `yaml:"beforeExpiry,omitempty" json:"beforeExpiry,omitempty"`
}

// BeforeExpiry defines notifications before credential expiration
type BeforeExpiry struct {
	Targets []string `yaml:"targets" json:"targets"`
	Advance string   `yaml:"advance" json:"advance"`
}

// Constraints defines additional constraints and requirements
type Constraints struct {
	RequireApproval      bool                   `yaml:"requireApproval,omitempty" json:"requireApproval,omitempty"`
	MaintenanceWindows   []MaintenanceWindow    `yaml:"maintenanceWindows,omitempty" json:"maintenanceWindows,omitempty"`
	ExcludeEnvironments  []string               `yaml:"excludeEnvironments,omitempty" json:"excludeEnvironments,omitempty"`
}

// MaintenanceWindow defines when rotation is allowed
type MaintenanceWindow struct {
	Cron     string `yaml:"cron" json:"cron"`
	Duration string `yaml:"duration" json:"duration"`
	Timezone string `yaml:"timezone,omitempty" json:"timezone,omitempty"`
}

// Principal represents an identity that can own or use credentials
type Principal struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Kind       string `yaml:"kind" json:"kind"`
	Metadata   struct {
		Name        string            `yaml:"name" json:"name"`
		Description string            `yaml:"description,omitempty" json:"description,omitempty"`
		Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	} `yaml:"metadata" json:"metadata"`
	Spec struct {
		Type        string                 `yaml:"type" json:"type"`
		Email       string                 `yaml:"email,omitempty" json:"email,omitempty"`
		Team        string                 `yaml:"team,omitempty" json:"team,omitempty"`
		Environment string                 `yaml:"environment,omitempty" json:"environment,omitempty"`
		Permissions *PrincipalPermissions  `yaml:"permissions,omitempty" json:"permissions,omitempty"`
		Contact     *PrincipalContact      `yaml:"contact,omitempty" json:"contact,omitempty"`
		Metadata    map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	} `yaml:"spec" json:"spec"`
}

// PrincipalPermissions defines permission settings for a principal
type PrincipalPermissions struct {
	AllowedServices         []string `yaml:"allowedServices,omitempty" json:"allowedServices,omitempty"`
	AllowedCredentialKinds  []string `yaml:"allowedCredentialKinds,omitempty" json:"allowedCredentialKinds,omitempty"`
	MaxCredentialTTL        string   `yaml:"maxCredentialTTL,omitempty" json:"maxCredentialTTL,omitempty"`
}

// PrincipalContact defines contact information for a principal
type PrincipalContact struct {
	Email  string `yaml:"email,omitempty" json:"email,omitempty"`
	Slack  string `yaml:"slack,omitempty" json:"slack,omitempty"`
	OnCall string `yaml:"oncall,omitempty" json:"oncall,omitempty"`
}

// Loader loads dsops-data definitions from a local directory
type Loader struct {
	dataDir     string
	schemasDir  string
	enableValidation bool
}

// NewLoader creates a new dsops-data loader
func NewLoader(dataDir string) *Loader {
	return &Loader{
		dataDir:          dataDir,
		schemasDir:       filepath.Join(dataDir, "schemas"),
		enableValidation: true,
	}
}

// NewLoaderWithoutValidation creates a loader that skips JSON schema validation
func NewLoaderWithoutValidation(dataDir string) *Loader {
	return &Loader{
		dataDir:          dataDir,
		schemasDir:       filepath.Join(dataDir, "schemas"),
		enableValidation: false,
	}
}

// validateWithSchema validates data against a JSON schema file
func (l *Loader) validateWithSchema(data interface{}, schemaFile string) error {
	if !l.enableValidation {
		return nil
	}

	// Check if schema file exists
	schemaPath := filepath.Join(l.schemasDir, schemaFile)
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		// Schema not found - skip validation with warning
		return nil
	}

	// Load schema
	schemaLoader := gojsonschema.NewReferenceLoader("file://" + schemaPath)
	
	// Convert data to JSON for validation
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data for validation: %w", err)
	}
	
	documentLoader := gojsonschema.NewBytesLoader(jsonData)
	
	// Perform validation
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}

	if !result.Valid() {
		var errorMessages []string
		for _, desc := range result.Errors() {
			errorMessages = append(errorMessages, desc.String())
		}
		return fmt.Errorf("schema validation failed:\n  - %s", strings.Join(errorMessages, "\n  - "))
	}

	return nil
}

// LoadServiceTypes loads all service type definitions
func (l *Loader) LoadServiceTypes(ctx context.Context) (map[string]*ServiceType, error) {
	serviceTypes := make(map[string]*ServiceType)
	serviceTypesDir := filepath.Join(l.dataDir, "service-types")

	err := filepath.WalkDir(serviceTypesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read service type file %s: %w", path, err)
		}

		var serviceType ServiceType
		if err := yaml.Unmarshal(data, &serviceType); err != nil {
			return fmt.Errorf("failed to unmarshal service type %s: %w", path, err)
		}

		if serviceType.Kind != "ServiceType" {
			return fmt.Errorf("invalid kind in service type file %s: expected ServiceType, got %s", path, serviceType.Kind)
		}

		// Validate against JSON schema
		if err := l.validateWithSchema(serviceType, "service-type.schema.json"); err != nil {
			return fmt.Errorf("validation failed for service type %s: %w", path, err)
		}

		serviceTypes[serviceType.Metadata.Name] = &serviceType
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load service types: %w", err)
	}

	return serviceTypes, nil
}

// LoadServiceInstances loads all service instance definitions
func (l *Loader) LoadServiceInstances(ctx context.Context) (map[string]*ServiceInstance, error) {
	instances := make(map[string]*ServiceInstance)
	instancesDir := filepath.Join(l.dataDir, "service-instances")

	err := filepath.WalkDir(instancesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read service instance file %s: %w", path, err)
		}

		var instance ServiceInstance
		if err := yaml.Unmarshal(data, &instance); err != nil {
			return fmt.Errorf("failed to unmarshal service instance %s: %w", path, err)
		}

		if instance.Kind != "ServiceInstance" {
			return fmt.Errorf("invalid kind in service instance file %s: expected ServiceInstance, got %s", path, instance.Kind)
		}

		// Validate against JSON schema
		if err := l.validateWithSchema(instance, "service-instance.schema.json"); err != nil {
			return fmt.Errorf("validation failed for service instance %s: %w", path, err)
		}

		key := fmt.Sprintf("%s/%s", instance.Metadata.Type, instance.Metadata.ID)
		instances[key] = &instance
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load service instances: %w", err)
	}

	return instances, nil
}

// LoadRotationPolicies loads all rotation policy definitions
func (l *Loader) LoadRotationPolicies(ctx context.Context) (map[string]*RotationPolicy, error) {
	policies := make(map[string]*RotationPolicy)
	policiesDir := filepath.Join(l.dataDir, "rotation-policies")

	err := filepath.WalkDir(policiesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read rotation policy file %s: %w", path, err)
		}

		var policy RotationPolicy
		if err := yaml.Unmarshal(data, &policy); err != nil {
			return fmt.Errorf("failed to unmarshal rotation policy %s: %w", path, err)
		}

		if policy.Kind != "RotationPolicy" {
			return fmt.Errorf("invalid kind in rotation policy file %s: expected RotationPolicy, got %s", path, policy.Kind)
		}

		// Validate against JSON schema
		if err := l.validateWithSchema(policy, "rotation-policy.schema.json"); err != nil {
			return fmt.Errorf("validation failed for rotation policy %s: %w", path, err)
		}

		policies[policy.Metadata.Name] = &policy
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load rotation policies: %w", err)
	}

	return policies, nil
}

// LoadPrincipals loads all principal definitions
func (l *Loader) LoadPrincipals(ctx context.Context) (map[string]*Principal, error) {
	principals := make(map[string]*Principal)
	principalsDir := filepath.Join(l.dataDir, "principals")

	err := filepath.WalkDir(principalsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read principal file %s: %w", path, err)
		}

		var principal Principal
		if err := yaml.Unmarshal(data, &principal); err != nil {
			return fmt.Errorf("failed to unmarshal principal %s: %w", path, err)
		}

		if principal.Kind != "Principal" {
			return fmt.Errorf("invalid kind in principal file %s: expected Principal, got %s", path, principal.Kind)
		}

		// Validate against JSON schema
		if err := l.validateWithSchema(principal, "principal.schema.json"); err != nil {
			return fmt.Errorf("validation failed for principal %s: %w", path, err)
		}

		principals[principal.Metadata.Name] = &principal
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load principals: %w", err)
	}

	return principals, nil
}

// LoadAll loads all dsops-data definitions
func (l *Loader) LoadAll(ctx context.Context) (*Repository, error) {
	serviceTypes, err := l.LoadServiceTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load service types: %w", err)
	}

	serviceInstances, err := l.LoadServiceInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load service instances: %w", err)
	}

	rotationPolicies, err := l.LoadRotationPolicies(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load rotation policies: %w", err)
	}

	principals, err := l.LoadPrincipals(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load principals: %w", err)
	}

	return &Repository{
		ServiceTypes:     serviceTypes,
		ServiceInstances: serviceInstances,
		RotationPolicies: rotationPolicies,
		Principals:       principals,
	}, nil
}

// Repository contains all loaded dsops-data definitions
type Repository struct {
	ServiceTypes     map[string]*ServiceType
	ServiceInstances map[string]*ServiceInstance
	RotationPolicies map[string]*RotationPolicy
	Principals       map[string]*Principal
}

// GetServiceType returns a service type by name
func (r *Repository) GetServiceType(name string) (*ServiceType, bool) {
	serviceType, exists := r.ServiceTypes[name]
	return serviceType, exists
}

// GetServiceInstance returns a service instance by type and ID
func (r *Repository) GetServiceInstance(serviceType, id string) (*ServiceInstance, bool) {
	key := fmt.Sprintf("%s/%s", serviceType, id)
	instance, exists := r.ServiceInstances[key]
	return instance, exists
}

// GetRotationPolicy returns a rotation policy by name
func (r *Repository) GetRotationPolicy(name string) (*RotationPolicy, bool) {
	policy, exists := r.RotationPolicies[name]
	return policy, exists
}

// GetPrincipal returns a principal by name
func (r *Repository) GetPrincipal(name string) (*Principal, bool) {
	principal, exists := r.Principals[name]
	return principal, exists
}

// ListServiceTypes returns all service type names
func (r *Repository) ListServiceTypes() []string {
	names := make([]string, 0, len(r.ServiceTypes))
	for name := range r.ServiceTypes {
		names = append(names, name)
	}
	return names
}

// ListServiceInstancesByType returns all service instances for a given type
func (r *Repository) ListServiceInstancesByType(serviceType string) []*ServiceInstance {
	var instances []*ServiceInstance
	prefix := serviceType + "/"
	
	for key, instance := range r.ServiceInstances {
		if strings.HasPrefix(key, prefix) {
			instances = append(instances, instance)
		}
	}
	
	return instances
}

// ListServiceInstancesByTag returns service instances that have any of the specified tags
func (r *Repository) ListServiceInstancesByTag(tags []string) []*ServiceInstance {
	var instances []*ServiceInstance
	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[tag] = true
	}
	
	for _, instance := range r.ServiceInstances {
		for _, instanceTag := range instance.Metadata.Tags {
			if tagSet[instanceTag] {
				instances = append(instances, instance)
				break
			}
		}
	}
	
	return instances
}

// Validate performs basic validation on the loaded repository
func (r *Repository) Validate() error {
	var errors []string

	// Validate service instances reference valid service types
	for key, instance := range r.ServiceInstances {
		if _, exists := r.ServiceTypes[instance.Metadata.Type]; !exists {
			errors = append(errors, fmt.Sprintf("service instance %s references unknown service type %s", key, instance.Metadata.Type))
		}

		// Validate credential kinds exist in service type
		serviceType, exists := r.ServiceTypes[instance.Metadata.Type]
		if exists {
			validKinds := make(map[string]bool)
			for _, credKind := range serviceType.Spec.CredentialKinds {
				validKinds[credKind.Name] = true
			}

			for _, credKind := range instance.Spec.CredentialKinds {
				if !validKinds[credKind.Name] {
					errors = append(errors, fmt.Sprintf("service instance %s references unknown credential kind %s for service type %s", key, credKind.Name, instance.Metadata.Type))
				}

				// Validate rotation policy exists
				if _, exists := r.RotationPolicies[credKind.Policy]; !exists {
					errors = append(errors, fmt.Sprintf("service instance %s references unknown rotation policy %s", key, credKind.Policy))
				}

				// Validate principals exist
				for _, principal := range credKind.Principals {
					if _, exists := r.Principals[principal]; !exists {
						errors = append(errors, fmt.Sprintf("service instance %s references unknown principal %s", key, principal))
					}
				}
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}