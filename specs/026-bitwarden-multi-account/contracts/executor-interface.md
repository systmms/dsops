# Contract: pkg/exec — Optional EnvCommandExecutor

This feature requires per-subprocess environment injection. The contract
extends `pkg/exec` with an **optional** interface so existing
`CommandExecutor` consumers and tests remain source-compatible.

## Existing surface (unchanged)

```go
package exec

type CommandExecutor interface {
    Execute(ctx context.Context, name string, args ...string) (stdout []byte, stderr []byte, err error)
}

type RealCommandExecutor struct{}

func (r *RealCommandExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, []byte, error)

func DefaultExecutor() CommandExecutor
```

No method signature changes. No new required methods on `CommandExecutor`.

## New surface

```go
package exec

// EnvCommandExecutor extends CommandExecutor with per-call environment
// variable injection. Implementations append the supplied entries to the
// caller's existing environment (os.Environ) before spawning the child.
// Each entry is a "KEY=VALUE" string, matching exec.Cmd.Env.
type EnvCommandExecutor interface {
    CommandExecutor
    ExecuteWithEnv(ctx context.Context, env []string, name string, args ...string) (stdout []byte, stderr []byte, err error)
}
```

`RealCommandExecutor` MUST satisfy `EnvCommandExecutor`. The implementation:

```go
func (r *RealCommandExecutor) ExecuteWithEnv(
    ctx context.Context, env []string, name string, args ...string,
) ([]byte, []byte, error) {
    cmd := exec.CommandContext(ctx, name, args...)
    if len(env) > 0 {
        cmd.Env = append(os.Environ(), env...)
    }
    var stdout, stderr bytes.Buffer
    cmd.Stdout, cmd.Stderr = &stdout, &stderr
    err := cmd.Run()
    return stdout.Bytes(), stderr.Bytes(), err
}
```

When `env` is empty/nil the call is equivalent to `Execute` (no env mutation
on the child).

## Consumer usage (Bitwarden provider)

```go
func (bw *BitwardenProvider) run(ctx context.Context, args ...string) ([]byte, []byte, error) {
    env := bw.bwEnv() // returns []string like {"BITWARDENCLI_APPDATA_DIR=/..."} or nil

    if eExec, ok := bw.executor.(pkgexec.EnvCommandExecutor); ok && len(env) > 0 {
        return eExec.ExecuteWithEnv(ctx, env, "bw", args...)
    }
    return bw.executor.Execute(ctx, "bw", args...)
}
```

All existing call sites in `bitwarden.go` route through `bw.run(...)` after
this refactor.

## Backwards compatibility

- Any `CommandExecutor` mock used in tests (e.g. `tests/fakes`) is unaffected
  unless that mock opts in to also implementing `EnvCommandExecutor`. When
  `bw.bwEnv()` returns nil (no `appDataDir` configured), the provider takes
  the non-`Env` branch and behavior is byte-identical to today.
- No other provider is touched. AWS Secrets Manager, 1Password, etc. keep
  using `CommandExecutor.Execute` unchanged.

## Test mock guidance

For test cases that need to assert env propagation, the mock executor in
`tests/fakes/` (or in-test inline mock) should implement
`ExecuteWithEnv(ctx, env, name, args...)` and record the `env` slice for
assertions. The provider unit tests in this feature MUST cover:

1. `bw.bwEnv()` returns a slice containing `BITWARDENCLI_APPDATA_DIR=<abs path>`
   exactly once when `appDataDir` is configured.
2. `bw.bwEnv()` returns nil/empty when `appDataDir` is unset.
3. The provider takes the `ExecuteWithEnv` branch only when both
   `appDataDir` is configured **and** the executor satisfies
   `EnvCommandExecutor`.
4. When the executor does **not** satisfy `EnvCommandExecutor`, the provider
   logs a single warning at provider construction time and falls back to
   `Execute` (this allows tests using legacy mocks to keep working without
   becoming silently broken; the warning is suppressed when `appDataDir`
   is unset).
