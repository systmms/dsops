# Data Model: Release & Distribution Infrastructure

**Date**: 2025-12-24
**Feature**: SPEC-020 Release & Distribution

## Entities

### Release

A versioned distribution of dsops artifacts.

| Field | Type | Description |
|-------|------|-------------|
| version | string | Semantic version (e.g., `1.0.0`, `1.2.3-beta.1`) |
| tag | string | Git tag (e.g., `v1.0.0`) |
| commit | string | Git commit SHA (short form) |
| date | datetime | Release build timestamp |
| changelog | string | Auto-generated from commit history |
| archives | []Archive | Platform-specific binary packages |
| checksums | Checksums | SHA256 hashes for all archives |
| docker_tags | []string | Docker image tags (e.g., `v1.0.0`, `latest`) |

**States**:
- `building` → Release workflow in progress
- `published` → All artifacts available on GitHub Releases
- `failed` → Build failed, no artifacts published

### Archive

A platform-specific binary package.

| Field | Type | Description |
|-------|------|-------------|
| name | string | Archive filename (e.g., `dsops_1.0.0_darwin_arm64.tar.gz`) |
| os | enum | Target OS: `darwin`, `linux`, `windows` |
| arch | enum | Target architecture: `amd64`, `arm64` |
| format | enum | Archive format: `tar.gz` (Unix), `zip` (Windows) |
| size | int | Archive size in bytes |
| sha256 | string | SHA256 hash of archive |

**Contents**:
```
dsops_1.0.0_darwin_arm64/
├── dsops                    # Binary executable
├── LICENSE                  # License file
├── README.md                # Quick start guide
└── completions/
    ├── dsops.bash           # Bash completion
    ├── dsops.zsh            # Zsh completion
    └── dsops.fish           # Fish completion
```

### Checksums

SHA256 hashes for all release artifacts.

| Field | Type | Description |
|-------|------|-------------|
| file | string | Checksums filename (e.g., `dsops_1.0.0_checksums.txt`) |
| hashes | map[string]string | Filename → SHA256 hash |

**Format** (standard sha256sum output):
```
a1b2c3d4...  dsops_1.0.0_darwin_arm64.tar.gz
e5f6g7h8...  dsops_1.0.0_darwin_amd64.tar.gz
i9j0k1l2...  dsops_1.0.0_linux_amd64.tar.gz
m3n4o5p6...  dsops_1.0.0_linux_arm64.tar.gz
q7r8s9t0...  dsops_1.0.0_windows_amd64.zip
```

### DockerImage

OCI container image with dsops pre-installed.

| Field | Type | Description |
|-------|------|-------------|
| registry | string | Container registry (e.g., `ghcr.io`) |
| repository | string | Image repository (e.g., `systmms/dsops`) |
| tags | []string | Image tags (e.g., `v1.0.0`, `latest`) |
| digest | string | Image digest (sha256) |
| size | int | Compressed image size in bytes |
| base | string | Base image (e.g., `gcr.io/distroless/static-debian12:nonroot`) |

**Labels** (OCI annotations):
```
org.opencontainers.image.version=1.0.0
org.opencontainers.image.source=https://github.com/systmms/dsops
org.opencontainers.image.revision=abc1234
org.opencontainers.image.created=2025-12-24T12:00:00Z
```

### HomebrewFormula

Ruby file defining Homebrew installation.

| Field | Type | Description |
|-------|------|-------------|
| name | string | Formula name: `dsops` |
| version | string | Package version |
| homepage | string | Project homepage URL |
| sha256 | map[string]string | Platform → archive SHA256 |
| binary | string | Binary name to install |
| completions | bool | Whether to install shell completions |

**Generated Formula Structure**:
```ruby
class Dsops < Formula
  desc "Secret management for development and production environments"
  homepage "https://github.com/systmms/dsops"
  version "1.0.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/systmms/dsops/releases/download/v1.0.0/dsops_1.0.0_darwin_arm64.tar.gz"
      sha256 "a1b2c3..."
    end
    on_intel do
      url "https://github.com/systmms/dsops/releases/download/v1.0.0/dsops_1.0.0_darwin_amd64.tar.gz"
      sha256 "e5f6g7..."
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/systmms/dsops/releases/download/v1.0.0/dsops_1.0.0_linux_arm64.tar.gz"
      sha256 "m3n4o5..."
    end
    on_intel do
      url "https://github.com/systmms/dsops/releases/download/v1.0.0/dsops_1.0.0_linux_amd64.tar.gz"
      sha256 "i9j0k1..."
    end
  end

  def install
    bin.install "dsops"
    bash_completion.install "completions/dsops.bash" => "dsops"
    zsh_completion.install "completions/dsops.zsh" => "_dsops"
    fish_completion.install "completions/dsops.fish"
  end

  test do
    system "#{bin}/dsops", "--version"
  end
end
```

## Relationships

```
Release 1 ─────── * Archive
    │
    └──────────── 1 Checksums
    │
    └──────────── 1 DockerImage
    │
    └──────────── 1 HomebrewFormula (external repo)
```

## Validation Rules

| Entity | Rule |
|--------|------|
| Release.version | Must be valid semver (MAJOR.MINOR.PATCH[-PRERELEASE]) |
| Release.tag | Must match pattern `v*` and correspond to version |
| Archive.sha256 | Must match computed hash of archive contents |
| DockerImage.size | Should be under 50MB (warning if exceeded) |
| HomebrewFormula | Must pass `brew audit --strict` |
