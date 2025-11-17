package rotation

import (
	"context"
	"testing"
	"time"
)

func TestProviderCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("AWSIAMCapabilities", func(t *testing.T) {
		t.Parallel()

		caps := AWSIAMCapabilities

		if caps.MaxActiveKeys != 2 {
			t.Errorf("MaxActiveKeys = %d, want 2", caps.MaxActiveKeys)
		}
		if caps.SupportsVersioning {
			t.Error("AWS IAM should not support versioning")
		}
		if !caps.SupportsRevocation {
			t.Error("AWS IAM should support revocation")
		}
		if !containsStrategy(caps.RecommendedStrategies, StrategyTwoKey) {
			t.Error("AWS IAM should recommend two-key strategy")
		}
		if !containsStrategy(caps.RecommendedStrategies, StrategyEmergency) {
			t.Error("AWS IAM should recommend emergency strategy")
		}
	})

	t.Run("AWSSecretsManagerCapabilities", func(t *testing.T) {
		t.Parallel()

		caps := AWSSecretsManagerCapabilities

		if caps.MaxActiveKeys != -1 {
			t.Errorf("MaxActiveKeys = %d, want -1 (unlimited)", caps.MaxActiveKeys)
		}
		if !caps.SupportsVersioning {
			t.Error("AWS Secrets Manager should support versioning")
		}
		if !caps.SupportsExpiration {
			t.Error("AWS Secrets Manager should support expiration")
		}
		if caps.MinRotationInterval != 1*time.Hour {
			t.Errorf("MinRotationInterval = %v, want 1h", caps.MinRotationInterval)
		}
		if !containsStrategy(caps.RecommendedStrategies, StrategyVersioned) {
			t.Error("AWS Secrets Manager should recommend versioned strategy")
		}
	})

	t.Run("AzureServicePrincipalCapabilities", func(t *testing.T) {
		t.Parallel()

		caps := AzureServicePrincipalCapabilities

		if caps.MaxActiveKeys != -1 {
			t.Errorf("MaxActiveKeys = %d, want -1", caps.MaxActiveKeys)
		}
		if !caps.SupportsExpiration {
			t.Error("Azure should support expiration")
		}
		if !caps.SupportsRevocation {
			t.Error("Azure should support revocation")
		}
		if len(caps.RecommendedStrategies) != 3 {
			t.Errorf("Expected 3 recommended strategies, got %d", len(caps.RecommendedStrategies))
		}
	})

	t.Run("GitHubPATCapabilities", func(t *testing.T) {
		t.Parallel()

		caps := GitHubPATCapabilities

		if !caps.SupportsExpiration {
			t.Error("GitHub PAT should support expiration")
		}
		if !caps.SupportsRevocation {
			t.Error("GitHub PAT should support revocation")
		}
		if !containsStrategy(caps.RecommendedStrategies, StrategyImmediate) {
			t.Error("GitHub PAT should recommend immediate strategy")
		}
	})

	t.Run("StripeAPIKeyCapabilities", func(t *testing.T) {
		t.Parallel()

		caps := StripeAPIKeyCapabilities

		if caps.MaxActiveKeys != -1 {
			t.Errorf("MaxActiveKeys = %d, want -1", caps.MaxActiveKeys)
		}
		if !caps.SupportsRevocation {
			t.Error("Stripe should support revocation")
		}
		if caps.SupportsExpiration {
			t.Error("Stripe should not support expiration")
		}
	})

	t.Run("DatadogAPIKeyCapabilities", func(t *testing.T) {
		t.Parallel()

		caps := DatadogAPIKeyCapabilities

		if caps.MaxActiveKeys != 50 {
			t.Errorf("MaxActiveKeys = %d, want 50", caps.MaxActiveKeys)
		}
		if !caps.SupportsRevocation {
			t.Error("Datadog should support revocation")
		}
	})

	t.Run("OktaTokenCapabilities", func(t *testing.T) {
		t.Parallel()

		caps := OktaTokenCapabilities

		if !caps.SupportsExpiration {
			t.Error("Okta should support expiration")
		}
		if !caps.SupportsRevocation {
			t.Error("Okta should support revocation")
		}
		if caps.SupportsMetadata {
			t.Error("Okta should not support metadata")
		}
		if !containsStrategy(caps.RecommendedStrategies, StrategyImmediate) {
			t.Error("Okta should recommend immediate strategy")
		}
	})
}

func TestRotationStrategyConstants(t *testing.T) {
	t.Parallel()

	t.Run("strategy names", func(t *testing.T) {
		t.Parallel()

		if StrategyTwoKey != "two-key" {
			t.Errorf("StrategyTwoKey = %s, want two-key", StrategyTwoKey)
		}
		if StrategyVersioned != "versioned" {
			t.Errorf("StrategyVersioned = %s, want versioned", StrategyVersioned)
		}
		if StrategyImmediate != "immediate" {
			t.Errorf("StrategyImmediate = %s, want immediate", StrategyImmediate)
		}
		if StrategyOAuthRefresh != "oauth-refresh" {
			t.Errorf("StrategyOAuthRefresh = %s, want oauth-refresh", StrategyOAuthRefresh)
		}
		if StrategyOverlap != "overlap" {
			t.Errorf("StrategyOverlap = %s, want overlap", StrategyOverlap)
		}
		if StrategyEmergency != "emergency" {
			t.Errorf("StrategyEmergency = %s, want emergency", StrategyEmergency)
		}
	})
}

func TestDefaultStrategySelectorSelectStrategy(t *testing.T) {
	t.Parallel()

	selector := &DefaultStrategySelector{}
	ctx := context.Background()

	t.Run("compromised secret uses emergency", func(t *testing.T) {
		t.Parallel()

		secret := SecretInfo{
			Metadata: map[string]string{
				"compromised": "true",
			},
		}

		caps := AWSIAMCapabilities
		strategy, err := selector.SelectStrategy(ctx, secret, caps)
		if err != nil {
			t.Fatalf("SelectStrategy() failed: %v", err)
		}
		if strategy != StrategyEmergency {
			t.Errorf("Strategy = %s, want emergency for compromised secret", strategy)
		}
	})

	t.Run("two-key when supported and recommended", func(t *testing.T) {
		t.Parallel()

		secret := SecretInfo{}
		caps := AWSIAMCapabilities // MaxActiveKeys=2, recommends TwoKey

		strategy, err := selector.SelectStrategy(ctx, secret, caps)
		if err != nil {
			t.Fatalf("SelectStrategy() failed: %v", err)
		}
		if strategy != StrategyTwoKey {
			t.Errorf("Strategy = %s, want two-key", strategy)
		}
	})

	t.Run("versioned when available", func(t *testing.T) {
		t.Parallel()

		secret := SecretInfo{}
		caps := ProviderCapabilities{
			MaxActiveKeys:         1, // Can't do two-key
			SupportsVersioning:    true,
			RecommendedStrategies: []RotationStrategy{StrategyVersioned},
		}

		strategy, err := selector.SelectStrategy(ctx, secret, caps)
		if err != nil {
			t.Fatalf("SelectStrategy() failed: %v", err)
		}
		if strategy != StrategyVersioned {
			t.Errorf("Strategy = %s, want versioned", strategy)
		}
	})

	t.Run("overlap when expiration supported", func(t *testing.T) {
		t.Parallel()

		secret := SecretInfo{}
		caps := ProviderCapabilities{
			MaxActiveKeys:         1,
			SupportsVersioning:    false,
			SupportsExpiration:    true,
			RecommendedStrategies: []RotationStrategy{StrategyOverlap},
		}

		strategy, err := selector.SelectStrategy(ctx, secret, caps)
		if err != nil {
			t.Fatalf("SelectStrategy() failed: %v", err)
		}
		if strategy != StrategyOverlap {
			t.Errorf("Strategy = %s, want overlap", strategy)
		}
	})

	t.Run("immediate as fallback", func(t *testing.T) {
		t.Parallel()

		secret := SecretInfo{}
		caps := ProviderCapabilities{
			MaxActiveKeys:         1,
			SupportsVersioning:    false,
			SupportsExpiration:    false,
			RecommendedStrategies: []RotationStrategy{StrategyImmediate},
		}

		strategy, err := selector.SelectStrategy(ctx, secret, caps)
		if err != nil {
			t.Fatalf("SelectStrategy() failed: %v", err)
		}
		if strategy != StrategyImmediate {
			t.Errorf("Strategy = %s, want immediate", strategy)
		}
	})

	t.Run("unlimited keys returns immediate without versioning", func(t *testing.T) {
		t.Parallel()

		secret := SecretInfo{}
		caps := ProviderCapabilities{
			MaxActiveKeys:         -1, // Unlimited, but -1 < 2 in Go
			RecommendedStrategies: []RotationStrategy{StrategyTwoKey},
		}

		// The code checks MaxActiveKeys >= 2, so -1 is not >= 2
		// Falls through to immediate since no versioning/expiration
		strategy, err := selector.SelectStrategy(ctx, secret, caps)
		if err != nil {
			t.Fatalf("SelectStrategy() failed: %v", err)
		}
		if strategy != StrategyImmediate {
			t.Errorf("Strategy = %s, want immediate (because -1 < 2)", strategy)
		}
	})

	t.Run("many keys supports two-key", func(t *testing.T) {
		t.Parallel()

		secret := SecretInfo{}
		caps := DatadogAPIKeyCapabilities // MaxActiveKeys=50

		strategy, err := selector.SelectStrategy(ctx, secret, caps)
		if err != nil {
			t.Fatalf("SelectStrategy() failed: %v", err)
		}
		if strategy != StrategyTwoKey {
			t.Errorf("Strategy = %s, want two-key for Datadog", strategy)
		}
	})

	t.Run("context is respected", func(t *testing.T) {
		t.Parallel()

		// Even with cancelled context, SelectStrategy should work (doesn't do I/O)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		secret := SecretInfo{}
		caps := AWSIAMCapabilities

		strategy, err := selector.SelectStrategy(ctx, secret, caps)
		if err != nil {
			t.Fatalf("SelectStrategy() failed: %v", err)
		}
		if strategy != StrategyTwoKey {
			t.Errorf("Strategy = %s, want two-key", strategy)
		}
	})
}

func TestDefaultStrategySelectorValidateStrategy(t *testing.T) {
	t.Parallel()

	selector := &DefaultStrategySelector{}

	t.Run("two-key valid with 2+ keys", func(t *testing.T) {
		t.Parallel()

		caps := AWSIAMCapabilities
		err := selector.ValidateStrategy(StrategyTwoKey, caps)
		if err != nil {
			t.Errorf("ValidateStrategy() failed: %v", err)
		}
	})

	t.Run("two-key invalid with 1 key", func(t *testing.T) {
		t.Parallel()

		caps := ProviderCapabilities{
			MaxActiveKeys: 1,
		}
		err := selector.ValidateStrategy(StrategyTwoKey, caps)
		if err == nil {
			t.Error("Expected error for two-key strategy with single key limit")
		}
	})

	t.Run("two-key valid with unlimited keys", func(t *testing.T) {
		t.Parallel()

		caps := ProviderCapabilities{
			MaxActiveKeys:         -1, // -1 signals unlimited
			RecommendedStrategies: []RotationStrategy{StrategyTwoKey},
		}
		err := selector.ValidateStrategy(StrategyTwoKey, caps)
		// The code specifically checks != -1 in the validation
		if err != nil {
			t.Errorf("ValidateStrategy() failed for unlimited keys: %v", err)
		}
	})

	t.Run("versioned invalid without support", func(t *testing.T) {
		t.Parallel()

		caps := ProviderCapabilities{
			SupportsVersioning: false,
		}
		err := selector.ValidateStrategy(StrategyVersioned, caps)
		if err == nil {
			t.Error("Expected error for versioned strategy without versioning support")
		}
	})

	t.Run("versioned valid with support", func(t *testing.T) {
		t.Parallel()

		caps := AWSSecretsManagerCapabilities
		err := selector.ValidateStrategy(StrategyVersioned, caps)
		if err != nil {
			t.Errorf("ValidateStrategy() failed: %v", err)
		}
	})

	t.Run("overlap invalid without expiration", func(t *testing.T) {
		t.Parallel()

		caps := ProviderCapabilities{
			SupportsExpiration: false,
		}
		err := selector.ValidateStrategy(StrategyOverlap, caps)
		if err == nil {
			t.Error("Expected error for overlap strategy without expiration support")
		}
	})

	t.Run("overlap valid with expiration", func(t *testing.T) {
		t.Parallel()

		caps := AWSSecretsManagerCapabilities
		err := selector.ValidateStrategy(StrategyOverlap, caps)
		if err != nil {
			t.Errorf("ValidateStrategy() failed: %v", err)
		}
	})

	t.Run("oauth-refresh always invalid", func(t *testing.T) {
		t.Parallel()

		caps := ProviderCapabilities{}
		err := selector.ValidateStrategy(StrategyOAuthRefresh, caps)
		if err == nil {
			t.Error("Expected error for OAuth refresh strategy")
		}
	})

	t.Run("emergency invalid without revocation", func(t *testing.T) {
		t.Parallel()

		caps := ProviderCapabilities{
			SupportsRevocation: false,
		}
		err := selector.ValidateStrategy(StrategyEmergency, caps)
		if err == nil {
			t.Error("Expected error for emergency strategy without revocation support")
		}
	})

	t.Run("emergency valid with revocation", func(t *testing.T) {
		t.Parallel()

		caps := AWSIAMCapabilities
		err := selector.ValidateStrategy(StrategyEmergency, caps)
		if err != nil {
			t.Errorf("ValidateStrategy() failed: %v", err)
		}
	})

	t.Run("strategy not recommended warning", func(t *testing.T) {
		t.Parallel()

		// GitHubPAT recommends Immediate, Overlap, Emergency - not TwoKey
		caps := GitHubPATCapabilities
		err := selector.ValidateStrategy(StrategyTwoKey, caps)
		// Should return warning (still an error)
		if err == nil {
			t.Error("Expected warning error for non-recommended strategy")
		}
		if err.Error() != "strategy two-key is not recommended for this provider" {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("immediate always valid", func(t *testing.T) {
		t.Parallel()

		// Immediate strategy has no special requirements
		caps := ProviderCapabilities{
			RecommendedStrategies: []RotationStrategy{StrategyImmediate},
		}
		err := selector.ValidateStrategy(StrategyImmediate, caps)
		if err != nil {
			t.Errorf("ValidateStrategy() failed for immediate: %v", err)
		}
	})
}

func TestProviderCapabilitiesStruct(t *testing.T) {
	t.Parallel()

	t.Run("custom capabilities", func(t *testing.T) {
		t.Parallel()

		caps := ProviderCapabilities{
			MaxActiveKeys:       10,
			SupportsVersioning:  true,
			SupportsExpiration:  true,
			SupportsRevocation:  true,
			SupportsMetadata:    true,
			MinRotationInterval: 24 * time.Hour,
			RecommendedStrategies: []RotationStrategy{
				StrategyTwoKey,
				StrategyVersioned,
				StrategyOverlap,
			},
		}

		if caps.MaxActiveKeys != 10 {
			t.Errorf("MaxActiveKeys = %d, want 10", caps.MaxActiveKeys)
		}
		if !caps.SupportsVersioning {
			t.Error("SupportsVersioning should be true")
		}
		if caps.MinRotationInterval != 24*time.Hour {
			t.Errorf("MinRotationInterval = %v, want 24h", caps.MinRotationInterval)
		}
		if len(caps.RecommendedStrategies) != 3 {
			t.Errorf("RecommendedStrategies length = %d, want 3", len(caps.RecommendedStrategies))
		}
	})

	t.Run("zero value capabilities", func(t *testing.T) {
		t.Parallel()

		var caps ProviderCapabilities

		if caps.MaxActiveKeys != 0 {
			t.Errorf("Zero value MaxActiveKeys = %d, want 0", caps.MaxActiveKeys)
		}
		if caps.SupportsVersioning {
			t.Error("Zero value SupportsVersioning should be false")
		}
		if caps.MinRotationInterval != 0 {
			t.Errorf("Zero value MinRotationInterval = %v, want 0", caps.MinRotationInterval)
		}
		if caps.RecommendedStrategies != nil {
			t.Error("Zero value RecommendedStrategies should be nil")
		}
	})
}

// Helper function
func containsStrategy(strategies []RotationStrategy, target RotationStrategy) bool {
	for _, s := range strategies {
		if s == target {
			return true
		}
	}
	return false
}
