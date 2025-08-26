package permissions

import (
	"context"
	"fmt"
	"time"
	
	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/rotation"
)

// PermissionChecker handles principal-based permission checking
type PermissionChecker struct {
	repository *dsopsdata.Repository
	logger     *logging.Logger
}

// NewPermissionChecker creates a new permission checker
func NewPermissionChecker(repository *dsopsdata.Repository, logger *logging.Logger) *PermissionChecker {
	return &PermissionChecker{
		repository: repository,
		logger:     logger,
	}
}

// RotationRequest represents a rotation permission check request
type RotationRequest struct {
	Principal       string                 // Principal name making the request
	ServiceType     string                 // Service type being rotated
	CredentialKind  string                 // Credential kind being rotated
	RequestedTTL    time.Duration          // Requested TTL for the credential
	Environment     string                 // Environment context
	SecretKey       string                 // Secret key for logging
}

// PermissionResult represents the result of a permission check
type PermissionResult struct {
	Allowed       bool   `json:"allowed"`
	Reason        string `json:"reason"`
	Principal     *dsopsdata.Principal `json:"principal,omitempty"`
	Constraints   []string `json:"constraints,omitempty"`
}

// CheckRotationPermission checks if a principal can perform a rotation
func (p *PermissionChecker) CheckRotationPermission(ctx context.Context, req RotationRequest) *PermissionResult {
	if p.repository == nil {
		p.logger.Debug("No repository available, allowing rotation by default")
		return &PermissionResult{
			Allowed: true,
			Reason:  "No permission system configured",
		}
	}

	// Get principal
	principal, exists := p.repository.GetPrincipal(req.Principal)
	if !exists {
		p.logger.Warn("Unknown principal %s requested rotation for %s", req.Principal, logging.Secret(req.SecretKey))
		return &PermissionResult{
			Allowed: false,
			Reason:  fmt.Sprintf("Unknown principal: %s", req.Principal),
		}
	}

	// Check if principal has permissions configured
	if principal.Spec.Permissions == nil {
		p.logger.Debug("Principal %s has no specific permissions, allowing rotation", req.Principal)
		return &PermissionResult{
			Allowed:   true,
			Reason:    "No specific permissions configured for principal",
			Principal: principal,
		}
	}

	permissions := principal.Spec.Permissions
	var constraints []string

	// Check allowed services
	if len(permissions.AllowedServices) > 0 {
		allowed := false
		for _, allowedService := range permissions.AllowedServices {
			if allowedService == req.ServiceType {
				allowed = true
				break
			}
		}
		if !allowed {
			p.logger.Warn("Principal %s not allowed to rotate %s (service type not in allowed list)", req.Principal, req.ServiceType)
			return &PermissionResult{
				Allowed:   false,
				Reason:    fmt.Sprintf("Service type %s not in allowed services", req.ServiceType),
				Principal: principal,
			}
		}
	}

	// Check allowed credential kinds
	if len(permissions.AllowedCredentialKinds) > 0 {
		allowed := false
		for _, allowedKind := range permissions.AllowedCredentialKinds {
			if allowedKind == req.CredentialKind {
				allowed = true
				break
			}
		}
		if !allowed {
			p.logger.Warn("Principal %s not allowed to rotate credential kind %s", req.Principal, req.CredentialKind)
			return &PermissionResult{
				Allowed:   false,
				Reason:    fmt.Sprintf("Credential kind %s not in allowed kinds", req.CredentialKind),
				Principal: principal,
			}
		}
	}

	// Check maximum TTL
	if permissions.MaxCredentialTTL != "" && req.RequestedTTL > 0 {
		maxTTL, err := time.ParseDuration(permissions.MaxCredentialTTL)
		if err != nil {
			p.logger.Warn("Invalid maxCredentialTTL format for principal %s: %v", req.Principal, err)
			constraints = append(constraints, fmt.Sprintf("Invalid maxCredentialTTL format: %s", permissions.MaxCredentialTTL))
		} else if req.RequestedTTL > maxTTL {
			p.logger.Warn("Principal %s requested TTL %v exceeds maximum %v", req.Principal, req.RequestedTTL, maxTTL)
			return &PermissionResult{
				Allowed:   false,
				Reason:    fmt.Sprintf("Requested TTL %v exceeds maximum allowed %v", req.RequestedTTL, maxTTL),
				Principal: principal,
			}
		} else {
			constraints = append(constraints, fmt.Sprintf("TTL limited to %v", maxTTL))
		}
	}

	// Check environment context if principal has environment restrictions
	if principal.Spec.Environment != "" && req.Environment != "" {
		if principal.Spec.Environment != req.Environment {
			// Allow more flexible matching - check if environment is in metadata
			if envs, exists := principal.Spec.Metadata["environments"]; exists {
				if envList, isList := envs.([]interface{}); isList {
					found := false
					for _, env := range envList {
						if envStr, isString := env.(string); isString && envStr == req.Environment {
							found = true
							break
						}
					}
					if !found {
						p.logger.Warn("Principal %s not allowed in environment %s", req.Principal, req.Environment)
						return &PermissionResult{
							Allowed:   false,
							Reason:    fmt.Sprintf("Environment %s not allowed for principal", req.Environment),
							Principal: principal,
						}
					}
				}
			} else {
				p.logger.Warn("Principal %s environment mismatch: expected %s, got %s", req.Principal, principal.Spec.Environment, req.Environment)
				return &PermissionResult{
					Allowed:   false,
					Reason:    fmt.Sprintf("Environment mismatch: principal limited to %s", principal.Spec.Environment),
					Principal: principal,
				}
			}
		}
	}

	p.logger.Info("Principal %s authorized for rotation of %s:%s", req.Principal, req.ServiceType, req.CredentialKind)

	return &PermissionResult{
		Allowed:     true,
		Reason:      "Permission granted",
		Principal:   principal,
		Constraints: constraints,
	}
}

// GetPrincipalForRotation attempts to determine the principal for a rotation request
func (p *PermissionChecker) GetPrincipalForRotation(ctx context.Context, secret rotation.SecretInfo) string {
	// Check if principal is specified in metadata
	if principal, exists := secret.Metadata["principal"]; exists {
		return principal
	}

	// Check if it's in the config (from service instance)
	// This would be set by the rotation engine when it enhances the request
	// For now, return a default or empty
	return ""
}