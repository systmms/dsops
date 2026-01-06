package commands

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/providers"
)

func NewProvidersCommand(cfg *config.Config) *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "providers",
		Short: "List available providers",
		Long: `Display information about available secret providers.

Shows both built-in provider types and configured provider instances.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := providers.NewRegistry()

			fmt.Println("Built-in Provider Types:")
			fmt.Println("=======================")
			
			supportedTypes := registry.GetSupportedTypes()
			sort.Strings(supportedTypes)
			
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintf(w, "TYPE\tDESCRIPTION\n")
			_, _ = fmt.Fprintf(w, "----\t-----------\n")
			
			for _, providerType := range supportedTypes {
				description := getProviderDescription(providerType)
				_, _ = fmt.Fprintf(w, "%s\t%s\n", providerType, description)
			}
			_ = w.Flush()

			// Show configured providers if config is available
			if err := cfg.Load(); err == nil && cfg.Definition != nil {
				fmt.Println("\nConfigured Providers:")
				fmt.Println("====================")
				
				providers := cfg.ListAllProviders()
				if len(providers) == 0 {
					fmt.Println("No providers configured")
				} else {
					w2 := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					_, _ = fmt.Fprintf(w2, "NAME\tTYPE\tSTATUS\n")
					_, _ = fmt.Fprintf(w2, "----\t----\t------\n")
					
					for name, providerCfg := range providers {
						status := "configured"
						if !registry.IsSupported(providerCfg.Type) {
							status = "unsupported"
						}
						
						_, _ = fmt.Fprintf(w2, "%s\t%s\t%s\n", name, providerCfg.Type, status)
					}
					_ = w2.Flush()
				}
			}

			if verbose {
				fmt.Println("\nProvider Details:")
				fmt.Println("================")
				for _, providerType := range supportedTypes {
					fmt.Printf("\n%s:\n", providerType)
					details := getProviderDetails(providerType)
					for _, detail := range details {
						fmt.Printf("  • %s\n", detail)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed provider information")

	return cmd
}

// getProviderDescription returns a description for a provider type
func getProviderDescription(providerType string) string {
	descriptions := map[string]string{
		"literal":             "Static literal values for testing",
		"mock":                "Mock provider for testing and development",
		"json":                "JSON provider with test data for transforms",
		"bitwarden":           "Bitwarden password manager via CLI",
		"aws.secretsmanager":  "AWS Secrets Manager via SDK",
		"aws.ssm":             "AWS Systems Manager Parameter Store",
		"aws.sts":             "AWS STS for temporary credentials",
		"aws.sso":             "AWS IAM Identity Center (SSO)",
		"aws":                 "AWS unified provider (intelligent routing)",
		"gcp.secretmanager":   "Google Cloud Secret Manager",
		"gcp":                 "GCP unified provider (intelligent routing)",
		"azure.keyvault":      "Azure Key Vault",
		"azure.identity":      "Azure Managed Identity / Service Principal",
		"azure":               "Azure unified provider (intelligent routing)",
		"onepassword":         "1Password password manager via CLI",
		"vault":               "HashiCorp Vault",
		"doppler":             "Doppler centralized secrets management",
		"pass":                "pass (zx2c4) Unix password manager",
		"keychain":            "OS native keychain (macOS Keychain, Linux Secret Service)",
		"infisical":           "Infisical open-source secret management platform",
		"akeyless":            "Akeyless enterprise zero-knowledge secret management",
	}
	
	if desc, exists := descriptions[providerType]; exists {
		return desc
	}
	return "No description available"
}

// getProviderDetails returns detailed information for a provider type
func getProviderDetails(providerType string) []string {
	details := map[string][]string{
		"literal": {
			"Uses static values defined in configuration",
			"Useful for non-secret configuration values",
			"No external dependencies required",
		},
		"mock": {
			"Simulates external provider behavior",
			"Pre-populated with test values",
			"Supports simulated failures and delays",
		},
		"json": {
			"Contains JSON test data for testing transforms",
			"Useful for testing json_extract transforms",
			"Pre-populated with sample data structures",
		},
		"bitwarden": {
			"Requires Bitwarden CLI ('bw') in PATH",
			"Supports all item types and custom fields",
			"Requires authentication: bw login && bw unlock",
			"Key format: 'item-name.field' or 'item-id.field'",
		},
		"aws.secretsmanager": {
			"Uses AWS SDK v2 for direct API access",
			"Supports JSON secrets with field extraction",
			"Requires AWS credentials (CLI, env vars, IAM roles)",
			"Key format: 'secret-name' or 'secret-name#.json.path'",
			"Supports versioning (AWSCURRENT, AWSPENDING, version-id)",
		},
		"onepassword": {
			"Requires 1Password CLI ('op') in PATH",
			"Supports all item types and custom fields",
			"Requires authentication: op signin",
			"Key format: 'item-name.field' or 'op://vault/item/field'",
			"Supports vault-specific access",
		},
		"aws.ssm": {
			"AWS Systems Manager Parameter Store",
			"Supports standard and SecureString parameters",
			"Automatic KMS decryption for SecureString",
			"Key format: '/path/to/parameter'",
			"Supports parameter prefixing and hierarchies",
		},
		"aws.sts": {
			"AWS Security Token Service for temporary credentials",
			"Supports role assumption with MFA",
			"External ID support for cross-account access",
			"Session policies for fine-grained permissions",
			"Key format: 'access_key_id', 'secret_access_key', 'session_token'",
		},
		"aws.sso": {
			"AWS IAM Identity Center (formerly AWS SSO)",
			"Browser-based authentication flow",
			"Requires AWS CLI v2 with SSO support",
			"Caches credentials locally for reuse",
			"Key format: same as aws.sts",
		},
		"vault": {
			"HashiCorp Vault enterprise secret management",
			"Multiple auth methods: token, userpass, LDAP, AWS, k8s",
			"Supports KV v1 and v2 secret engines",
			"Dynamic secret generation",
			"Key format: 'secret/path:field' or 'secret/path'",
		},
		"aws": {
			"Unified AWS provider with intelligent routing",
			"Auto-detects service based on reference format",
			"Supports all AWS secret services in one provider",
			"Prefixes: sm:, ssm:, sts:, sso: or auto-detection",
			"Configurable default service (secretsmanager, ssm, etc.)",
		},
		"gcp.secretmanager": {
			"Google Cloud Secret Manager",
			"Supports versioned secrets and binary data",
			"JSON path extraction with # syntax",
			"Service account and ADC authentication",
			"Key format: 'secret-name:version' or 'secret-name#.json.path'",
		},
		"gcp": {
			"Unified GCP provider with intelligent routing",
			"Auto-detects service based on reference format",
			"Currently supports Secret Manager (extensible)",
			"Prefixes: sm:, secretmanager:, secrets: or auto-detection",
			"Configurable default service (secretmanager)",
		},
		"azure.keyvault": {
			"Azure Key Vault for secrets, keys, and certificates",
			"Supports versioned secrets and HSM-backed storage",
			"JSON path extraction with # syntax",
			"Managed Identity and Service Principal authentication",
			"Key format: 'secret-name/version' or 'secret-name#.json.path'",
		},
		"azure.identity": {
			"Azure Managed Identity and Service Principal tokens",
			"Provides access tokens for Azure services",
			"System-assigned and user-assigned managed identity",
			"Service principal with client secret or certificate",
			"Key format: 'scope:field' (e.g., 'https://management.azure.com/.default:access_token')",
		},
		"azure": {
			"Unified Azure provider with intelligent routing",
			"Auto-detects service based on reference format",
			"Supports Key Vault and Identity services",
			"Prefixes: kv:, keyvault:, identity:, token: or auto-detection",
			"Configurable default service (keyvault)",
		},
		"doppler": {
			"Doppler cloud-based secrets management",
			"Simple environment variable injection",
			"Project and config-based secret organization",
			"Service token authentication",
			"Key format: 'SECRET_NAME' (direct secret names)",
		},
		"pass": {
			"Unix password manager using GPG and Git",
			"Local password store in ~/.password-store",
			"Hierarchical organization with folders",
			"GPG key-based encryption",
			"Key format: 'path/to/secret' (filesystem-like paths)",
		},
		"keychain": {
			"OS native credential storage",
			"macOS: Keychain Services (hardware-backed on Apple Silicon)",
			"Linux: Secret Service D-Bus API (gnome-keyring, KWallet)",
			"Works offline with no external dependencies",
			"Key format: 'service-name/account-name'",
			"Supports Touch ID authentication on macOS",
		},
		"infisical": {
			"Open-source secret management platform",
			"Self-hosted or cloud-hosted (infisical.com)",
			"End-to-end encryption with zero-knowledge architecture",
			"Project and environment-based organization",
			"Auth methods: machine_identity, service_token, api_key",
			"Key format: 'SECRET_NAME' or 'folder/SECRET_NAME[@vN]'",
		},
		"akeyless": {
			"Enterprise zero-knowledge secret management",
			"FIPS 140-2 certified with Distributed Fragment Cryptography",
			"Multiple auth methods: api_key, aws_iam, azure_ad, gcp",
			"Supports dynamic secrets and certificate management",
			"Key format: '/path/to/secret[@vN]'",
			"Self-hosted or cloud-hosted gateway",
		},
	}
	
	if detail, exists := details[providerType]; exists {
		return detail
	}
	return []string{"No details available"}
}