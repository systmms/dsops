{
  description = "dsops - CLI tool for managing secrets across different providers";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        
        # Go version 1.23+
        go = pkgs.go_1_23;
        
        # Common CLI tools for secret provider integrations
        providerTools = with pkgs; [
          # 1Password CLI
          _1password
          
          # Bitwarden CLI
          bitwarden-cli
          
          # HashiCorp Vault CLI
          vault
          
          # AWS CLI (for AWS Secrets Manager)
          awscli2
          
          # Google Cloud SDK (for Google Secret Manager)
          google-cloud-sdk
          
          # Azure CLI (for Azure Key Vault)
          azure-cli
          
          # Additional useful secret management tools
          age               # Modern encryption tool
          sops              # Secrets OPerationS
          jq                # JSON processor (useful for API responses)
          yq-go             # YAML processor
        ];

        # Development tools
        devTools = with pkgs; [
          # Core Go development
          go
          gopls             # Go language server
          gotools           # Includes goimports, godoc, etc.
          golangci-lint     # Go linter
          delve             # Go debugger
          
          # Build and development utilities
          gnumake           # GNU Make
          git               # Version control
          entr              # File watcher for the watch target
          
          # Useful development tools
          direnv            # Environment management
          pre-commit        # Pre-commit hooks
          
          # Documentation and formatting
          mdformat          # Markdown formatter
        ];

        # All tools combined
        allTools = devTools ++ providerTools;

        # Go environment variables
        goEnv = {
          # Ensure Go modules are enabled
          GO111MODULE = "on";
          
          # Set GOPATH and related paths
          GOPATH = "$PWD/.go";
          GOCACHE = "$PWD/.go/cache";
          GOMODCACHE = "$PWD/.go/pkg/mod";
          
          # Add Go bin to PATH
          PATH = "$GOPATH/bin:$PATH";
          
          # Set CGO for better compatibility
          CGO_ENABLED = "1";
          
          # Project-specific variables
          DSOPS_DEV = "true";
          DSOPS_DEBUG = "true";
        };

      in
      {
        # Default development shell
        devShells.default = pkgs.mkShell {
          buildInputs = allTools;
          
          shellHook = ''
            echo "🔐 dsops development environment"
            echo "Go version: $(go version)"
            echo "Project: github.com/systmms/dsops"
            echo ""
            
            # Create Go directories if they don't exist
            mkdir -p .go/{bin,cache,pkg/mod}
            
            # Display available make targets
            echo "Available make targets:"
            make help
            echo ""
            echo "Provider CLI tools available:"
            echo "  • 1Password CLI: op"
            echo "  • Bitwarden CLI: bw" 
            echo "  • Vault CLI: vault"
            echo "  • AWS CLI: aws"
            echo "  • Google Cloud SDK: gcloud"
            echo "  • Azure CLI: az"
            echo "  • Additional tools: age, sops, jq, yq"
            echo ""
            echo "Run 'make setup' to initialize the Go project dependencies."
          '';
          
          # Set environment variables
          inherit (goEnv) 
            GO111MODULE 
            GOPATH 
            GOCACHE 
            GOMODCACHE 
            CGO_ENABLED
            DSOPS_DEV
            DSOPS_DEBUG;
            
          # Ensure PATH includes Go bin directory
          PATH = goEnv.PATH;
        };

        # Alternative minimal shell for CI/production builds
        devShells.minimal = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gnumake
            git
          ];
          
          inherit (goEnv) GO111MODULE CGO_ENABLED;
        };

        # Package definition for the dsops CLI
        packages.default = pkgs.buildGoModule {
          pname = "dsops";
          version = "dev";
          
          src = ./.;
          
          # This will need to be updated when you have actual dependencies
          # For now, using a placeholder hash
          vendorHash = null;
          
          # Build from the correct main package
          subPackages = [ "cmd/dsops" ];
          
          # Set build-time variables
          ldflags = [
            "-X main.version=dev"
            "-X main.commit=nix-build"
            "-X main.date=nix"
            "-w"
            "-s"
          ];
          
          meta = with pkgs.lib; {
            description = "CLI tool for managing secrets across different providers";
            homepage = "https://github.com/systmms/dsops";
            license = licenses.mit; # Adjust as needed
            maintainers = [ ];
            platforms = platforms.unix;
          };
        };

        # Formatter for nix files
        formatter = pkgs.nixpkgs-fmt;
      }
    );
}