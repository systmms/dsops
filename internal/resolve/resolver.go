package resolve

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/systmms/dsops/internal/config"
	dserrors "github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
)

// Resolver handles secret resolution and transformation
type Resolver struct {
	config    *config.Config
	providers map[string]provider.Provider
	logger    *logging.Logger
	mu        sync.RWMutex // Protects providers map for concurrent access
}

// New creates a new resolver instance
func New(cfg *config.Config) *Resolver {
	return &Resolver{
		config:    cfg,
		providers: make(map[string]provider.Provider),
		logger:    cfg.Logger,
	}
}

// RegisterProvider registers a provider for use by the resolver
func (r *Resolver) RegisterProvider(name string, p provider.Provider) {
	r.mu.Lock()
	r.providers[name] = p
	r.mu.Unlock()
	r.logger.Debug("Registered provider: %s", name)
}

// GetProvider returns a registered provider by name
func (r *Resolver) GetProvider(name string) (provider.Provider, bool) {
	r.mu.RLock()
	p, exists := r.providers[name]
	r.mu.RUnlock()
	return p, exists
}

// GetRegisteredProviders returns a map of all registered providers
func (r *Resolver) GetRegisteredProviders() map[string]provider.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// Return a copy to prevent external modification
	result := make(map[string]provider.Provider)
	for name, p := range r.providers {
		result[name] = p
	}
	return result
}

// ValidateProvider validates a single provider with timeout
func (r *Resolver) ValidateProvider(ctx context.Context, providerName string) error {
	r.mu.RLock()
	prov, exists := r.providers[providerName]
	r.mu.RUnlock()
	if !exists {
		return dserrors.ConfigError{
			Field:      "provider",
			Value:      providerName,
			Message:    "provider not registered",
			Suggestion: fmt.Sprintf("Check that provider '%s' is configured correctly", providerName),
		}
	}

	// Get provider configuration for timeout
	providerConfig, err := r.config.GetProvider(providerName)
	if err != nil {
		return err
	}

	// Create context with timeout for validation
	timeoutMs := providerConfig.GetProviderTimeout()
	timeoutCtx, cancel := withProviderTimeout(ctx, timeoutMs)
	defer cancel()

	// Validate the provider with timeout
	err = prov.Validate(timeoutCtx)
	if err != nil {
		// Check if it's a timeout error and enhance the message
		if timeoutErr := isTimeoutError(err, providerName, timeoutMs); timeoutErr != err {
			return timeoutErr
		}
		return dserrors.ProviderError(providerName, "validate", err)
	}

	return nil
}

// ResolvedVariable represents a resolved environment variable
type ResolvedVariable struct {
	Name        string
	Value       string
	Source      string
	Transformed bool
	Error       error
}

// PlanResult represents the result of planning variable resolution
type PlanResult struct {
	Variables []PlannedVariable
	Errors    []error
}

// PlannedVariable represents a variable that will be resolved
type PlannedVariable struct {
	Name      string
	Source    string
	Transform string
	Optional  bool
	Error     error
}

// Plan shows what variables would be resolved without fetching actual values
func (r *Resolver) Plan(ctx context.Context, envName string) (*PlanResult, error) {
	env, err := r.config.GetEnvironment(envName)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment %s: %w", envName, err)
	}

	result := &PlanResult{
		Variables: make([]PlannedVariable, 0, len(env)),
		Errors:    make([]error, 0),
	}

	for varName, variable := range env {
		planned := PlannedVariable{
			Name:      varName,
			Transform: variable.Transform,
			Optional:  variable.Optional,
		}

		// Determine source
		if variable.Literal != "" {
			planned.Source = "literal"
		} else if variable.From != nil {
			// Check if this is a service reference
			if variable.From.IsServiceReference() {
				planned.Source = variable.From.Service
				planned.Error = fmt.Errorf("service references (svc://) are for credential rotation, not secret retrieval")
				result.Errors = append(result.Errors, planned.Error)
			} else {
				providerName := variable.From.GetEffectiveProvider()
				legacyRef := variable.From.ToLegacyProviderRef()
				planned.Source = fmt.Sprintf("provider:%s key:%s", providerName, legacyRef.Key)

				// Check if provider exists
				r.mu.RLock()
				_, exists := r.providers[providerName]
				r.mu.RUnlock()
				if !exists {
					planned.Error = fmt.Errorf("provider '%s' not registered", providerName)
					result.Errors = append(result.Errors, planned.Error)
				}
			}
		} else {
			planned.Error = fmt.Errorf("variable '%s' has no source (literal or from)", varName)
			result.Errors = append(result.Errors, planned.Error)
		}

		result.Variables = append(result.Variables, planned)
	}

	return result, nil
}

// Resolve fetches and processes all variables for an environment
func (r *Resolver) Resolve(ctx context.Context, envName string) (map[string]ResolvedVariable, error) {
	env, err := r.config.GetEnvironment(envName)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment %s: %w", envName, err)
	}

	// Apply policy enforcement
	if err := r.enforcePolicies(envName, env); err != nil {
		return nil, err
	}

	return r.ResolveVariablesConcurrently(ctx, env)
}

// ResolveEnvironment fetches and processes all variables in the given environment map
// This method uses concurrent provider calls for better performance
func (r *Resolver) ResolveEnvironment(ctx context.Context, env config.Environment) (map[string]string, error) {
	// First, resolve all variables concurrently
	resolvedVars, err := r.ResolveVariablesConcurrently(ctx, env)
	if err != nil {
		return nil, err
	}

	// Convert to simple string map for backward compatibility
	result := make(map[string]string)
	for name, resolved := range resolvedVars {
		if resolved.Error == nil {
			result[name] = resolved.Value
		}
	}

	return result, nil
}

// ResolveVariablesConcurrently resolves all variables using concurrent provider calls
func (r *Resolver) ResolveVariablesConcurrently(ctx context.Context, env config.Environment) (map[string]ResolvedVariable, error) {
	result := make(map[string]ResolvedVariable)
	resultMutex := &sync.Mutex{}
	
	var wg sync.WaitGroup
	errorChan := make(chan error, len(env))
	
	// Use a semaphore to limit concurrent provider calls
	// This prevents overwhelming providers with too many concurrent requests
	maxConcurrent := 10
	semaphore := make(chan struct{}, maxConcurrent)

	for varName, variable := range env {
		wg.Add(1)
		go func(name string, varDef config.Variable) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			resolved := r.resolveVariable(ctx, name, varDef)
			
			resultMutex.Lock()
			result[name] = resolved
			resultMutex.Unlock()
			
			if resolved.Error != nil && !varDef.Optional {
				errorChan <- dserrors.UserError{
				Message:    fmt.Sprintf("Failed to resolve variable '%s'", name),
				Details:    resolved.Error.Error(),
				Suggestion: "Check that the provider is configured correctly and the secret exists",
				Err:        resolved.Error,
			}
			}
		}(varName, variable)
	}

	wg.Wait()
	close(errorChan)

	// Collect errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		if len(errors) == 1 {
			return result, errors[0]
		}
		return result, dserrors.UserError{
			Message:    fmt.Sprintf("Failed to resolve %d variables", len(errors)),
			Details:    fmt.Sprintf("%v", errors),
			Suggestion: "Fix the errors above and try again. Use 'dsops doctor' to check provider connectivity",
		}
	}

	return result, nil
}

// resolveVariable resolves a single variable (can be called concurrently)
func (r *Resolver) resolveVariable(ctx context.Context, varName string, variable config.Variable) ResolvedVariable {
	resolved := ResolvedVariable{
		Name: varName,
	}

	// Resolve value
	if variable.Literal != "" {
		// Use literal value
		resolved.Value = variable.Literal
		resolved.Source = "literal"
	} else if variable.From != nil {
		// Fetch from provider
		value, source, err := r.resolveFromProvider(ctx, variable.From)
		if err != nil {
			resolved.Error = err
			return resolved
		}
		resolved.Value = value
		resolved.Source = source
	} else {
		resolved.Error = dserrors.ConfigError{
			Field:      varName,
			Message:    "variable has no source defined",
			Suggestion: "Add either 'literal: value' or 'from: { provider: name, key: keyname }' to the variable",
		}
		return resolved
	}

	// Apply transforms
	if variable.Transform != "" {
		transformed, err := r.applyTransform(resolved.Value, variable.Transform)
		if err != nil {
			resolved.Error = dserrors.UserError{
				Message:    fmt.Sprintf("Transform failed for variable '%s'", varName),
				Details:    err.Error(),
				Suggestion: "Check the transform syntax. Available transforms: trim, base64_encode, base64_decode, json_extract:.path, yaml_extract:.path, multiline_to_single, replace:old:new, join:separator",
				Err:        err,
			}
		} else {
			resolved.Value = transformed
			resolved.Transformed = true
		}
	}

	return resolved
}

// resolveFromProvider fetches a value from the specified provider
func (r *Resolver) resolveFromProvider(ctx context.Context, ref *config.Reference) (string, string, error) {
	// Check if this is a service reference
	if ref.IsServiceReference() {
		return "", "", dserrors.ConfigError{
			Field:      "reference",
			Value:      ref.Service,
			Message:    "service references (svc://) are for credential rotation, not secret retrieval",
			Suggestion: "Use a secret store reference (store://) instead. Services define rotation targets, not secret sources",
		}
	}

	providerName := ref.GetEffectiveProvider()
	if providerName == "" {
		return "", "", dserrors.ConfigError{
			Field:      "provider",
			Value:      "unknown",
			Message:    "could not determine provider from reference",
			Suggestion: "Check your reference format",
		}
	}

	r.mu.RLock()
	prov, exists := r.providers[providerName]
	r.mu.RUnlock()
	if !exists {
		return "", "", dserrors.ConfigError{
			Field:      "provider",
			Value:      providerName,
			Message:    "provider not found in configuration",
			Suggestion: fmt.Sprintf("Add provider '%s' to the 'secretStores:' section of your dsops.yaml", providerName),
		}
	}

	// Get provider configuration for timeout
	providerConfig, err := r.config.GetProvider(providerName)
	if err != nil {
		return "", "", err
	}

	// Create context with timeout
	timeoutMs := providerConfig.GetProviderTimeout()
	timeoutCtx, cancel := withProviderTimeout(ctx, timeoutMs)
	defer cancel()

	// Convert to legacy format for compatibility with existing provider interface
	legacyRef := ref.ToLegacyProviderRef()
	providerRef := provider.Reference{
		Provider: providerName,
		Key:      legacyRef.Key,
		Version:  legacyRef.Version,
	}

	// Resolve the secret with timeout
	secret, err := prov.Resolve(timeoutCtx, providerRef)
	if err != nil {
		// Check if it's a timeout error and enhance the message
		if timeoutErr := isTimeoutError(err, providerName, timeoutMs); timeoutErr != err {
			return "", "", timeoutErr
		}
		return "", "", dserrors.ProviderError(providerName, "resolve", err)
	}

	source := fmt.Sprintf("%s:%s", providerName, legacyRef.Key)
	if secret.Version != "" {
		source += "@" + secret.Version
	}

	return secret.Value, source, nil
}

// applyTransform applies a transform string to a value
func (r *Resolver) applyTransform(value, transform string) (string, error) {
	// Support transform chains separated by commas or pipes
	transforms := strings.Split(transform, ",")
	if len(transforms) == 1 {
		transforms = strings.Split(transform, "|")
	}

	result := value
	for _, t := range transforms {
		t = strings.TrimSpace(t)
		var err error
		result, err = r.applySingleTransform(result, t)
		if err != nil {
			return "", fmt.Errorf("transform '%s' failed: %w", t, err)
		}
	}

	return result, nil
}

// applySingleTransform applies a single transform operation
func (r *Resolver) applySingleTransform(value, transform string) (string, error) {
	switch {
	case transform == "trim":
		return strings.TrimSpace(value), nil
	
	case transform == "multiline_to_single":
		return strings.ReplaceAll(strings.ReplaceAll(value, "\n", "\\n"), "\r", ""), nil
	
	case strings.HasPrefix(transform, "json_extract:"):
		path := strings.TrimPrefix(transform, "json_extract:")
		return extractJSONPath(value, path)
	
	case transform == "base64_decode":
		return base64Decode(value)
	
	case transform == "base64_encode":
		return base64Encode(value)
	
	case strings.HasPrefix(transform, "replace:"):
		parts := strings.SplitN(strings.TrimPrefix(transform, "replace:"), ":", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("replace transform requires format 'replace:from:to'")
		}
		return strings.ReplaceAll(value, parts[0], parts[1]), nil

	case strings.HasPrefix(transform, "yaml_extract:"):
		path := strings.TrimPrefix(transform, "yaml_extract:")
		return extractYAMLPath(value, path)

	case strings.HasPrefix(transform, "join:"):
		separator := strings.TrimPrefix(transform, "join:")
		return joinValues(value, separator)
	
	default:
		return "", fmt.Errorf("unknown transform: %s", transform)
	}
}

// enforcePolicies validates environment configuration against policies
func (r *Resolver) enforcePolicies(envName string, env config.Environment) error {
	if !r.config.HasPolicies() {
		return nil // No policies to enforce
	}

	enforcer := r.config.GetPolicyEnforcer()
	
	// Validate secret count for environment
	if err := enforcer.ValidateEnvironmentSecretCount(envName, len(env)); err != nil {
		return err
	}
	
	// Validate each variable's provider
	for varName, variable := range env {
		if variable.From != nil {
			// Get provider configuration to check type
			providerConfig, err := r.config.GetProvider(variable.From.Provider)
			if err != nil {
				continue // Provider validation will catch this later
			}
			
			// Validate provider type globally
			if err := enforcer.ValidateProviderType(providerConfig.Type); err != nil {
				return fmt.Errorf("variable %s: %w", varName, err)
			}
			
			// Validate provider type for this environment
			if err := enforcer.ValidateEnvironmentProvider(envName, providerConfig.Type); err != nil {
				return fmt.Errorf("variable %s: %w", varName, err)
			}
		}
	}
	
	return nil
}