package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/resolve"
	"github.com/systmms/dsops/internal/template"
)

func NewRenderCommand(cfg *config.Config) *cobra.Command {
	var (
		envName      string
		outputPath   string
		format       string
		templatePath string
		ttl          string
		permissions  string
	)

	cmd := &cobra.Command{
		Use:   "render --env <name> --out <file>",
		Short: "Render environment file from secrets",
		Long: `Generate .env files, JSON, YAML, or custom templates from resolved secrets.

The output format is auto-detected from the file extension, or can be specified 
explicitly with --format.

Supported formats:
  dotenv   - .env file format (default)
  json     - JSON object with variables
  yaml     - YAML object with variables  
  template - Custom Go template

Examples:
  dsops render --env development --out .env.development
  dsops render --env production --out config.json --format json
  dsops render --env staging --out app.yaml --format yaml
  dsops render --env prod --out k8s-secret.yaml --template secret.tmpl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate required flags
			if outputPath == "" {
				return fmt.Errorf("--out flag is required for security (explicit opt-in to write files)")
			}

			// Parse TTL if provided
			var ttlDuration time.Duration
			if ttl != "" {
				var err error
				ttlDuration, err = time.ParseDuration(ttl)
				if err != nil {
					return fmt.Errorf("invalid TTL duration: %w", err)
				}
			}

			// Parse permissions
			var perms os.FileMode = 0600 // Default: owner read/write only
			if permissions != "" {
				var err error
				perm64, err := fmt.Sscanf(permissions, "%o", &perms)
				if err != nil || perm64 != 1 {
					return fmt.Errorf("invalid permissions format, use octal like '0644'")
				}
			}

			// Load configuration
			if err := cfg.Load(); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Check output path policy
			if cfg.HasPolicies() {
				enforcer := cfg.GetPolicyEnforcer()
				if err := enforcer.ValidateOutputPath(outputPath); err != nil {
					return fmt.Errorf("output path policy violation: %w", err)
				}
			}

			// Create resolver
			resolver := resolve.New(cfg)

			// Register providers
			if err := registerProviders(resolver, cfg, ""); err != nil {
				return fmt.Errorf("failed to register providers: %w", err)
			}

			// Resolve secrets
			ctx := context.Background()
			resolved, err := resolver.Resolve(ctx, envName)
			if err != nil {
				return fmt.Errorf("failed to resolve secrets: %w", err)
			}

			// Convert to simple string map and check for errors
			variables := make(map[string]string)
			var resolveErrors []string

			for name, variable := range resolved {
				if variable.Error != nil {
					resolveErrors = append(resolveErrors, fmt.Sprintf("%s: %s", name, variable.Error))
					continue
				}
				variables[name] = variable.Value
			}

			if len(resolveErrors) > 0 {
				cfg.Logger.Error("Failed to resolve %d variables:", len(resolveErrors))
				for _, err := range resolveErrors {
					cfg.Logger.Error("  %s", err)
				}
				return fmt.Errorf("secret resolution failed")
			}

			// Load template content if using template format
			var templateContent string
			if format == "template" || templatePath != "" {
				if templatePath == "" {
					return fmt.Errorf("--template flag is required when using template format")
				}
				
				content, err := os.ReadFile(templatePath)
				if err != nil {
					return fmt.Errorf("failed to read template file: %w", err)
				}
				templateContent = string(content)
				format = "template" // Force template format
			}

			// Create renderer
			renderer := template.New(cfg.Logger)

			// Render output
			renderOptions := template.RenderOptions{
				Format:      format,
				Variables:   variables,
				OutputPath:  outputPath,
				Template:    templateContent,
				TTL:         ttlDuration,
				Permissions: perms,
			}

			if err := renderer.Render(renderOptions); err != nil {
				return fmt.Errorf("failed to render: %w", err)
			}

			// Security reminder
			cfg.Logger.Warn("File contains secrets - ensure it's added to .gitignore")
			if ttlDuration == 0 {
				cfg.Logger.Info("Consider using --ttl flag for auto-deletion of temporary files")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment name to render (required)")
	cmd.Flags().StringVar(&outputPath, "out", "", "Output file path (required for security)")
	cmd.Flags().StringVar(&format, "format", "", "Output format (dotenv|json|yaml|template, auto-detected from extension)")
	cmd.Flags().StringVar(&templatePath, "template", "", "Template file path (required for template format)")
	cmd.Flags().StringVar(&ttl, "ttl", "", "Auto-delete file after duration (e.g., '10m', '1h')")
	cmd.Flags().StringVar(&permissions, "permissions", "0600", "File permissions in octal (default: 0600)")

	_ = cmd.MarkFlagRequired("env")
	_ = cmd.MarkFlagRequired("out")

	return cmd
}