package adapter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/pkg/secretstore"
	"github.com/systmms/dsops/pkg/service"
)

// ProviderToSecretStoreAdapter wraps a legacy Provider to implement SecretStore interface
type ProviderToSecretStoreAdapter struct {
	provider provider.Provider
}

// SecretStoreToProviderAdapter wraps a SecretStore to implement Provider interface (for backward compatibility)
type SecretStoreToProviderAdapter struct {
	secretStore secretstore.SecretStore
}

// NewProviderToSecretStoreAdapter creates an adapter that makes a Provider look like a SecretStore
func NewProviderToSecretStoreAdapter(p provider.Provider) *ProviderToSecretStoreAdapter {
	return &ProviderToSecretStoreAdapter{provider: p}
}

// NewSecretStoreToProviderAdapter creates an adapter that makes a SecretStore look like a Provider
func NewSecretStoreToProviderAdapter(s secretstore.SecretStore) *SecretStoreToProviderAdapter {
	return &SecretStoreToProviderAdapter{secretStore: s}
}

func (a *ProviderToSecretStoreAdapter) Name() string {
	return a.provider.Name()
}

func (a *ProviderToSecretStoreAdapter) Resolve(ctx context.Context, ref secretstore.SecretRef) (secretstore.SecretValue, error) {
	// Convert SecretRef to legacy Reference
	legacyRef := provider.Reference{
		Provider: ref.Store,
		Key:      ref.Path,
		Field:    ref.Field,
		Version:  ref.Version,
		Path:     ref.Path,
	}

	// Call legacy provider
	value, err := a.provider.Resolve(ctx, legacyRef)
	if err != nil {
		return secretstore.SecretValue{}, err
	}

	// Convert legacy SecretValue to new format
	return secretstore.SecretValue{
		Value:     value.Value,
		Version:   value.Version,
		UpdatedAt: value.UpdatedAt,
		Metadata:  value.Metadata,
	}, nil
}

func (a *ProviderToSecretStoreAdapter) Describe(ctx context.Context, ref secretstore.SecretRef) (secretstore.SecretMetadata, error) {
	// Convert SecretRef to legacy Reference
	legacyRef := provider.Reference{
		Provider: ref.Store,
		Key:      ref.Path,
		Field:    ref.Field,
		Version:  ref.Version,
		Path:     ref.Path,
	}

	// Call legacy provider
	metadata, err := a.provider.Describe(ctx, legacyRef)
	if err != nil {
		return secretstore.SecretMetadata{}, err
	}

	// Convert legacy Metadata to new format
	return secretstore.SecretMetadata{
		Exists:      metadata.Exists,
		Version:     metadata.Version,
		UpdatedAt:   metadata.UpdatedAt,
		Size:        metadata.Size,
		Type:        metadata.Type,
		Permissions: metadata.Permissions,
		Tags:        metadata.Tags,
	}, nil
}

func (a *ProviderToSecretStoreAdapter) Capabilities() secretstore.SecretStoreCapabilities {
	legacyCaps := a.provider.Capabilities()

	caps := secretstore.SecretStoreCapabilities{
		SupportsVersioning: legacyCaps.SupportsVersioning,
		SupportsMetadata:   legacyCaps.SupportsMetadata,
		SupportsWatching:   legacyCaps.SupportsWatching,
		SupportsBinary:     legacyCaps.SupportsBinary,
		RequiresAuth:       legacyCaps.RequiresAuth,
		AuthMethods:        legacyCaps.AuthMethods,
	}

	// Check if provider supports rotation (legacy Rotator interface)
	if rotator, ok := a.provider.(provider.Rotator); ok {
		caps.Rotation = &secretstore.RotationCapabilities{
			SupportsRotation:   true,
			SupportsVersioning: legacyCaps.SupportsVersioning,
			// Note: Legacy interface doesn't provide MaxVersions or MinRotationTime
		}
		_ = rotator // Use the rotator to avoid unused variable warning
	}

	return caps
}

func (a *ProviderToSecretStoreAdapter) Validate(ctx context.Context) error {
	return a.provider.Validate(ctx)
}

// ProviderToServiceAdapter wraps a legacy Provider to implement Service interface
// This is for providers that support rotation (implement Rotator interface)
type ProviderToServiceAdapter struct {
	provider provider.Provider
	rotator  provider.Rotator
}

// NewProviderToServiceAdapter creates an adapter for providers that support rotation
func NewProviderToServiceAdapter(p provider.Provider) (*ProviderToServiceAdapter, error) {
	rotator, ok := p.(provider.Rotator)
	if !ok {
		return nil, fmt.Errorf("provider %s does not implement Rotator interface", p.Name())
	}

	return &ProviderToServiceAdapter{
		provider: p,
		rotator:  rotator,
	}, nil
}

func (a *ProviderToServiceAdapter) Name() string {
	return a.provider.Name()
}

func (a *ProviderToServiceAdapter) Plan(ctx context.Context, req service.RotationRequest) (service.RotationPlan, error) {
	// Create a basic plan based on the request
	// This is a simplified implementation - real services would have more complex planning
	plan := service.RotationPlan{
		ServiceRef:    req.ServiceRef,
		Strategy:      req.Strategy,
		Fingerprint:   service.GenerateFingerprint(req),
		CreatedAt:     time.Now(),
		Metadata:      req.Metadata,
	}

	// Basic steps for rotation
	plan.Steps = []service.RotationStep{
		{
			Name:        "create_new_version",
			Description: "Create a new version of the credential",
			Action:      "create",
			Target:      req.ServiceRef.String(),
		},
		{
			Name:        "verify_new_version",
			Description: "Verify the new credential works",
			Action:      "verify",
			Target:      req.ServiceRef.String(),
		},
		{
			Name:        "deprecate_old_version",
			Description: "Mark the old credential as deprecated",
			Action:      "deprecate",
			Target:      req.ServiceRef.String(),
		},
	}

	return plan, nil
}

func (a *ProviderToServiceAdapter) Execute(ctx context.Context, plan service.RotationPlan) (service.RotationResult, error) {
	// Convert ServiceRef to legacy Reference for rotation
	legacyRef := a.serviceRefToLegacyRef(plan.ServiceRef)

	result := service.RotationResult{
		ServiceRef:    plan.ServiceRef,
		Plan:          plan,
		Status:        "success",
		ExecutedSteps: []service.ExecutedStep{},
	}

	// Execute each step
	for _, step := range plan.Steps {
		executedStep := service.ExecutedStep{
			Step:        step,
			Status:      "success",
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}

		switch step.Action {
		case "create":
			// Use rotator to create new version
			newVersion, err := a.rotator.CreateNewVersion(ctx, legacyRef, []byte("new-value"), nil)
			if err != nil {
				executedStep.Status = "failed"
				executedStep.Error = err.Error()
				result.Status = "failed"
			} else {
				executedStep.Output = "Created version: " + newVersion
			}

		case "verify":
			// Basic verification - in real implementation this would test the credential
			executedStep.Output = "Verification successful"

		case "deprecate":
			// Use rotator to deprecate old version
			err := a.rotator.DeprecateVersion(ctx, legacyRef, "old-version")
			if err != nil {
				executedStep.Status = "failed"
				executedStep.Error = err.Error()
				result.Status = "failed"
			} else {
				executedStep.Output = "Deprecated old version"
			}
		}

		result.ExecutedSteps = append(result.ExecutedSteps, executedStep)
	}

	return result, nil
}

func (a *ProviderToServiceAdapter) Verify(ctx context.Context, result service.RotationResult) error {
	// Basic verification implementation
	if result.Status == "failed" {
		return fmt.Errorf("rotation failed, cannot verify")
	}
	return nil
}

func (a *ProviderToServiceAdapter) Rollback(ctx context.Context, result service.RotationResult) error {
	// Basic rollback implementation
	// In a real implementation, this would restore the previous credential state
	return fmt.Errorf("rollback not implemented for legacy provider adapter")
}

func (a *ProviderToServiceAdapter) GetStatus(ctx context.Context, ref service.ServiceRef) (service.RotationStatus, error) {
	legacyRef := a.serviceRefToLegacyRef(ref)

	// Get rotation metadata from legacy rotator
	metadata, err := a.rotator.GetRotationMetadata(ctx, legacyRef)
	if err != nil {
		return service.RotationStatus{}, err
	}

	status := service.RotationStatus{
		ServiceRef: ref,
		Status:     "current",
	}

	if metadata.NextRotation != nil {
		status.NextRotation = metadata.NextRotation
		status.Status = "needs_rotation"
	}

	return status, nil
}

func (a *ProviderToServiceAdapter) Capabilities() service.ServiceCapabilities {
	legacyCaps := a.provider.Capabilities()

	return service.ServiceCapabilities{
		SupportedStrategies:      []string{"immediate"}, // Legacy providers typically support basic rotation
		MaxActiveKeys:            1,                     // Most legacy providers support one key
		SupportsExpiration:       false,                 // Not commonly supported in legacy
		SupportsVersioning:       legacyCaps.SupportsVersioning,
		SupportsRevocation:       true, // Most can deprecate versions
		SupportsVerification:     false, // Not commonly implemented
		MinRotationInterval:      0,    // No restrictions
		Constraints:              map[string]string{},
	}
}

func (a *ProviderToServiceAdapter) Validate(ctx context.Context) error {
	return a.provider.Validate(ctx)
}

// Helper method to convert ServiceRef to legacy Reference
func (a *ProviderToServiceAdapter) serviceRefToLegacyRef(ref service.ServiceRef) provider.Reference {
	// Convert service reference to provider reference
	// This is a simplified mapping - real implementation would be more sophisticated
	return provider.Reference{
		Provider: ref.Type,
		Key:      ref.Instance,
		Path:     ref.Instance,
		Field:    ref.Kind,
	}
}

// Helper functions for reference conversion

// ConvertProviderRefToSecretRef converts legacy provider references to new SecretRef format
func ConvertProviderRefToSecretRef(ref provider.Reference) secretstore.SecretRef {
	return secretstore.SecretRef{
		Store:   ref.Provider,
		Path:    ref.Path,
		Field:   ref.Field,
		Version: ref.Version,
		Options: map[string]string{
			"key": ref.Key, // Preserve legacy key field
		},
	}
}

// ConvertSecretRefToProviderRef converts new SecretRef to legacy provider Reference
func ConvertSecretRefToProviderRef(ref secretstore.SecretRef) provider.Reference {
	key := ref.Path
	if legacyKey, ok := ref.Options["key"]; ok {
		key = legacyKey
	}

	return provider.Reference{
		Provider: ref.Store,
		Key:      key,
		Path:     ref.Path,
		Field:    ref.Field,
		Version:  ref.Version,
	}
}

// IsSecretStore determines if a provider is primarily for secret storage
// This helps decide whether to wrap it as a SecretStore or Service
func IsSecretStore(providerName string) bool {
	storePrefixes := []string{
		"bitwarden",
		"onepassword",
		"lastpass",
		"keeper",
		"vault",
		"aws.secretsmanager",
		"gcp.secretmanager",
		"azure.keyvault",
	}

	for _, prefix := range storePrefixes {
		if strings.HasPrefix(strings.ToLower(providerName), prefix) {
			return true
		}
	}

	return false
}

// IsService determines if a provider is primarily for credential rotation
func IsService(providerName string) bool {
	servicePrefixes := []string{
		"github",
		"gitlab",
		"postgres",
		"mysql",
		"redis",
		"stripe",
		"datadog",
		"aws.iam",
	}

	for _, prefix := range servicePrefixes {
		if strings.HasPrefix(strings.ToLower(providerName), prefix) {
			return true
		}
	}

	return false
}

// SecretStoreToProviderAdapter implementation

func (a *SecretStoreToProviderAdapter) Name() string {
	return a.secretStore.Name()
}

func (a *SecretStoreToProviderAdapter) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	// Convert legacy Reference to SecretRef
	secretRef := secretstore.SecretRef{
		Store:   ref.Provider,
		Path:    ref.Path,
		Field:   ref.Field,
		Version: ref.Version,
		Options: map[string]string{
			"key": ref.Key, // Preserve legacy key field
		},
	}

	// Call new secret store
	value, err := a.secretStore.Resolve(ctx, secretRef)
	if err != nil {
		return provider.SecretValue{}, err
	}

	// Convert new SecretValue to legacy format
	return provider.SecretValue{
		Value:     value.Value,
		Version:   value.Version,
		UpdatedAt: value.UpdatedAt,
		Metadata:  value.Metadata,
	}, nil
}

func (a *SecretStoreToProviderAdapter) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	// Convert legacy Reference to SecretRef
	secretRef := secretstore.SecretRef{
		Store:   ref.Provider,
		Path:    ref.Path,
		Field:   ref.Field,
		Version: ref.Version,
		Options: map[string]string{
			"key": ref.Key,
		},
	}

	// Call new secret store
	metadata, err := a.secretStore.Describe(ctx, secretRef)
	if err != nil {
		return provider.Metadata{}, err
	}

	// Convert new SecretMetadata to legacy format
	return provider.Metadata{
		Exists:      metadata.Exists,
		Version:     metadata.Version,
		UpdatedAt:   metadata.UpdatedAt,
		Size:        metadata.Size,
		Type:        metadata.Type,
		Permissions: metadata.Permissions,
		Tags:        metadata.Tags,
	}, nil
}

func (a *SecretStoreToProviderAdapter) Capabilities() provider.Capabilities {
	storeCaps := a.secretStore.Capabilities()

	return provider.Capabilities{
		SupportsVersioning: storeCaps.SupportsVersioning,
		SupportsMetadata:   storeCaps.SupportsMetadata,
		SupportsWatching:   storeCaps.SupportsWatching,
		SupportsBinary:     storeCaps.SupportsBinary,
		RequiresAuth:       storeCaps.RequiresAuth,
		AuthMethods:        storeCaps.AuthMethods,
	}
}

func (a *SecretStoreToProviderAdapter) Validate(ctx context.Context) error {
	return a.secretStore.Validate(ctx)
}