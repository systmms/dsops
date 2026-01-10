# Feature Specification: Stdout Output Masking

**Feature Branch**: `024-stdout-masking`
**Created**: 2026-01-10
**Status**: Draft
**Input**: User description: "Add automatic secret redaction in stdout/stderr output from child processes executed via `dsops exec`. This makes dsops safe for AI agent workflows where command output is captured and analyzed."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - AI Agent Safe Execution (Priority: P1)

As a developer using AI coding assistants (Claude Code, Cursor, Copilot), I want secrets to be automatically masked in command output so that sensitive credentials never enter the AI's context window, even when commands print secrets to stdout.

**Why this priority**: This is the primary use case driving this feature. AI coding assistants capture all command output, and without masking, any API key, password, or token that appears in output becomes visible to the AI model, creating a significant security risk.

**Independent Test**: Can be fully tested by running `dsops exec --env test --mask-output -- echo $SECRET_VAR` and verifying the output shows `[REDACTED]` instead of the actual secret value.

**Acceptance Scenarios**:

1. **Given** a secret `API_KEY=sk-live-abc123` is resolved, **When** running `dsops exec --env prod --mask-output -- curl https://api.example.com` and the response contains `sk-live-abc123`, **Then** the output displays `[REDACTED]` instead of the actual value.

2. **Given** multiple secrets are resolved, **When** running a command that outputs several secrets, **Then** all secret values are replaced with `[REDACTED]` in both stdout and stderr.

3. **Given** the `--mask-output` flag is NOT provided, **When** running `dsops exec --env prod -- curl https://api.example.com`, **Then** output passes through unmodified (preserving backward compatibility).

---

### User Story 2 - Audit-Safe Logging (Priority: P2)

As a security engineer, I want command output to be automatically sanitized so that CI/CD logs, terminal recordings, and audit trails never contain plaintext secrets.

**Why this priority**: Compliance and audit requirements often mandate that secrets never appear in logs. This extends the AI agent use case to broader operational security.

**Independent Test**: Can be tested by capturing dsops exec output to a log file and verifying no resolved secrets appear in plaintext.

**Acceptance Scenarios**:

1. **Given** stdout masking is enabled, **When** command output is redirected to a file (`> output.log`), **Then** the log file contains `[REDACTED]` instead of actual secrets.

2. **Given** a CI/CD pipeline runs `dsops exec --mask-output`, **When** the pipeline logs are reviewed, **Then** no secrets are visible in the job output.

---

### User Story 3 - Real-time Streaming Output (Priority: P2)

As a developer, I want output masking to work in real-time so that long-running commands display output progressively rather than buffering until completion.

**Why this priority**: Many commands (build processes, servers, streaming APIs) produce continuous output. Buffering all output would break the user experience and make dsops unsuitable for interactive use.

**Independent Test**: Can be tested by running a command that streams output (e.g., `tail -f`) and verifying that masked output appears in real-time, not just when the command exits.

**Acceptance Scenarios**:

1. **Given** a command produces streaming output over 30 seconds, **When** running with `--mask-output`, **Then** output appears progressively (within 200ms of being generated) with secrets masked.

2. **Given** a command produces interleaved stdout and stderr, **When** running with `--mask-output`, **Then** both streams are masked and output ordering is preserved as much as possible.

---

### Edge Cases

- What happens when a secret value is split across multiple write() calls (e.g., partial secret at buffer boundary)?
- How does the system handle binary output that may contain secret byte sequences?
- What happens when the secret value is very short (1-3 characters) and may cause false positives?
- How does masking affect terminal control sequences (colors, cursor positioning)?
- What happens when the `[REDACTED]` replacement text itself appears in legitimate output?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a `--mask-output` flag on the `exec` command to enable output masking
- **FR-002**: System MUST replace all resolved secret values with `[REDACTED]` in stdout when masking is enabled
- **FR-003**: System MUST replace all resolved secret values with `[REDACTED]` in stderr when masking is enabled
- **FR-004**: System MUST process output in near real-time (streaming), not buffer until process completion
- **FR-005**: System MUST preserve the child process exit code regardless of masking
- **FR-006**: System MUST NOT modify output when `--mask-output` flag is not provided (backward compatibility)
- **FR-007**: System MUST only mask secrets that are 4 characters or longer (to avoid excessive false positives with short values)
- **FR-008**: System MUST handle secrets that span buffer boundaries (partial matches across write calls)
- **FR-009**: System MUST preserve terminal control sequences (ANSI colors, cursor movement) in output

### Non-Functional Requirements

- **NFR-001**: Masking overhead MUST NOT add more than 10ms latency per 1MB of output
- **NFR-002**: Memory usage for buffering MUST NOT exceed the length of the longest secret + reasonable buffer size

### Key Entities

- **ResolvedSecret**: A secret value that was fetched from a provider and should be masked in output
- **RedactingWriter**: An io.Writer wrapper that intercepts writes and replaces secret values before forwarding to the underlying writer

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Commands execute with identical exit codes whether masking is enabled or disabled
- **SC-002**: All resolved secret values appearing in command output are replaced with `[REDACTED]` (100% masking rate)
- **SC-003**: Streaming output appears within 200ms of being generated by the child process
- **SC-004**: Existing dsops exec users experience no change in behavior when not using the new flag
- **SC-005**: Output masking works correctly with at least 10 concurrent secrets of varying lengths

## Assumptions

- Secrets are known at execution time (resolved before the command runs)
- The `[REDACTED]` placeholder is acceptable and won't conflict with legitimate output in most use cases
- Real-time streaming is preferred over perfect ordering of stdout/stderr (some interleaving variance acceptable)
- Binary output containing secret byte sequences is an edge case that may not be perfectly handled
