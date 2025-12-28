---
title: "Installation"
description: "Install dsops on your system"
lead: "Multiple installation methods are available for dsops. Choose the one that works best for your environment."
date: 2024-08-26T12:00:00-07:00
lastmod: 2024-08-26T12:00:00-07:00
draft: false
weight: 10
---

## Recommended: Homebrew

For macOS and Linux users with [Homebrew](https://brew.sh) installed:

```bash
brew install systmms/tap/dsops
```

## Go Install

For Go developers with Go 1.25 or later:

```bash
go install github.com/systmms/dsops/cmd/dsops@latest
```

Or install a specific version:

```bash
go install github.com/systmms/dsops/cmd/dsops@v0.2.0
```

## Binary Download

Download pre-compiled binaries from [GitHub Releases](https://github.com/systmms/dsops/releases):

1. Download the appropriate binary for your platform
2. Extract the archive
3. Move the binary to a location in your PATH

### Linux
```bash
# Download a specific version (adjust version as needed)
VERSION="0.2.0"
curl -L "https://github.com/systmms/dsops/releases/download/v${VERSION}/dsops_${VERSION}_linux_amd64.tar.gz" -o dsops.tar.gz

# Extract
tar -xzf dsops.tar.gz

# Install
sudo mv dsops /usr/local/bin/
sudo chmod +x /usr/local/bin/dsops

# Verify
dsops --version
```

### macOS
```bash
# Download a specific version (adjust version and architecture as needed)
VERSION="0.2.0"
# Use darwin_arm64 for Apple Silicon, darwin_amd64 for Intel Macs
ARCH="darwin_arm64"
curl -L "https://github.com/systmms/dsops/releases/download/v${VERSION}/dsops_${VERSION}_${ARCH}.tar.gz" -o dsops.tar.gz

# Extract
tar -xzf dsops.tar.gz

# Install
sudo mv dsops /usr/local/bin/
sudo chmod +x /usr/local/bin/dsops

# Verify
dsops --version
```

### Verify Checksums

For security-conscious users, verify the downloaded archive:

```bash
# Download checksums file
curl -LO "https://github.com/systmms/dsops/releases/download/v${VERSION}/dsops_${VERSION}_checksums.txt"

# Verify (uses sha256sum on Linux, shasum on macOS)
sha256sum -c dsops_${VERSION}_checksums.txt --ignore-missing
# or on macOS:
shasum -a 256 -c dsops_${VERSION}_checksums.txt --ignore-missing
```

## From Source

Build from source using Go 1.25 or later:

```bash
# Clone repository
git clone https://github.com/systmms/dsops.git
cd dsops

# Build
make build

# Install
sudo make install

# Or install to custom location
make install PREFIX=$HOME/.local
```

## Docker

Run dsops in a container:

```bash
docker run --rm -v $(pwd):/work ghcr.io/systmms/dsops:latest plan --env production
```

Or add as an alias:

```bash
alias dsops='docker run --rm -v $(pwd):/work -v $HOME/.config:/root/.config ghcr.io/systmms/dsops:latest'
```

## Nix

For Nix users:

```bash
# Run directly
nix run github:systmms/dsops -- --version

# Install into profile
nix profile install github:systmms/dsops
```

## Verify Installation

After installation, verify dsops is working:

```bash
dsops --version
dsops --help
```

## Shell Completion

Enable shell completion for better CLI experience:

{{< tabs >}}
{{< tab "bash" >}}
```bash
# Add to ~/.bashrc
eval "$(dsops completion bash)"
```
{{< /tab >}}
{{< tab "zsh" >}}
```bash
# Add to ~/.zshrc
eval "$(dsops completion zsh)"
```
{{< /tab >}}
{{< tab "fish" >}}
```bash
# Add to ~/.config/fish/config.fish
dsops completion fish | source
```
{{< /tab >}}
{{< /tabs >}}

## Next Steps

- [Configure your first provider](/getting-started/quick-start/)
- [Learn about dsops configuration](/getting-started/configuration/)
- [Explore available providers](/providers/)