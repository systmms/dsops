package rotation

import (
	"context"
	"fmt"
	"time"
)

// RotationStrategy defines the approach for rotating a secret
type RotationStrategy string

const (
	// StrategyTwoKey maintains two active keys for zero-downtime rotation
	StrategyTwoKey RotationStrategy = "two-key"
	
	// StrategyVersioned creates new versions while old remain accessible
	StrategyVersioned RotationStrategy = "versioned"
	
	// StrategyImmediate replaces immediately (may cause downtime)
	StrategyImmediate RotationStrategy = "immediate"
	
	// StrategyOAuthRefresh uses refresh tokens for rotation
	StrategyOAuthRefresh RotationStrategy = "oauth-refresh"
	
	// StrategyOverlap creates new with validity overlap period
	StrategyOverlap RotationStrategy = "overlap"
	
	// StrategyEmergency for compromised secrets (immediate revoke)
	StrategyEmergency RotationStrategy = "emergency"
)

// ProviderCapabilities describes what rotation features a provider supports
type ProviderCapabilities struct {
	// MaxActiveKeys is the maximum number of active keys (-1 for unlimited)
	MaxActiveKeys int
	
	// SupportsVersioning indicates if provider maintains version history
	SupportsVersioning bool
	
	// SupportsExpiration indicates if secrets can have expiration dates
	SupportsExpiration bool
	
	// SupportsRevocation indicates if secrets can be revoked immediately
	SupportsRevocation bool
	
	// SupportsMetadata indicates if secrets can have associated metadata
	SupportsMetadata bool
	
	// MinRotationInterval is the minimum time between rotations
	MinRotationInterval time.Duration
	
	// RecommendedStrategies lists strategies that work well with this provider
	RecommendedStrategies []RotationStrategy
}

// Common provider capabilities
var (
	// AWSIAMCapabilities for AWS IAM access keys
	AWSIAMCapabilities = ProviderCapabilities{
		MaxActiveKeys:       2,
		SupportsVersioning:  false,
		SupportsExpiration:  false,
		SupportsRevocation:  true,
		SupportsMetadata:    true,
		MinRotationInterval: 0,
		RecommendedStrategies: []RotationStrategy{
			StrategyTwoKey,
			StrategyEmergency,
		},
	}
	
	// AWSSecretsManagerCapabilities for AWS Secrets Manager
	AWSSecretsManagerCapabilities = ProviderCapabilities{
		MaxActiveKeys:       -1, // Unlimited versions
		SupportsVersioning:  true,
		SupportsExpiration:  true,
		SupportsRevocation:  false,
		SupportsMetadata:    true,
		MinRotationInterval: 1 * time.Hour,
		RecommendedStrategies: []RotationStrategy{
			StrategyVersioned,
			StrategyOverlap,
		},
	}
	
	// AzureServicePrincipalCapabilities for Azure AD
	AzureServicePrincipalCapabilities = ProviderCapabilities{
		MaxActiveKeys:       -1, // Multiple secrets allowed
		SupportsVersioning:  false,
		SupportsExpiration:  true,
		SupportsRevocation:  true,
		SupportsMetadata:    true,
		MinRotationInterval: 0,
		RecommendedStrategies: []RotationStrategy{
			StrategyTwoKey,
			StrategyOverlap,
			StrategyEmergency,
		},
	}
	
	// GitHubPATCapabilities for GitHub Personal Access Tokens
	GitHubPATCapabilities = ProviderCapabilities{
		MaxActiveKeys:       -1, // Multiple tokens but independent
		SupportsVersioning:  false,
		SupportsExpiration:  true,
		SupportsRevocation:  true,
		SupportsMetadata:    true,
		MinRotationInterval: 0,
		RecommendedStrategies: []RotationStrategy{
			StrategyImmediate,
			StrategyOverlap,
			StrategyEmergency,
		},
	}
	
	// StripeAPIKeyCapabilities for Stripe
	StripeAPIKeyCapabilities = ProviderCapabilities{
		MaxActiveKeys:       -1, // Multiple keys allowed
		SupportsVersioning:  false,
		SupportsExpiration:  false,
		SupportsRevocation:  true,
		SupportsMetadata:    true,
		MinRotationInterval: 0,
		RecommendedStrategies: []RotationStrategy{
			StrategyTwoKey,
			StrategyEmergency,
		},
	}
	
	// DatadogAPIKeyCapabilities for Datadog
	DatadogAPIKeyCapabilities = ProviderCapabilities{
		MaxActiveKeys:       50,
		SupportsVersioning:  false,
		SupportsExpiration:  false,
		SupportsRevocation:  true,
		SupportsMetadata:    true,
		MinRotationInterval: 0,
		RecommendedStrategies: []RotationStrategy{
			StrategyTwoKey,
			StrategyEmergency,
		},
	}
	
	// OktaTokenCapabilities for Okta (static tokens)
	OktaTokenCapabilities = ProviderCapabilities{
		MaxActiveKeys:       -1,
		SupportsVersioning:  false,
		SupportsExpiration:  true,
		SupportsRevocation:  true,
		SupportsMetadata:    false,
		MinRotationInterval: 0,
		RecommendedStrategies: []RotationStrategy{
			StrategyImmediate, // Okta recommends OAuth instead
			StrategyEmergency,
		},
	}
)

// StrategySelector helps choose the best rotation strategy
type StrategySelector interface {
	// SelectStrategy chooses the best strategy based on provider capabilities
	SelectStrategy(ctx context.Context, secret SecretInfo, capabilities ProviderCapabilities) (RotationStrategy, error)
	
	// ValidateStrategy checks if a strategy is compatible with provider
	ValidateStrategy(strategy RotationStrategy, capabilities ProviderCapabilities) error
}

// DefaultStrategySelector implements intelligent strategy selection
type DefaultStrategySelector struct{}

// SelectStrategy chooses the best rotation strategy
func (s *DefaultStrategySelector) SelectStrategy(ctx context.Context, secret SecretInfo, capabilities ProviderCapabilities) (RotationStrategy, error) {
	// Emergency rotation takes precedence
	if secret.Metadata["compromised"] == "true" {
		return StrategyEmergency, nil
	}
	
	// Check if two-key strategy is possible and recommended
	if capabilities.MaxActiveKeys >= 2 {
		for _, rec := range capabilities.RecommendedStrategies {
			if rec == StrategyTwoKey {
				return StrategyTwoKey, nil
			}
		}
	}
	
	// Use versioned if available
	if capabilities.SupportsVersioning {
		return StrategyVersioned, nil
	}
	
	// Use overlap if expiration is supported
	if capabilities.SupportsExpiration {
		return StrategyOverlap, nil
	}
	
	// Default to immediate replacement
	return StrategyImmediate, nil
}

// ValidateStrategy ensures strategy is compatible with provider
func (s *DefaultStrategySelector) ValidateStrategy(strategy RotationStrategy, capabilities ProviderCapabilities) error {
	switch strategy {
	case StrategyTwoKey:
		if capabilities.MaxActiveKeys < 2 && capabilities.MaxActiveKeys != -1 {
			return fmt.Errorf("provider only supports %d active keys, need at least 2 for two-key strategy", capabilities.MaxActiveKeys)
		}
		
	case StrategyVersioned:
		if !capabilities.SupportsVersioning {
			return fmt.Errorf("provider does not support versioning")
		}
		
	case StrategyOverlap:
		if !capabilities.SupportsExpiration {
			return fmt.Errorf("provider does not support expiration dates needed for overlap strategy")
		}
		
	case StrategyOAuthRefresh:
		// This would need specific OAuth support
		return fmt.Errorf("OAuth refresh strategy requires OAuth-specific implementation")
		
	case StrategyEmergency:
		if !capabilities.SupportsRevocation {
			return fmt.Errorf("provider does not support immediate revocation needed for emergency rotation")
		}
	}
	
	// Check if strategy is in recommended list
	recommended := false
	for _, rec := range capabilities.RecommendedStrategies {
		if rec == strategy {
			recommended = true
			break
		}
	}
	
	if !recommended {
		// Warning, not error
		return fmt.Errorf("strategy %s is not recommended for this provider", strategy)
	}
	
	return nil
}