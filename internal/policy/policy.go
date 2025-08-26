package policy

import (
	"fmt"
	"regexp"
	"strings"

	dserrors "github.com/systmms/dsops/internal/errors"
)

// PolicyConfig defines security policies for dsops operations
type PolicyConfig struct {
	// Global policies
	AllowedProviders   []string          `yaml:"allowed_providers,omitempty"`   // Whitelist of allowed provider types
	BlockedProviders   []string          `yaml:"blocked_providers,omitempty"`   // Blacklist of blocked provider types
	RequireEncryption  bool              `yaml:"require_encryption,omitempty"`  // Require encrypted provider configs
	
	// Secret policies
	SecretComplexity   *ComplexityPolicy `yaml:"secret_complexity,omitempty"`   // Secret value complexity requirements
	ForbiddenPatterns  []string          `yaml:"forbidden_patterns,omitempty"`  // Regex patterns that secrets must not match
	RequiredPatterns   []string          `yaml:"required_patterns,omitempty"`   // Regex patterns that secrets must match
	
	// Environment policies
	EnvironmentRules   map[string]*EnvironmentPolicy `yaml:"environment_rules,omitempty"` // Per-environment restrictions
	
	// File policies
	OutputRestrictions *OutputPolicy     `yaml:"output_restrictions,omitempty"` // File output restrictions
	
	// Audit policies
	AuditLogging       *AuditPolicy      `yaml:"audit_logging,omitempty"`       // Audit and compliance logging
}

// ComplexityPolicy defines requirements for secret values
type ComplexityPolicy struct {
	MinLength    int  `yaml:"min_length,omitempty"`     // Minimum secret length
	MaxLength    int  `yaml:"max_length,omitempty"`     // Maximum secret length  
	RequireUpper bool `yaml:"require_upper,omitempty"`  // Require uppercase letters
	RequireLower bool `yaml:"require_lower,omitempty"`  // Require lowercase letters
	RequireDigit bool `yaml:"require_digit,omitempty"`  // Require digits
	RequireSymbol bool `yaml:"require_symbol,omitempty"` // Require symbols
}

// EnvironmentPolicy defines per-environment restrictions
type EnvironmentPolicy struct {
	AllowedProviders []string `yaml:"allowed_providers,omitempty"` // Environment-specific provider whitelist
	BlockedProviders []string `yaml:"blocked_providers,omitempty"` // Environment-specific provider blacklist
	RequireApproval  bool     `yaml:"require_approval,omitempty"`  // Require manual approval for this env
	MaxSecrets       int      `yaml:"max_secrets,omitempty"`       // Maximum number of secrets allowed
}

// OutputPolicy defines file output restrictions
type OutputPolicy struct {
	AllowedPaths     []string `yaml:"allowed_paths,omitempty"`     // Whitelist of allowed output paths
	BlockedPaths     []string `yaml:"blocked_paths,omitempty"`     // Blacklist of blocked output paths
	RequireGitignore bool     `yaml:"require_gitignore,omitempty"` // Require output files to be in .gitignore
	MaxTTL           int      `yaml:"max_ttl,omitempty"`           // Maximum TTL for output files (seconds)
}

// AuditPolicy defines audit logging requirements
type AuditPolicy struct {
	Enabled      bool   `yaml:"enabled,omitempty"`       // Enable audit logging
	LogPath      string `yaml:"log_path,omitempty"`      // Path to audit log file
	LogLevel     string `yaml:"log_level,omitempty"`     // Audit log level (info, warn, error)
	IncludeValues bool  `yaml:"include_values,omitempty"` // Include secret values in audit log (NOT RECOMMENDED)
}

// PolicyEnforcer validates operations against configured policies
type PolicyEnforcer struct {
	config *PolicyConfig
}

// NewPolicyEnforcer creates a new policy enforcer
func NewPolicyEnforcer(config *PolicyConfig) *PolicyEnforcer {
	if config == nil {
		config = &PolicyConfig{} // Empty config = no restrictions
	}
	return &PolicyEnforcer{config: config}
}

// ValidateProviderType checks if a provider type is allowed
func (pe *PolicyEnforcer) ValidateProviderType(providerType string) error {
	// Check blocked providers first
	for _, blocked := range pe.config.BlockedProviders {
		if strings.EqualFold(blocked, providerType) {
			return dserrors.UserError{
				Message:    fmt.Sprintf("Provider type '%s' is blocked by policy", providerType),
				Suggestion: "Use an allowed provider type or update your policy configuration",
			}
		}
	}
	
	// Check allowed providers (if specified)
	if len(pe.config.AllowedProviders) > 0 {
		allowed := false
		for _, allowedType := range pe.config.AllowedProviders {
			if strings.EqualFold(allowedType, providerType) {
				allowed = true
				break
			}
		}
		if !allowed {
			return dserrors.UserError{
				Message:    fmt.Sprintf("Provider type '%s' is not in allowed list", providerType),
				Suggestion: fmt.Sprintf("Allowed providers: %s", strings.Join(pe.config.AllowedProviders, ", ")),
			}
		}
	}
	
	return nil
}

// ValidateEnvironmentProvider checks provider usage for specific environment
func (pe *PolicyEnforcer) ValidateEnvironmentProvider(envName, providerType string) error {
	envPolicy, exists := pe.config.EnvironmentRules[envName]
	if !exists {
		return nil // No specific rules for this environment
	}
	
	// Check environment-specific blocked providers
	for _, blocked := range envPolicy.BlockedProviders {
		if strings.EqualFold(blocked, providerType) {
			return dserrors.UserError{
				Message:    fmt.Sprintf("Provider '%s' is blocked for environment '%s'", providerType, envName),
				Suggestion: "Use a different provider or update environment policy",
			}
		}
	}
	
	// Check environment-specific allowed providers
	if len(envPolicy.AllowedProviders) > 0 {
		allowed := false
		for _, allowedType := range envPolicy.AllowedProviders {
			if strings.EqualFold(allowedType, providerType) {
				allowed = true
				break
			}
		}
		if !allowed {
			return dserrors.UserError{
				Message:    fmt.Sprintf("Provider '%s' is not allowed for environment '%s'", providerType, envName),
				Suggestion: fmt.Sprintf("Allowed providers for %s: %s", envName, strings.Join(envPolicy.AllowedProviders, ", ")),
			}
		}
	}
	
	return nil
}

// ValidateSecretValue checks if a secret value meets policy requirements
func (pe *PolicyEnforcer) ValidateSecretValue(secretValue string) error {
	if pe.config.SecretComplexity != nil {
		if err := pe.validateComplexity(secretValue); err != nil {
			return err
		}
	}
	
	// Check forbidden patterns
	for _, pattern := range pe.config.ForbiddenPatterns {
		if matched, _ := regexp.MatchString(pattern, secretValue); matched {
			return dserrors.UserError{
				Message:    "Secret value matches forbidden pattern",
				Suggestion: "Use a different secret value that doesn't match restricted patterns",
			}
		}
	}
	
	// Check required patterns
	for _, pattern := range pe.config.RequiredPatterns {
		if matched, _ := regexp.MatchString(pattern, secretValue); !matched {
			return dserrors.UserError{
				Message:    "Secret value doesn't match required pattern",
				Suggestion: "Ensure secret value meets the required format",
			}
		}
	}
	
	return nil
}

// ValidateOutputPath checks if output path is allowed
func (pe *PolicyEnforcer) ValidateOutputPath(outputPath string) error {
	if pe.config.OutputRestrictions == nil {
		return nil
	}
	
	restrictions := pe.config.OutputRestrictions
	
	// Check blocked paths
	for _, blocked := range restrictions.BlockedPaths {
		if matched, _ := regexp.MatchString(blocked, outputPath); matched {
			return dserrors.UserError{
				Message:    fmt.Sprintf("Output path '%s' matches blocked pattern", outputPath),
				Suggestion: "Choose a different output path or update policy configuration",
			}
		}
	}
	
	// Check allowed paths
	if len(restrictions.AllowedPaths) > 0 {
		allowed := false
		for _, allowedPattern := range restrictions.AllowedPaths {
			if matched, _ := regexp.MatchString(allowedPattern, outputPath); matched {
				allowed = true
				break
			}
		}
		if !allowed {
			return dserrors.UserError{
				Message:    fmt.Sprintf("Output path '%s' is not in allowed patterns", outputPath),
				Suggestion: fmt.Sprintf("Use paths matching: %s", strings.Join(restrictions.AllowedPaths, ", ")),
			}
		}
	}
	
	return nil
}

// ValidateEnvironmentSecretCount checks secret count limits
func (pe *PolicyEnforcer) ValidateEnvironmentSecretCount(envName string, secretCount int) error {
	envPolicy, exists := pe.config.EnvironmentRules[envName]
	if !exists || envPolicy.MaxSecrets == 0 {
		return nil
	}
	
	if secretCount > envPolicy.MaxSecrets {
		return dserrors.UserError{
			Message:    fmt.Sprintf("Environment '%s' has %d secrets, exceeding limit of %d", envName, secretCount, envPolicy.MaxSecrets),
			Suggestion: "Reduce the number of secrets or increase the limit in policy configuration",
		}
	}
	
	return nil
}

// ShouldAudit returns whether an operation should be audited
func (pe *PolicyEnforcer) ShouldAudit() bool {
	return pe.config.AuditLogging != nil && pe.config.AuditLogging.Enabled
}

// GetAuditConfig returns audit configuration
func (pe *PolicyEnforcer) GetAuditConfig() *AuditPolicy {
	return pe.config.AuditLogging
}

func (pe *PolicyEnforcer) validateComplexity(value string) error {
	complexity := pe.config.SecretComplexity
	
	if complexity.MinLength > 0 && len(value) < complexity.MinLength {
		return dserrors.UserError{
			Message:    fmt.Sprintf("Secret must be at least %d characters", complexity.MinLength),
			Suggestion: "Use a longer secret value",
		}
	}
	
	if complexity.MaxLength > 0 && len(value) > complexity.MaxLength {
		return dserrors.UserError{
			Message:    fmt.Sprintf("Secret must not exceed %d characters", complexity.MaxLength),
			Suggestion: "Use a shorter secret value",
		}
	}
	
	if complexity.RequireUpper && !regexp.MustCompile(`[A-Z]`).MatchString(value) {
		return dserrors.UserError{
			Message:    "Secret must contain uppercase letters",
			Suggestion: "Include at least one uppercase letter in the secret",
		}
	}
	
	if complexity.RequireLower && !regexp.MustCompile(`[a-z]`).MatchString(value) {
		return dserrors.UserError{
			Message:    "Secret must contain lowercase letters", 
			Suggestion: "Include at least one lowercase letter in the secret",
		}
	}
	
	if complexity.RequireDigit && !regexp.MustCompile(`[0-9]`).MatchString(value) {
		return dserrors.UserError{
			Message:    "Secret must contain digits",
			Suggestion: "Include at least one digit in the secret",
		}
	}
	
	if complexity.RequireSymbol && !regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(value) {
		return dserrors.UserError{
			Message:    "Secret must contain symbols",
			Suggestion: "Include at least one symbol in the secret",
		}
	}
	
	return nil
}