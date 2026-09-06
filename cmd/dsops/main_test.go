package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
)

// runPreRun executes the root command's PersistentPreRun through the
// "providers" subcommand (which tolerates a missing project config) so the
// global flags are parsed exactly as they would be for a real invocation.
func runPreRun(t *testing.T, cfg *config.Config, args ...string) {
	t.Helper()
	root := newRootCommand(cfg)
	root.SetArgs(append(args, "providers"))
	root.SetOut(nil)
	require.NoError(t, root.Execute())
}

func TestRootCommand_DefinesUserConfigFlag(t *testing.T) {
	root := newRootCommand(&config.Config{})
	f := root.PersistentFlags().Lookup("user-config")
	require.NotNil(t, f)
	assert.Equal(t, "", f.DefValue)
	assert.Contains(t, f.Usage, config.UserConfigEnvVar)
	assert.Contains(t, f.Usage, config.UserConfigDisabled)
}

func TestRootCommand_ConfigFlagHonoursEnv(t *testing.T) {
	t.Setenv(config.ProjectConfigEnvVar, "from-env.yaml")
	t.Setenv(config.UserConfigEnvVar, config.UserConfigDisabled)

	cfg := &config.Config{}
	runPreRun(t, cfg, "--config", filepath.Join(t.TempDir(), "missing.yaml"))
	assert.NotEqual(t, "from-env.yaml", cfg.Path, "explicit --config must beat DSOPS_CONFIG")

	cfg = &config.Config{}
	runPreRun(t, cfg)
	assert.Equal(t, "from-env.yaml", cfg.Path)
}

func TestRootCommand_UserConfigDisabledBySentinel(t *testing.T) {
	t.Setenv(config.UserConfigEnvVar, "")

	cfg := &config.Config{}
	runPreRun(t, cfg, "--config", filepath.Join(t.TempDir(), "missing.yaml"), "--user-config", "none")
	assert.Equal(t, config.UserConfigOriginNone, cfg.UserConfig.Origin)
	assert.Empty(t, cfg.UserConfig.Path)
}

func TestRootCommand_UserConfigFromFlag(t *testing.T) {
	t.Setenv(config.UserConfigEnvVar, "")

	want := filepath.Join(t.TempDir(), "user.yaml")
	cfg := &config.Config{}
	runPreRun(t, cfg, "--config", filepath.Join(t.TempDir(), "missing.yaml"), "--user-config", want)
	assert.Equal(t, config.UserConfigOriginFlag, cfg.UserConfig.Origin)
	assert.Equal(t, want, cfg.UserConfig.Path)
}

func TestRootCommand_UserConfigFromEnv(t *testing.T) {
	want := filepath.Join(t.TempDir(), "user.yaml")
	t.Setenv(config.UserConfigEnvVar, want)

	cfg := &config.Config{}
	runPreRun(t, cfg, "--config", filepath.Join(t.TempDir(), "missing.yaml"))
	assert.Equal(t, config.UserConfigOriginEnv, cfg.UserConfig.Origin)
	assert.Equal(t, want, cfg.UserConfig.Path)
}

func TestRootCommand_UserConfigDefaultsToXDG(t *testing.T) {
	t.Setenv(config.UserConfigEnvVar, "")
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg := &config.Config{}
	runPreRun(t, cfg, "--config", filepath.Join(t.TempDir(), "missing.yaml"))
	assert.Equal(t, config.UserConfigOriginDefault, cfg.UserConfig.Origin)
	assert.Equal(t, filepath.Join(xdg, "dsops", "config.yaml"), cfg.UserConfig.Path)
}
