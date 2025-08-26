package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/internal/resolve"
	"github.com/systmms/dsops/internal/secretstores"
	"github.com/systmms/dsops/internal/services"
	"github.com/systmms/dsops/pkg/adapter"
)

func NewPlanCommand(cfg *config.Config) *cobra.Command {
	var (
		envName     string
		outputJSON  bool
		dataDir     string
	)

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show what secrets will be resolved (no values shown)",
		Long: `Plan shows which variables will be resolved and from which sources, 
without fetching actual secret values. This is useful for debugging 
configuration and verifying provider connectivity.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			if err := cfg.Load(); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create resolver
			resolver := resolve.New(cfg)

			// Register built-in providers based on config
			if err := registerProviders(resolver, cfg, dataDir); err != nil {
				return fmt.Errorf("failed to register providers: %w", err)
			}

			// Run the plan
			ctx := context.Background()
			result, err := resolver.Plan(ctx, envName)
			if err != nil {
				return fmt.Errorf("failed to plan: %w", err)
			}

			// Output results
			if outputJSON {
				return outputPlanJSON(result)
			}
			return outputPlanTable(result, cfg, envName)
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment name to plan (required)")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")
	cmd.Flags().StringVar(&dataDir, "data-dir", "./dsops-data", "Path to dsops-data repository (optional)")
	_ = cmd.MarkFlagRequired("env")

	return cmd
}

// registerProviders registers providers based on the configuration using new split registries
func registerProviders(resolver *resolve.Resolver, cfg *config.Config, dataDir string) error {
	if cfg.Definition == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Create all registries
	secretStoreRegistry := secretstores.NewRegistry()
	
	// Try to load dsops-data if directory exists
	var serviceRegistry *services.Registry
	if dataDir != "" {
		if _, err := os.Stat(dataDir); err == nil {
			cfg.Logger.Debug("Loading dsops-data from %s", dataDir)
			loader := dsopsdata.NewLoader(dataDir)
			repository, err := loader.LoadAll(context.Background())
			if err != nil {
				cfg.Logger.Warn("Failed to load dsops-data from %s: %v", dataDir, err)
				cfg.Logger.Debug("Falling back to hardcoded service registry")
				serviceRegistry = services.NewRegistry()
			} else {
				// Validate the repository
				if err := repository.Validate(); err != nil {
					cfg.Logger.Warn("dsops-data validation failed: %v", err)
					cfg.Logger.Debug("Falling back to hardcoded service registry")
					serviceRegistry = services.NewRegistry()
				} else {
					cfg.Logger.Debug("Loaded dsops-data: %d service types, %d instances, %d policies, %d principals", 
						len(repository.ServiceTypes), len(repository.ServiceInstances), 
						len(repository.RotationPolicies), len(repository.Principals))
					serviceRegistry = services.NewRegistryWithDataDriven(repository)
					supportedTypes := serviceRegistry.GetSupportedTypes()
					cfg.Logger.Debug("Service registry has %d supported types: %v", len(supportedTypes), supportedTypes)
				}
			}
		} else {
			cfg.Logger.Debug("dsops-data directory not found at %s, using hardcoded service registry", dataDir)
			serviceRegistry = services.NewRegistry()
		}
	} else {
		serviceRegistry = services.NewRegistry()
	}
	
	legacyRegistry := providers.NewRegistry()

	// Register secret stores from new format
	for name, storeConfig := range cfg.Definition.SecretStores {
		if !secretStoreRegistry.IsSupported(storeConfig.Type) {
			cfg.Logger.Warn("Secret store type '%s' not yet implemented for store '%s'", storeConfig.Type, name)
			continue
		}

		secretStore, err := secretStoreRegistry.CreateSecretStore(name, storeConfig)
		if err != nil {
			return fmt.Errorf("failed to create secret store '%s': %w", name, err)
		}

		// Wrap secret store with adapter to provide Provider interface
		provider := adapter.NewSecretStoreToProviderAdapter(secretStore)
		resolver.RegisterProvider(name, provider)
		cfg.Logger.Debug("Registered secret store '%s' with type '%s'", name, storeConfig.Type)
	}

	// Register services from new format
	for name, serviceConfig := range cfg.Definition.Services {
		cfg.Logger.Debug("Checking service '%s' of type '%s'", name, serviceConfig.Type)
		if !serviceRegistry.IsSupported(serviceConfig.Type) {
			cfg.Logger.Warn("Service type '%s' is not recognized in dsops-data for service '%s'", serviceConfig.Type, name)
			continue
		}

		if !serviceRegistry.HasImplementation(serviceConfig.Type) {
			cfg.Logger.Warn("Service type '%s' is recognized but has no rotation engine for service '%s'", serviceConfig.Type, name)
			continue
		}

		_, err := serviceRegistry.CreateService(name, serviceConfig)
		if err != nil {
			return fmt.Errorf("failed to create service '%s': %w", name, err)
		}

		// Services don't directly provide Provider interface, but we could create an adapter
		// For now, log that services are registered but not yet usable in resolution
		cfg.Logger.Debug("Registered service '%s' with type '%s' (not yet usable for secret resolution)", name, serviceConfig.Type)
	}

	// Register legacy providers (for backward compatibility)
	for name, providerConfig := range cfg.Definition.Providers {
		if !legacyRegistry.IsSupported(providerConfig.Type) {
			cfg.Logger.Warn("Provider type '%s' not yet implemented for provider '%s'", providerConfig.Type, name)
			continue
		}

		provider, err := legacyRegistry.CreateProvider(name, providerConfig)
		if err != nil {
			return fmt.Errorf("failed to create provider '%s': %w", name, err)
		}

		resolver.RegisterProvider(name, provider)
		cfg.Logger.Debug("Registered legacy provider '%s' with type '%s'", name, providerConfig.Type)
	}

	return nil
}

// outputPlanJSON outputs the plan result as JSON
func outputPlanJSON(result *resolve.PlanResult) error {
	output := map[string]interface{}{
		"variables": result.Variables,
		"errors":    make([]string, len(result.Errors)),
		"summary": map[string]interface{}{
			"total_variables": len(result.Variables),
			"error_count":     len(result.Errors),
		},
	}

	// Convert errors to strings for JSON output
	for i, err := range result.Errors {
		output["errors"].([]string)[i] = err.Error()
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// outputPlanTable outputs the plan result as a formatted table
func outputPlanTable(result *resolve.PlanResult, cfg *config.Config, envName string) error {
	// Sort variables by name for consistent output
	sort.Slice(result.Variables, func(i, j int) bool {
		return result.Variables[i].Name < result.Variables[j].Name
	})

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	_, _ = fmt.Fprintf(w, "VARIABLE\tSOURCE\tTRANSFORM\tOPTIONAL\tSTATUS\n")
	_, _ = fmt.Fprintf(w, "--------\t------\t---------\t--------\t------\n")

	errorCount := 0
	for _, variable := range result.Variables {
		status := "✓ OK"
		if variable.Error != nil {
			status = "✗ ERROR"
			errorCount++
		}

		optional := ""
		if variable.Optional {
			optional = "yes"
		}

		transform := variable.Transform
		if transform == "" {
			transform = "-"
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			variable.Name,
			variable.Source,
			transform,
			optional,
			status,
		)
	}

	_ = w.Flush()

	// Print summary
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Total variables: %d\n", len(result.Variables))
	fmt.Printf("  Ready to resolve: %d\n", len(result.Variables)-errorCount)
	
	if errorCount > 0 {
		fmt.Printf("  Errors: %d\n", errorCount)
		fmt.Printf("\nErrors:\n")
		for i, err := range result.Errors {
			fmt.Printf("  %d. %s\n", i+1, err.Error())
		}
		
		// Suggest next steps
		fmt.Printf("\nNext steps:\n")
		if strings.Contains(strings.Join(errorStrings(result.Errors), " "), "not registered") {
			fmt.Printf("  • Configure missing providers in dsops.yaml\n")
			fmt.Printf("  • Run 'dsops doctor' to check provider connectivity\n")
		}
		fmt.Printf("  • Fix configuration errors and try again\n")
		
		return fmt.Errorf("plan completed with %d errors", errorCount)
	}

	fmt.Printf("\n✓ All variables ready to resolve!\n")
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  • Run 'dsops exec --env %s -- <command>' to run with secrets\n", envName)
	fmt.Printf("  • Run 'dsops render --env %s --out .env' to create env file\n", envName)

	return nil
}

// Helper functions
func errorStrings(errors []error) []string {
	result := make([]string, len(errors))
	for i, err := range errors {
		result[i] = err.Error()
	}
	return result
}