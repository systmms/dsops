package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	dserrors "github.com/systmms/dsops/internal/errors"
)

func NewLoginCommand(cfg *config.Config) *cobra.Command {
	var (
		listProviders bool
		interactive   bool
	)

	cmd := &cobra.Command{
		Use:   "login [provider]",
		Short: "Assist with provider authentication",
		Long: `Help authenticate with secret providers by providing step-by-step guidance.

This command provides authentication instructions and can optionally run 
authentication commands for supported providers.

Examples:
  dsops login                  # Show all available providers
  dsops login bitwarden       # Show Bitwarden authentication steps
  dsops login 1password       # Show 1Password authentication steps  
  dsops login aws              # Show AWS authentication steps
  dsops login --interactive    # Interactive provider selection`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listProviders || len(args) == 0 {
				return showAvailableProviders()
			}

			providerType := strings.ToLower(args[0])
			return authenticateProvider(providerType, interactive)
		},
	}

	cmd.Flags().BoolVarP(&listProviders, "list", "l", false, "List all available providers")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Run authentication commands interactively")

	return cmd
}

func showAvailableProviders() error {
	fmt.Println("🔐 Available Secret Providers:")
	fmt.Println()
	
	providers := []struct {
		name        string
		description string
		status      string
	}{
		{"bitwarden", "Bitwarden password manager", "✅ Supported"},
		{"1password", "1Password password manager", "✅ Supported"},
		{"aws", "AWS Secrets Manager", "✅ Supported"},
		{"aws.ssm", "AWS SSM Parameter Store", "✅ Supported"},
		{"aws.sts", "AWS STS (temporary credentials)", "✅ Supported"},
		{"aws.sso", "AWS IAM Identity Center (SSO)", "✅ Supported"},
		{"gcp", "Google Cloud Secret Manager", "✅ Supported"},
		{"gcp.secretmanager", "Google Cloud Secret Manager", "✅ Supported"},
		{"azure", "Azure Key Vault", "✅ Supported"},
		{"azure.keyvault", "Azure Key Vault", "✅ Supported"},
		{"azure.identity", "Azure Managed Identity", "✅ Supported"},
		{"vault", "HashiCorp Vault", "✅ Supported"},
	}

	for _, p := range providers {
		fmt.Printf("  %-12s %s %s\n", p.name, p.status, p.description)
	}
	
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  dsops login <provider>           # Show authentication steps")
	fmt.Println("  dsops login <provider> --interactive  # Run authentication commands")
	
	return nil
}

func authenticateProvider(providerType string, interactive bool) error {
	switch providerType {
	case "bitwarden", "bw":
		return authenticateBitwarden(interactive)
	case "1password", "onepassword", "op":
		return authenticateOnePassword(interactive)
	case "aws", "aws-secretsmanager":
		return authenticateAWS(interactive)
	case "aws.ssm":
		return authenticateAWSSSM(interactive)
	case "aws.sts":
		return authenticateAWSSTS(interactive)
	case "aws.sso":
		return authenticateAWSSSO(interactive)
	case "gcp", "google", "google-cloud", "gcp.secretmanager":
		return authenticateGCP(interactive)
	case "azure", "az", "azure.keyvault", "azure.identity":
		return authenticateAzure(interactive)
	case "vault", "hashicorp-vault":
		return authenticateVault(interactive)
	default:
		return dserrors.UserError{
			Message:    fmt.Sprintf("Unknown provider: %s", providerType),
			Suggestion: "Run 'dsops login --list' to see available providers",
		}
	}
}

func authenticateBitwarden(interactive bool) error {
	fmt.Println("🔐 Bitwarden Authentication")
	fmt.Println()
	
	// Check if CLI is installed
	if _, err := exec.LookPath("bw"); err != nil {
		fmt.Println("❌ Bitwarden CLI not found")
		fmt.Println()
		fmt.Println("Install instructions:")
		fmt.Println("  npm install -g @bitwarden/cli")
		fmt.Println("  # or")
		fmt.Println("  brew install bitwarden-cli")
		fmt.Println()
		fmt.Println("More info: https://bitwarden.com/help/cli/")
		return nil
	}

	fmt.Println("✅ Bitwarden CLI found")
	fmt.Println()
	fmt.Println("Authentication steps:")
	fmt.Println("  1. bw login                    # Login to your account")
	fmt.Println("  2. bw unlock                   # Unlock your vault")
	fmt.Println("  3. export BW_SESSION=\"...\"     # Export session token")
	fmt.Println()

	if interactive {
		fmt.Println("🔄 Running interactive authentication...")
		return runCommand("bw", "login")
	}

	fmt.Println("Next: Run 'dsops doctor' to verify authentication")
	return nil
}

func authenticateOnePassword(interactive bool) error {
	fmt.Println("🔐 1Password Authentication")
	fmt.Println()
	
	// Check if CLI is installed
	if _, err := exec.LookPath("op"); err != nil {
		fmt.Println("❌ 1Password CLI not found")
		fmt.Println()
		fmt.Println("Install instructions:")
		fmt.Println("  Visit: https://developer.1password.com/docs/cli/get-started/")
		fmt.Println()
		return nil
	}

	fmt.Println("✅ 1Password CLI found")
	fmt.Println()
	fmt.Println("Authentication steps:")
	fmt.Println("  1. op signin                   # Sign in to your account")
	fmt.Println("  2. eval $(op signin)           # Load session variables")
	fmt.Println()

	if interactive {
		fmt.Println("🔄 Running interactive authentication...")
		return runCommand("op", "signin")
	}

	fmt.Println("Next: Run 'dsops doctor' to verify authentication")
	return nil
}

func authenticateAWS(interactive bool) error {
	fmt.Println("🔐 AWS Authentication")
	fmt.Println()

	// Check if CLI is installed
	if _, err := exec.LookPath("aws"); err != nil {
		fmt.Println("❌ AWS CLI not found")
		fmt.Println()
		fmt.Println("Install instructions:")
		fmt.Println("  pip install awscli")
		fmt.Println("  # or")
		fmt.Println("  brew install awscli")
		fmt.Println()
		fmt.Println("More info: https://aws.amazon.com/cli/")
		return nil
	}

	fmt.Println("✅ AWS CLI found")
	fmt.Println()
	fmt.Println("Authentication options:")
	fmt.Println("  1. aws configure               # Interactive credential setup")
	fmt.Println("  2. export AWS_PROFILE=myprofile")
	fmt.Println("  3. Set environment variables:")
	fmt.Println("     export AWS_ACCESS_KEY_ID=...")
	fmt.Println("     export AWS_SECRET_ACCESS_KEY=...")
	fmt.Println("     export AWS_REGION=us-east-1")
	fmt.Println()

	if interactive {
		fmt.Println("🔄 Running interactive configuration...")
		return runCommand("aws", "configure")
	}

	fmt.Println("Next: Run 'dsops doctor' to verify authentication")
	return nil
}

func authenticateGCP(interactive bool) error {
	fmt.Println("🔐 Google Cloud Platform Authentication")
	fmt.Println()
	
	// Check if gcloud CLI is installed
	if _, err := exec.LookPath("gcloud"); err != nil {
		fmt.Println("❌ Google Cloud CLI not found")
		fmt.Println()
		fmt.Println("Install instructions:")
		fmt.Println("  Visit: https://cloud.google.com/sdk/docs/install")
		fmt.Println("  # or")
		fmt.Println("  brew install google-cloud-sdk")
		fmt.Println()
		return nil
	}

	fmt.Println("✅ Google Cloud CLI found")
	fmt.Println()
	fmt.Println("Authentication options:")
	fmt.Println()
	fmt.Println("1. Application Default Credentials (recommended):")
	fmt.Println("   gcloud auth application-default login")
	fmt.Println()
	fmt.Println("2. Service Account Key:")
	fmt.Println("   export GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json")
	fmt.Println()
	fmt.Println("3. Compute Engine / Cloud Shell:")
	fmt.Println("   # Automatic - uses instance metadata")
	fmt.Println()
	fmt.Println("4. Workload Identity (GKE):")
	fmt.Println("   # Automatic - uses workload identity")
	fmt.Println()
	fmt.Println("Required IAM permissions:")
	fmt.Println("  • secretmanager.secrets.get")
	fmt.Println("  • secretmanager.versions.access")
	fmt.Println("  • secretmanager.secrets.list (for validation)")
	fmt.Println()

	if interactive {
		fmt.Println("🔄 Running Application Default Credentials setup...")
		return runCommand("gcloud", "auth", "application-default", "login")
	}

	fmt.Println("Configuration example:")
	fmt.Println("  providers:")
	fmt.Println("    gcp:")
	fmt.Println("      type: gcp")
	fmt.Println("      project_id: my-project-id")
	fmt.Println("      # Optional: service_account_key_path: /path/to/key.json")
	fmt.Println("      # Optional: impersonate_service_account: sa@project.iam.gserviceaccount.com")
	fmt.Println()
	fmt.Println("Next: Run 'dsops doctor' to verify authentication")
	return nil
}

func authenticateAzure(interactive bool) error {
	fmt.Println("🔐 Microsoft Azure Authentication")
	fmt.Println()
	
	// Check if Azure CLI is installed
	if _, err := exec.LookPath("az"); err != nil {
		fmt.Println("❌ Azure CLI not found")
		fmt.Println()
		fmt.Println("Install instructions:")
		fmt.Println("  Visit: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli")
		fmt.Println("  # or")
		fmt.Println("  brew install azure-cli")
		fmt.Println()
		return nil
	}

	fmt.Println("✅ Azure CLI found")
	fmt.Println()
	fmt.Println("Authentication options:")
	fmt.Println()
	fmt.Println("1. Managed Identity (recommended for Azure VMs/Containers):")
	fmt.Println("   # Automatic - no configuration needed")
	fmt.Println("   # System-assigned or user-assigned managed identity")
	fmt.Println()
	fmt.Println("2. Azure CLI (for development):")
	fmt.Println("   az login")
	fmt.Println()
	fmt.Println("3. Service Principal with Client Secret:")
	fmt.Println("   export AZURE_TENANT_ID=your-tenant-id")
	fmt.Println("   export AZURE_CLIENT_ID=your-client-id")
	fmt.Println("   export AZURE_CLIENT_SECRET=your-client-secret")
	fmt.Println()
	fmt.Println("4. Service Principal with Certificate:")
	fmt.Println("   # Certificate authentication (advanced)")
	fmt.Println()
	fmt.Println("Required permissions for Key Vault:")
	fmt.Println("  • Key Vault Secrets User (or custom policy)")
	fmt.Println("  • GET and LIST permissions on secrets")
	fmt.Println()
	fmt.Println("Required permissions for Managed Identity:")
	fmt.Println("  • Appropriate role assignments for target resources")
	fmt.Println()

	if interactive {
		fmt.Println("🔄 Running Azure CLI login...")
		return runCommand("az", "login")
	}

	fmt.Println("Configuration examples:")
	fmt.Println()
	fmt.Println("Key Vault with Managed Identity:")
	fmt.Println("  providers:")
	fmt.Println("    azure-kv:")
	fmt.Println("      type: azure.keyvault")
	fmt.Println("      vault_url: https://my-vault.vault.azure.net/")
	fmt.Println("      use_managed_identity: true")
	fmt.Println()
	fmt.Println("Service Principal:")
	fmt.Println("  providers:")
	fmt.Println("    azure-sp:")
	fmt.Println("      type: azure")
	fmt.Println("      tenant_id: your-tenant-id")
	fmt.Println("      client_id: your-client-id")
	fmt.Println("      client_secret: your-client-secret")
	fmt.Println("      keyvault:")
	fmt.Println("        vault_url: https://my-vault.vault.azure.net/")
	fmt.Println()
	fmt.Println("Next: Run 'dsops doctor' to verify authentication")
	return nil
}

func authenticateVault(interactive bool) error {
	fmt.Println("🔐 HashiCorp Vault Authentication")
	fmt.Println()
	
	// Check if Vault CLI is installed
	if _, err := exec.LookPath("vault"); err != nil {
		fmt.Println("❌ Vault CLI not found")
		fmt.Println()
		fmt.Println("Install instructions:")
		fmt.Println("  brew install vault")
		fmt.Println("  # or")
		fmt.Println("  wget https://releases.hashicorp.com/vault/.../vault_linux_amd64.zip")
		fmt.Println()
		fmt.Println("More info: https://developer.hashicorp.com/vault/docs/install")
		return nil
	}

	fmt.Println("✅ Vault CLI found")
	fmt.Println()
	fmt.Println("Authentication methods:")
	fmt.Println("  1. Token (simple):")
	fmt.Println("     export VAULT_ADDR=https://vault.example.com:8200")
	fmt.Println("     export VAULT_TOKEN=hvs.your-token-here")
	fmt.Println()
	fmt.Println("  2. Username/Password:")
	fmt.Println("     vault auth -method=userpass username=myuser")
	fmt.Println("     export VAULT_TOKEN=$(vault print token)")
	fmt.Println()
	fmt.Println("  3. LDAP:")
	fmt.Println("     vault auth -method=ldap username=myuser")
	fmt.Println()
	fmt.Println("  4. AWS IAM:")
	fmt.Println("     vault auth -method=aws role=my-role")
	fmt.Println()
	fmt.Println("  5. Kubernetes:")
	fmt.Println("     vault auth -method=kubernetes role=my-role")
	fmt.Println()

	if interactive {
		fmt.Println("🔄 Running interactive authentication...")
		fmt.Println("Choose your auth method and run the appropriate vault auth command")
		return nil
	}

	fmt.Println("Configuration example:")
	fmt.Println("  providers:")
	fmt.Println("    vault:")
	fmt.Println("      type: vault")
	fmt.Println("      address: https://vault.example.com:8200")
	fmt.Println("      auth_method: token  # or userpass, ldap, aws, k8s")
	fmt.Println()
	fmt.Println("Next: Run 'dsops doctor' to verify authentication")
	return nil
}

func runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	
	if err := cmd.Run(); err != nil {
		return dserrors.CommandError{
			Command:    fmt.Sprintf("%s %s", command, strings.Join(args, " ")),
			Message:    err.Error(),
			Suggestion: "Check the command output above for details",
		}
	}
	
	return nil
}

func authenticateAWSSSM(interactive bool) error {
	fmt.Println("🔐 AWS SSM Parameter Store Authentication")
	fmt.Println()
	fmt.Println("AWS SSM uses the same authentication as AWS Secrets Manager.")
	fmt.Println()
	return authenticateAWS(interactive)
}

func authenticateAWSSTS(interactive bool) error {
	fmt.Println("🔐 AWS STS Authentication")
	fmt.Println()
	fmt.Println("AWS STS is used to assume roles and get temporary credentials.")
	fmt.Println()
	fmt.Println("Configuration example:")
	fmt.Println("  providers:")
	fmt.Println("    temp-creds:")
	fmt.Println("      type: aws.sts")
	fmt.Println("      assume_role: arn:aws:iam::123456789012:role/MyRole")
	fmt.Println("      role_session_name: dsops-session  # optional")
	fmt.Println("      external_id: my-external-id       # optional")
	fmt.Println("      duration: 3600                    # optional (seconds)")
	fmt.Println()
	fmt.Println("With MFA:")
	fmt.Println("      mfa_serial_number: arn:aws:iam::123456789012:mfa/myuser")
	fmt.Println("      mfa_token_code: 123456            # from your MFA device")
	fmt.Println()
	fmt.Println("Next: Run 'dsops doctor' to verify authentication")
	return nil
}

func authenticateAWSSSO(interactive bool) error {
	fmt.Println("🔐 AWS IAM Identity Center (SSO) Authentication")
	fmt.Println()
	
	// Check if AWS CLI is installed
	if _, err := exec.LookPath("aws"); err != nil {
		fmt.Println("❌ AWS CLI not found")
		fmt.Println()
		fmt.Println("Install instructions:")
		fmt.Println("  pip install awscli")
		fmt.Println("  # or")
		fmt.Println("  brew install awscli")
		fmt.Println()
		fmt.Println("AWS CLI v2 is required for SSO support.")
		return nil
	}

	fmt.Println("✅ AWS CLI found")
	fmt.Println()
	fmt.Println("SSO Authentication steps:")
	fmt.Println("  1. Configure SSO profile:")
	fmt.Println("     aws configure sso")
	fmt.Println()
	fmt.Println("  2. Login to SSO:")
	fmt.Println("     aws sso login --profile my-sso-profile")
	fmt.Println()
	fmt.Println("Configuration example:")
	fmt.Println("  providers:")
	fmt.Println("    sso-creds:")
	fmt.Println("      type: aws.sso")
	fmt.Println("      start_url: https://my-sso-portal.awsapps.com/start")
	fmt.Println("      account_id: \"123456789012\"")
	fmt.Println("      role_name: AdministratorAccess")
	fmt.Println("      region: us-east-1  # optional")
	fmt.Println()

	if interactive {
		fmt.Println("🔄 Running SSO configuration...")
		return runCommand("aws", "configure", "sso")
	}

	fmt.Println("Next: Run 'dsops doctor' to verify authentication")
	return nil
}