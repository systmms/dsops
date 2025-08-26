{
  description = "dsops - Developer Secret Operations";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config = {
            allowUnfree = true;
            allowBroken = true;
          };
        };
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go toolchain
            go_1_23
            gopls
            gotools
            go-tools
            delve

            # Linting and formatting
            golangci-lint
            gofumpt

            # Build tools
            gnumake
            git

            # Provider CLI tools for integration (install separately if needed)
            # _1password-cli
            # bitwarden-cli
            # awscli2
            # google-cloud-sdk
            # azure-cli
            # vault

            # Development utilities
            entr        # for file watching
            jq          # JSON processing
            yq          # YAML processing
            curl        # HTTP requests
            tree        # directory structure
            
            # Testing and debugging
            gotestsum   # enhanced go test output
            
            # Documentation
            mdbook      # if we want to build docs later
            hugo        # Static site generator for documentation
          ];

          shellHook = ''
            echo "🔐 dsops development environment activated"
            echo ""
            echo "Available commands:"
            echo "  make setup     - Install development tools"
            echo "  make build     - Build the binary"
            echo "  make test      - Run tests"
            echo "  make lint      - Run linter"
            echo "  make dev       - Build and run in development mode"
            echo ""
            echo "Go version: $(go version)"
            echo "golangci-lint version: $(golangci-lint --version)"
            echo ""

            # Set up Go environment
            export GOPATH=$(pwd)/.go
            export GOCACHE=$(pwd)/.cache/go-build
            export GOMODCACHE=$(pwd)/.cache/go-mod
            
            # Create directories if they don't exist
            mkdir -p .go .cache/go-build .cache/go-mod

            # Add local bin to PATH for installed tools
            export PATH="$(pwd)/bin:$PATH"
            
            # Provider CLI configuration hints
            echo "Provider CLI tools available:"
            echo "  op (1Password)     - Run 'op signin' to authenticate"
            echo "  bw (Bitwarden)     - Run 'bw login' to authenticate"
            echo "  aws                - Configure with 'aws configure' or env vars"
            echo "  gcloud             - Run 'gcloud auth login' to authenticate"
            echo "  az (Azure)         - Run 'az login' to authenticate"
            echo "  vault              - Set VAULT_ADDR and authenticate"
            echo ""
          '';

          # Environment variables
          CGO_ENABLED = "0";  # Static builds by default
          GOFLAGS = "-buildvcs=false";  # Disable VCS stamping in Nix
        };

        # Package definition for building dsops
        packages.default = pkgs.buildGoModule rec {
          pname = "dsops";
          version = "dev";
          
          src = ./.;
          
          vendorHash = null;  # Will need to be updated after go mod tidy
          
          buildInputs = with pkgs; [ git ];
          
          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
            "-X main.commit=${self.rev or "dev"}"
            "-X main.date=1970-01-01T00:00:00Z"
          ];

          meta = with pkgs.lib; {
            description = "Developer Secret Operations - Manage secrets across providers";
            homepage = "https://github.com/systmms/dsops";
            license = licenses.asl20;
            maintainers = [ ];
            platforms = platforms.unix;
          };
        };

        # Apps for running dsops directly
        apps.default = flake-utils.lib.mkApp {
          drv = self.packages.${system}.default;
        };

        # Formatter for nix files
        formatter = pkgs.nixpkgs-fmt;
      });
}