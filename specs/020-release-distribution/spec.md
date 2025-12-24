# Feature Specification: Release & Distribution Infrastructure

**Feature Branch**: `020-release-distribution`
**Created**: 2025-12-24
**Status**: Draft
**Input**: User description: "Release & Distribution Infrastructure - Enable users to easily install dsops via multiple methods: Homebrew tap (brew install systmms/tap/dsops), GitHub Releases with pre-built binaries for macOS (arm64, amd64), Linux (amd64, arm64), and Windows (amd64), Docker images published to ghcr.io/systmms/dsops, and go install. Implementation should use GoReleaser for cross-platform builds and changelog generation, GitHub Actions workflows triggered on version tags, automated Homebrew formula updates, and shell completion scripts bundled with releases."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Install via Homebrew (Priority: P1)

A developer on macOS or Linux wants to quickly install dsops using their existing package manager. They expect a single command to install the tool and have it immediately available in their PATH.

**Why this priority**: Homebrew is the most common package manager for macOS developers, and the recommended installation method documented in the project. This provides the smoothest onboarding experience.

**Independent Test**: Can be fully tested by running `brew install systmms/tap/dsops` on a fresh macOS system and verifying `dsops --version` works.

**Acceptance Scenarios**:

1. **Given** a macOS user with Homebrew installed, **When** they run `brew install systmms/tap/dsops`, **Then** dsops is installed and `dsops --version` displays the current version
2. **Given** an existing dsops installation via Homebrew, **When** the user runs `brew upgrade dsops`, **Then** dsops is updated to the latest version
3. **Given** a Homebrew installation, **When** a new dsops version is released, **Then** the Homebrew formula is automatically updated within 1 hour

---

### User Story 2 - Download Pre-built Binary (Priority: P1)

A developer wants to download a pre-built binary for their platform without using a package manager. They need binaries for different operating systems and architectures.

**Why this priority**: Binary downloads are essential for CI/CD pipelines, Docker images, and users who don't use package managers. This is a fundamental distribution method.

**Independent Test**: Can be fully tested by downloading a binary from GitHub Releases, extracting it, and running `./dsops --version`.

**Acceptance Scenarios**:

1. **Given** any GitHub user, **When** they visit the Releases page, **Then** they see pre-built binaries for macOS (arm64, amd64), Linux (amd64, arm64), and Windows (amd64)
2. **Given** a downloaded binary archive, **When** the user extracts it, **Then** they find the dsops binary, shell completion scripts, and a LICENSE file
3. **Given** a downloaded macOS binary, **When** the user runs it, **Then** it executes (may require security bypass on first run if not notarized)
4. **Given** a version tag is pushed, **When** the release workflow completes, **Then** binaries and checksums are published to GitHub Releases within 15 minutes

---

### User Story 3 - Run via Docker (Priority: P2)

A DevOps engineer wants to use dsops in a containerized environment without installing it on their host system. They need a lightweight Docker image that can be used in CI/CD pipelines.

**Why this priority**: Docker provides isolation and reproducibility, essential for CI/CD and environments where installing tools isn't practical.

**Independent Test**: Can be fully tested by running `docker run --rm ghcr.io/systmms/dsops:latest --version`.

**Acceptance Scenarios**:

1. **Given** Docker is installed, **When** the user runs `docker pull ghcr.io/systmms/dsops:latest`, **Then** the image downloads successfully
2. **Given** a dsops configuration file, **When** the user mounts it and runs `docker run --rm -v $(pwd):/work ghcr.io/systmms/dsops plan`, **Then** dsops executes correctly
3. **Given** a new version tag, **When** the release workflow completes, **Then** images are published with both `:latest` and `:vX.Y.Z` tags
4. **Given** the Docker image, **When** inspected, **Then** it is based on a minimal base and is under 50MB

---

### User Story 4 - Install via go install (Priority: P2)

A Go developer wants to install dsops using the standard Go toolchain. They expect `go install` to work without cloning the repository.

**Why this priority**: Go developers expect this method to work. It requires minimal infrastructure (just a public repo) and is often preferred by contributors.

**Independent Test**: Can be fully tested by running `go install github.com/systmms/dsops/cmd/dsops@latest`.

**Acceptance Scenarios**:

1. **Given** Go 1.21+ is installed, **When** the user runs `go install github.com/systmms/dsops/cmd/dsops@latest`, **Then** dsops is installed to `$GOPATH/bin`
2. **Given** a specific version tag, **When** the user runs `go install github.com/systmms/dsops/cmd/dsops@v1.0.0`, **Then** that specific version is installed

---

### User Story 5 - Automated Release on Version Tag (Priority: P1)

A maintainer wants to release a new version by simply pushing a version tag. The entire release process should be automated with no manual steps.

**Why this priority**: Automation ensures consistent, reproducible releases and reduces human error. This is the foundation for all other distribution methods.

**Independent Test**: Can be fully tested by pushing a `v0.2.0` tag and verifying all artifacts are created.

**Acceptance Scenarios**:

1. **Given** a maintainer pushes a tag matching `v*`, **When** the workflow triggers, **Then** binaries are built for all 5 platform/arch combinations
2. **Given** a release build succeeds, **When** artifacts are published, **Then** a GitHub Release is created with auto-generated changelog from commit history
3. **Given** a release is published, **When** inspecting the Release, **Then** checksums (SHA256) are included for all binary archives
4. **Given** a release workflow fails, **When** examining the failure, **Then** the maintainer can re-run the workflow without creating duplicate releases

---

### User Story 6 - Verify Installation Integrity (Priority: P3)

A security-conscious user wants to verify that the downloaded binary matches what was published. They need checksums and optionally signatures.

**Why this priority**: Security verification is important but not blocking for initial adoption. Most users trust GitHub's HTTPS delivery.

**Independent Test**: Can be tested by downloading checksum file and verifying with `sha256sum -c`.

**Acceptance Scenarios**:

1. **Given** a downloaded binary archive, **When** the user downloads the corresponding checksums file, **Then** they can verify integrity with `sha256sum -c checksums.txt`
2. **Given** the checksums file, **When** inspected, **Then** it contains SHA256 hashes for all platform archives

---

### Edge Cases

- What happens when a user tries to install on an unsupported platform (e.g., FreeBSD)?
  - *Expected*: Clear error message indicating supported platforms
- What happens when GitHub Releases is temporarily unavailable?
  - *Expected*: Homebrew and Docker registries serve as alternate sources
- What happens when a release build partially fails (some platforms succeed, others fail)?
  - *Expected*: Workflow fails entirely; no partial releases are published
- What happens when a user has an old Homebrew formula cache?
  - *Expected*: `brew update` fetches the new formula before install

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST build binaries for macOS (arm64, amd64), Linux (amd64, arm64), and Windows (amd64)
- **FR-002**: System MUST trigger release workflow on git tags matching pattern `v*` (e.g., v1.0.0, v1.2.3-beta.1)
- **FR-003**: System MUST publish binaries to GitHub Releases with auto-generated changelog
- **FR-004**: System MUST generate SHA256 checksums for all binary archives
- **FR-005**: System MUST publish Docker images to ghcr.io with `:latest` and version tags
- **FR-006**: System MUST update Homebrew formula automatically when releases are published
- **FR-007**: System MUST include shell completion scripts (bash, zsh, fish) in release archives
- **FR-008**: System MUST embed version information in binary at build time
- **FR-009**: System MUST use semantic versioning (major.minor.patch) for releases
- **FR-010**: System MUST fail the entire release if any platform build fails (no partial releases)
- **FR-011**: Docker image MUST use a minimal base image for security and small size

### Key Entities

- **Release**: A versioned distribution of dsops (version tag, binaries, checksums, changelog)
- **Binary Archive**: A compressed package containing the dsops binary and supporting files for a specific platform
- **Homebrew Formula**: A Ruby file defining how to install dsops via Homebrew
- **Docker Image**: An OCI container image with dsops pre-installed

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can install dsops via Homebrew with a single command in under 60 seconds
- **SC-002**: Binary downloads are available for all 5 supported platforms within 15 minutes of tagging
- **SC-003**: Docker image size is under 50MB
- **SC-004**: Release workflow completes successfully in under 10 minutes
- **SC-005**: 100% of releases have valid checksums for all binary archives
- **SC-006**: Homebrew formula is updated automatically within 1 hour of release
- **SC-007**: All installation methods result in a working `dsops --version` command

## Assumptions

- The GitHub repository will be made public (required for Homebrew, go install, and public releases)
- The organization has access to GitHub Container Registry (ghcr.io)
- A separate Homebrew tap repository (`systmms/homebrew-dsops`) will be created
- GoReleaser is the industry-standard tool for Go binary releases and will be used
- Maintainers will use conventional commit messages to enable automatic changelog generation
- Maintainers will follow semantic versioning (FR-009) by convention; GoReleaser extracts version from tag
- Code signing for macOS binaries is out of scope for initial release (may cause Gatekeeper prompts)
- Windows binaries will not be code-signed initially (may trigger SmartScreen warnings)

## Out of Scope

- Nix flake publishing to nixpkgs (existing flake.nix works locally)
- Windows package managers (Scoop, Chocolatey)
- Linux package repositories (APT, RPM, AUR)
- macOS code signing and notarization
- Windows code signing
- Automatic vulnerability scanning of Docker images (can be added later)
