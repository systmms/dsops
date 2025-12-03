package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	dserrors "github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/pkg/rotation"
)

func NewSecretsRotateCommand(cfg *config.Config) *cobra.Command {
	var (
		envName    string
		keys       []string
		strategy   string
		newValue   string
		dryRun     bool
		force      bool
		notify     []string
		onConflict string
	)

	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate secret values (passwords, API keys, certificates)",
		Long: `Rotate secret values by generating new passwords, API keys, or certificates.

IMPORTANT: This rotates actual secret VALUES (like database passwords), 
not encryption keys. For file encryption key rotation, use SOPS instead.

This command creates new secret values and updates them in the target systems
(databases, API providers, etc.) using the specified rotation strategy.

Rotation strategies:
  postgres         Rotate PostgreSQL user password
  mysql           Rotate MySQL user password  
  mongodb         Rotate MongoDB user password
  stripe          Rotate Stripe API key
  github          Rotate GitHub personal access token
  certificate     Rotate TLS certificate
  generic         Generic rotation using custom script
  random          Just generate a random value (for testing)

Examples:
  # Rotate database password using PostgreSQL strategy
  dsops secrets rotate --env production --key DATABASE_PASSWORD --strategy postgres

  # Rotate multiple secrets with dry run
  dsops secrets rotate --env production --key API_KEY,JWT_SECRET --strategy random --dry-run

  # Force rotation regardless of schedule
  dsops secrets rotate --env production --key STRIPE_KEY --strategy stripe --force

  # Rotate with custom value from file
  dsops secrets rotate --env production --key TLS_CERT --strategy certificate --new-value file:./new-cert.pem`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if envName == "" {
				return dserrors.UserError{
					Message:    "Environment name is required",
					Suggestion: "Specify environment with --env <name>",
					Details:    "Use --env to specify which environment contains the secrets to rotate",
				}
			}

			if len(keys) == 0 {
				return dserrors.UserError{
					Message:    "At least one secret key is required",
					Suggestion: "Specify secrets with --key KEY1,KEY2 or --key KEY1 --key KEY2",
					Details:    "Specify which secrets to rotate",
				}
			}

			if strategy == "" {
				return dserrors.UserError{
					Message:    "Rotation strategy is required",
					Suggestion: "Specify strategy with --strategy postgres, --strategy stripe, etc.",
					Details:    "The rotation strategy determines how the secret value is updated",
				}
			}

			return runSecretsRotate(cfg, envName, keys, strategy, newValue, dryRun, force, notify, onConflict)
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment name (required)")
	cmd.Flags().StringSliceVar(&keys, "key", nil, "Secret key(s) to rotate (required)")
	cmd.Flags().StringVar(&strategy, "strategy", "", "Rotation strategy (required)")
	cmd.Flags().StringVar(&newValue, "new-value", "", "New value specification (optional, strategy-dependent)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be rotated without making changes")
	cmd.Flags().BoolVar(&force, "force", false, "Force rotation even if recently rotated")
	cmd.Flags().StringSliceVar(&notify, "notify", nil, "Notification channels: slack,github,webhook")
	cmd.Flags().StringVar(&onConflict, "on-conflict", "fail", "Behavior on conflict: fail (default), skip, rollback")

	_ = cmd.MarkFlagRequired("env")
	_ = cmd.MarkFlagRequired("key")
	_ = cmd.MarkFlagRequired("strategy")

	return cmd
}

func runSecretsRotate(cfg *config.Config, envName string, keys []string, strategy, newValueSpec string, dryRun, force bool, notifyChannels []string, onConflict string) error {
	// Validate onConflict value
	switch onConflict {
	case "fail", "skip", "rollback":
		// Valid values
	default:
		return dserrors.UserError{
			Message:    fmt.Sprintf("Invalid --on-conflict value: %s", onConflict),
			Suggestion: "Valid values are: fail, skip, rollback",
			Details:    "fail: stop on any error (default), skip: skip conflicting secrets, rollback: attempt rollback on failure",
		}
	}
	if err := cfg.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get environment definition
	env, exists := cfg.Definition.Envs[envName]
	if !exists {
		return dserrors.UserError{
			Message:    fmt.Sprintf("Environment '%s' not found", envName),
			Suggestion: fmt.Sprintf("Available environments: %s", strings.Join(getSecretsEnvNames(cfg.Definition.Envs), ", ")),
			Details:    "Check your dsops.yaml for available environment names",
		}
	}

	logger := cfg.Logger
	providerRegistry := providers.NewRegistry()

	// Create provider instances
	providerInstances, err := createSecretsProviderInstances(cfg.ListAllProviders(), providerRegistry)
	if err != nil {
		return fmt.Errorf("failed to create providers: %w", err)
	}

	// Create rotation engine and strategy registry
	rotationEngine := rotation.NewRotationEngine(logger)
	strategyRegistry := rotation.NewStrategyRegistry(logger)

	// Register available strategies
	for _, strategyName := range strategyRegistry.ListStrategies() {
		rotationStrategy, err := strategyRegistry.CreateStrategy(strategyName)
		if err != nil {
			logger.Warn("Failed to create strategy %s: %v", strategyName, err)
			continue
		}
		if err := rotationEngine.RegisterStrategy(rotationStrategy); err != nil {
			logger.Warn("Failed to register strategy %s: %v", strategyName, err)
		}
	}

	// Process each key
	var rotationResults []rotation.RotationResult
	ctx := context.Background()
	for _, key := range keys {
		result, err := rotateSecretValueWithEngine(ctx, env, key, strategy, newValueSpec, providerInstances, rotationEngine, logger, dryRun, force)
		if err != nil {
			logger.Error("Failed to rotate secret %s: %v", logging.Secret(key), err)
			result = rotation.RotationResult{
				Secret: rotation.SecretInfo{Key: key},
				Status: rotation.StatusFailed,
				Error:  err.Error(),
			}
		}
		rotationResults = append(rotationResults, result)
	}

	// Display results
	return displayEngineRotationResults(rotationResults, logger)
}

type SecretRotationResult struct {
	Key            string     `json:"key"`
	Provider       string     `json:"provider"`
	Status         string     `json:"status"` // "success", "failed", "skipped", "not_supported"
	Strategy       string     `json:"strategy"`
	NewVersion     string     `json:"new_version,omitempty"`
	OldVersion     string     `json:"old_version,omitempty"`
	RotatedAt      *time.Time `json:"rotated_at,omitempty"`
	Error          string     `json:"error,omitempty"`
	DryRun         bool       `json:"dry_run"`
	Reason         string     `json:"reason,omitempty"`
}

func rotateSecretValueWithEngine(ctx context.Context, env config.Environment, key, strategy, newValueSpec string, providers map[string]provider.Provider, engine rotation.RotationEngine, logger *logging.Logger, dryRun, force bool) (rotation.RotationResult, error) {
	// Find the variable definition
	varDef, exists := env[key]
	if !exists {
		return rotation.RotationResult{}, dserrors.UserError{
			Message:    fmt.Sprintf("Variable '%s' not found in environment", key),
			Suggestion: "Check the variable name and environment definition",
			Details:    "The variable must be defined in the environment to be rotated",
		}
	}

	// Get provider reference
	if varDef.From == nil {
		return rotation.RotationResult{
			Secret: rotation.SecretInfo{Key: key},
			Status: rotation.StatusPending,
			Error:  "Variable uses literal value, not a provider",
		}, nil
	}

	providerName := varDef.From.Provider
	_, exists = providers[providerName]
	if !exists {
		return rotation.RotationResult{}, dserrors.UserError{
			Message:    fmt.Sprintf("Provider '%s' not found", providerName),
			Suggestion: "Check provider configuration in dsops.yaml",
			Details:    "The provider must be configured to perform rotation",
		}
	}

	// Create secret info for the rotation engine
	secretInfo := rotation.SecretInfo{
		Key:         key,
		Provider:    providerName,
		ProviderRef: provider.Reference{
			Provider: providerName,
			Key:      varDef.From.Key,
			Version:  varDef.From.Version,
		},
		SecretType: inferSecretType(key, strategy, varDef),
		Metadata:   extractMetadata(varDef),
		Constraints: &rotation.RotationConstraints{
			MinRotationInterval: 1 * time.Hour, // Default constraints
			GracePeriod:         5 * time.Minute,
		},
	}

	// Parse new value specification
	var newValue *rotation.NewSecretValue
	if newValueSpec != "" {
		newValue = &rotation.NewSecretValue{
			Type: rotation.ValueTypeRandom,
			Config: map[string]interface{}{
				"length": 32,
			},
		}
		
		// Parse the new value spec (simplified for now)
		if strings.HasPrefix(newValueSpec, "literal:") {
			newValue.Type = rotation.ValueTypeLiteral
			newValue.Value = strings.TrimPrefix(newValueSpec, "literal:")
		}
	}

	// Create rotation request
	request := rotation.RotationRequest{
		Secret:   secretInfo,
		Strategy: strategy,
		NewValue: newValue,
		DryRun:   dryRun,
		Force:    force,
		Config:   make(map[string]interface{}),
	}

	// Use the rotation engine
	result, err := engine.Rotate(ctx, request)
	if err != nil {
		return rotation.RotationResult{}, err
	}
	return *result, nil
}


func displayEngineRotationResults(results []rotation.RotationResult, logger *logging.Logger) error {
	logger.Info("\nSecret Value Rotation Results (New Engine):")
	logger.Info("==========================================")

	var successCount, failedCount, skippedCount int

	for _, result := range results {
		switch result.Status {
		case rotation.StatusCompleted:
			successCount++
			if result.RotatedAt != nil {
				logger.Info("✓ %s (rotated at %s)", result.Secret.Key, result.RotatedAt.Format("2006-01-02 15:04:05"))
			} else {
				logger.Info("✓ %s (rotation completed)", result.Secret.Key)
			}
		case rotation.StatusPending:
			if result.Error != "" {
				skippedCount++
				logger.Warn("○ %s: %s", result.Secret.Key, result.Error)
			} else {
				successCount++
				logger.Info("✓ %s (would rotate - dry run)", result.Secret.Key)
			}
		case rotation.StatusFailed:
			failedCount++
			logger.Error("✗ %s: %s", result.Secret.Key, result.Error)
		default:
			logger.Info("? %s: unknown status %s", result.Secret.Key, result.Status)
		}
	}

	logger.Info("\nSummary:")
	if successCount > 0 {
		logger.Info("  Successfully processed: %d", successCount)
	}
	if skippedCount > 0 {
		logger.Info("  Skipped: %d", skippedCount)
	}
	if failedCount > 0 {
		logger.Info("  Failed: %d", failedCount)
		return fmt.Errorf("%d secret(s) failed to rotate", failedCount)
	}

	return nil
}


func getSecretsEnvNames(envs map[string]config.Environment) []string {
	names := make([]string, 0, len(envs))
	for name := range envs {
		names = append(names, name)
	}
	return names
}

func createSecretsProviderInstances(providerConfigs map[string]config.ProviderConfig, registry *providers.Registry) (map[string]provider.Provider, error) {
	instances := make(map[string]provider.Provider)
	
	for name, providerConfig := range providerConfigs {
		instance, err := registry.CreateProvider(name, providerConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider %s: %w", name, err)
		}
		instances[name] = instance
	}
	
	return instances, nil
}

// inferSecretType determines the secret type based on strategy and key name
func inferSecretType(key, strategy string, varDef config.Variable) rotation.SecretType {
	// Strategy-based inference
	switch strategy {
	case "postgres", "mysql", "mongodb":
		return rotation.SecretTypePassword
	case "stripe", "github":
		return rotation.SecretTypeAPIKey  
	case "certificate":
		return rotation.SecretTypeCertificate
	case "random":
		return rotation.SecretTypeGeneric
	}
	
	// Key name-based inference
	keyLower := strings.ToLower(key)
	if strings.Contains(keyLower, "password") || strings.Contains(keyLower, "pass") {
		return rotation.SecretTypePassword
	}
	if strings.Contains(keyLower, "api_key") || strings.Contains(keyLower, "token") {
		return rotation.SecretTypeAPIKey
	}
	if strings.Contains(keyLower, "cert") || strings.Contains(keyLower, "certificate") {
		return rotation.SecretTypeCertificate
	}
	
	return rotation.SecretTypeGeneric
}

// extractMetadata extracts metadata from variable definition for rotation
func extractMetadata(varDef config.Variable) map[string]string {
	if varDef.Metadata != nil {
		return varDef.Metadata
	}
	return make(map[string]string)
}