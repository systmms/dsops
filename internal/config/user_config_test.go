package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dserrors "github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/logging"
)

// writeTempFile writes content to dir/name with mode 0600 and returns the path.
func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

const minimalProject = `version: 0
envs:
  dev:
    GREETING:
      literal: hello
`

func envLookup(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func fixedHome(home string) func() (string, error) {
	return func() (string, error) { return home, nil }
}

// --- discovery -------------------------------------------------------------

func TestResolveUserConfigPath(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	xdg := t.TempDir()
	explicitDir := t.TempDir()
	winDir := t.TempDir()

	tests := []struct {
		name       string
		flag       string
		flagSet    bool
		env        map[string]string
		goos       string
		homeErr    bool
		wantPath   string
		wantOrigin UserConfigOrigin
	}{
		{
			name:       "flag wins over env and default",
			flag:       filepath.Join(explicitDir, "flag.yaml"),
			flagSet:    true,
			env:        map[string]string{UserConfigEnvVar: filepath.Join(explicitDir, "env.yaml"), "XDG_CONFIG_HOME": xdg},
			goos:       "linux",
			wantPath:   filepath.Join(explicitDir, "flag.yaml"),
			wantOrigin: UserConfigOriginFlag,
		},
		{
			name:       "flag none disables even when env is set",
			flag:       "none",
			flagSet:    true,
			env:        map[string]string{UserConfigEnvVar: filepath.Join(explicitDir, "env.yaml")},
			goos:       "linux",
			wantPath:   "",
			wantOrigin: UserConfigOriginNone,
		},
		{
			name:       "flag NONE is case-insensitive",
			flag:       "NONE",
			flagSet:    true,
			goos:       "linux",
			wantPath:   "",
			wantOrigin: UserConfigOriginNone,
		},
		{
			name:       "flag expands tilde",
			flag:       "~/custom/dsops.yaml",
			flagSet:    true,
			goos:       "linux",
			wantPath:   filepath.Join(home, "custom", "dsops.yaml"),
			wantOrigin: UserConfigOriginFlag,
		},
		{
			name:       "env used when flag unset",
			env:        map[string]string{UserConfigEnvVar: filepath.Join(explicitDir, "env.yaml"), "XDG_CONFIG_HOME": xdg},
			goos:       "linux",
			wantPath:   filepath.Join(explicitDir, "env.yaml"),
			wantOrigin: UserConfigOriginEnv,
		},
		{
			name:       "env none disables",
			env:        map[string]string{UserConfigEnvVar: "none", "XDG_CONFIG_HOME": xdg},
			goos:       "linux",
			wantPath:   "",
			wantOrigin: UserConfigOriginNone,
		},
		{
			name:       "empty env means unset, falls through to XDG",
			env:        map[string]string{UserConfigEnvVar: "", "XDG_CONFIG_HOME": xdg},
			goos:       "linux",
			wantPath:   filepath.Join(xdg, "dsops", "config.yaml"),
			wantOrigin: UserConfigOriginDefault,
		},
		{
			name:       "relative XDG_CONFIG_HOME is ignored per spec",
			env:        map[string]string{"XDG_CONFIG_HOME": "relative/config"},
			goos:       "linux",
			wantPath:   filepath.Join(home, ".config", "dsops", "config.yaml"),
			wantOrigin: UserConfigOriginDefault,
		},
		{
			name:       "linux default is ~/.config",
			goos:       "linux",
			wantPath:   filepath.Join(home, ".config", "dsops", "config.yaml"),
			wantOrigin: UserConfigOriginDefault,
		},
		{
			name:       "darwin default is ~/.config, not ~/Library",
			goos:       "darwin",
			wantPath:   filepath.Join(home, ".config", "dsops", "config.yaml"),
			wantOrigin: UserConfigOriginDefault,
		},
		{
			name:       "darwin honours XDG_CONFIG_HOME",
			env:        map[string]string{"XDG_CONFIG_HOME": xdg},
			goos:       "darwin",
			wantPath:   filepath.Join(xdg, "dsops", "config.yaml"),
			wantOrigin: UserConfigOriginDefault,
		},
		{
			name:       "windows falls back to UserConfigDir",
			goos:       "windows",
			wantPath:   filepath.Join(winDir, "dsops", "config.yaml"),
			wantOrigin: UserConfigOriginDefault,
		},
		{
			name:       "windows honours XDG_CONFIG_HOME when set",
			env:        map[string]string{"XDG_CONFIG_HOME": xdg},
			goos:       "windows",
			wantPath:   filepath.Join(xdg, "dsops", "config.yaml"),
			wantOrigin: UserConfigOriginDefault,
		},
		{
			name:       "home lookup failure yields none",
			goos:       "linux",
			homeErr:    true,
			wantPath:   "",
			wantOrigin: UserConfigOriginNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lk := UserConfigLookup{
				Getenv:        envLookup(tt.env),
				UserHomeDir:   fixedHome(home),
				UserConfigDir: func() (string, error) { return winDir, nil },
				GOOS:          tt.goos,
			}
			if tt.homeErr {
				lk.UserHomeDir = func() (string, error) { return "", errors.New("no home") }
				lk.UserConfigDir = func() (string, error) { return "", errors.New("no config dir") }
			}

			got := ResolveUserConfigPath(tt.flag, tt.flagSet, lk)
			assert.Equal(t, tt.wantPath, got.Path)
			assert.Equal(t, tt.wantOrigin, got.Origin)
			assert.Equal(t, tt.wantOrigin == UserConfigOriginFlag || tt.wantOrigin == UserConfigOriginEnv, got.Explicit())
		})
	}
}

func TestResolveUserConfigPath_FlagSetButEmptyFallsThrough(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	lk := UserConfigLookup{Getenv: envLookup(nil), UserHomeDir: fixedHome(home), GOOS: "linux"}
	got := ResolveUserConfigPath("", true, lk)
	assert.Equal(t, UserConfigOriginDefault, got.Origin)
}

func TestDefaultUserConfigLookup_UsesRuntime(t *testing.T) {
	t.Parallel()
	lk := DefaultUserConfigLookup()
	require.NotNil(t, lk.Getenv)
	require.NotNil(t, lk.UserHomeDir)
	require.NotNil(t, lk.UserConfigDir)
	assert.Equal(t, runtime.GOOS, lk.GOOS)
}

func TestDefaultProjectConfigPath(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "dsops.yaml", DefaultProjectConfigPath(envLookup(nil)))
	assert.Equal(t, "prod.yaml", DefaultProjectConfigPath(envLookup(map[string]string{ProjectConfigEnvVar: "prod.yaml"})))
}

func TestExpandHome(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	hd := fixedHome(home)

	got, err := ExpandHome("~", hd)
	require.NoError(t, err)
	assert.Equal(t, home, got)

	got, err = ExpandHome("~/x/y.yaml", hd)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "x", "y.yaml"), got)

	got, err = ExpandHome("/abs/path.yaml", hd)
	require.NoError(t, err)
	assert.Equal(t, "/abs/path.yaml", got)

	_, err = ExpandHome("~bob/x.yaml", hd)
	assert.Error(t, err, "~user expansion is not supported")

	_, err = ExpandHome("~/x", func() (string, error) { return "", errors.New("boom") })
	assert.Error(t, err)
}

// --- Load() integration ----------------------------------------------------

func TestLoad_UserConfig_NotConfigured_BehavesAsBefore(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", `version: 0
secretStores:
  lit:
    type: literal
envs:
  dev: {}
`)
	cfg := &Config{Path: project}
	require.NoError(t, cfg.Load())

	assert.Empty(t, cfg.LoadedUserConfigPath)
	assert.Empty(t, cfg.ShadowedUserStores)
	src, ok := cfg.StoreSource("lit")
	require.True(t, ok)
	assert.Equal(t, StoreScopeProject, src.Scope)
	abs, _ := filepath.Abs(project)
	assert.Equal(t, abs, src.Path)
}

func TestLoad_UserConfig_DefaultMissing_IsSilent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	cfg := &Config{
		Path:       project,
		UserConfig: UserConfigSpec{Path: filepath.Join(dir, "missing.yaml"), Origin: UserConfigOriginDefault},
	}
	require.NoError(t, cfg.Load())
	assert.Empty(t, cfg.LoadedUserConfigPath)
}

func TestLoad_UserConfig_ExplicitMissing_Errors(t *testing.T) {
	t.Parallel()
	for _, origin := range []UserConfigOrigin{UserConfigOriginFlag, UserConfigOriginEnv} {
		t.Run(string(origin), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
			missing := filepath.Join(dir, "missing.yaml")
			cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: missing, Origin: origin}}
			err := cfg.Load()
			var ce dserrors.ConfigError
			require.ErrorAs(t, err, &ce)
			assert.Equal(t, "user-config", ce.Field)
			assert.Contains(t, err.Error(), missing)
		})
	}
}

func TestLoad_UserConfig_FillsGaps_ProjectWins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", `version: 0
secretStores:
  vault:
    type: vault
    address: https://project.example.com
services:
  pg:
    type: postgresql
providers:
  legacy:
    type: literal
envs:
  dev: {}
`)
	user := writeTempFile(t, dir, "user.yaml", `version: 0
secretStores:
  vault:
    type: literal
  pg:
    type: literal
  legacy:
    type: literal
  bw-work:
    type: bitwarden
    email: work@example.com
`)
	cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginFlag}}
	require.NoError(t, cfg.Load())

	// Project definitions untouched.
	assert.Equal(t, "vault", cfg.Definition.SecretStores["vault"].Type)
	assert.Equal(t, "https://project.example.com", cfg.Definition.SecretStores["vault"].Config["address"])
	assert.Equal(t, "postgresql", cfg.Definition.Services["pg"].Type)
	assert.Equal(t, "literal", cfg.Definition.Providers["legacy"].Type)
	assert.NotContains(t, cfg.Definition.SecretStores, "pg")
	assert.NotContains(t, cfg.Definition.SecretStores, "legacy")

	// Gap filled from the user file.
	bw, ok := cfg.Definition.SecretStores["bw-work"]
	require.True(t, ok)
	assert.Equal(t, "bitwarden", bw.Type)
	assert.Equal(t, "work@example.com", bw.Config["email"])

	// Provenance.
	src, ok := cfg.StoreSource("bw-work")
	require.True(t, ok)
	assert.Equal(t, StoreScopeUser, src.Scope)
	assert.Equal(t, user, src.Path)
	src, ok = cfg.StoreSource("vault")
	require.True(t, ok)
	assert.Equal(t, StoreScopeProject, src.Scope)
	src, ok = cfg.StoreSource("pg")
	require.True(t, ok)
	assert.Equal(t, StoreScopeProject, src.Scope)
	_, ok = cfg.StoreSource("nope")
	assert.False(t, ok)

	shadowed := append([]string(nil), cfg.ShadowedUserStores...)
	sort.Strings(shadowed)
	assert.Equal(t, []string{"legacy", "pg", "vault"}, shadowed)
	assert.Equal(t, user, cfg.LoadedUserConfigPath)

	// GetProvider sees the merged store.
	pc, err := cfg.GetProvider("bw-work")
	require.NoError(t, err)
	assert.Equal(t, "bitwarden", pc.Type)
	assert.Contains(t, cfg.ListAllProviders(), "bw-work")
}

func TestLoad_UserConfig_LegacyProvidersMerged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	user := writeTempFile(t, dir, "user.yaml", `version: 0
providers:
  kc:
    type: keychain
    service_prefix: dsops
secretStores:
  lit:
    type: literal
`)
	cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginEnv}}
	require.NoError(t, cfg.Load())

	// Project had no secretStores/providers maps at all; merge must allocate them.
	require.Contains(t, cfg.Definition.Providers, "kc")
	assert.Equal(t, "keychain", cfg.Definition.Providers["kc"].Type)
	require.Contains(t, cfg.Definition.SecretStores, "lit")

	src, ok := cfg.StoreSource("kc")
	require.True(t, ok)
	assert.Equal(t, StoreScopeUser, src.Scope)
	pc, err := cfg.GetProvider("kc")
	require.NoError(t, err)
	assert.Equal(t, "dsops", pc.Config["service_prefix"])
}

func TestLoad_UserConfig_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	user := writeTempFile(t, dir, "user.yaml", "")
	cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginFlag}}
	require.NoError(t, cfg.Load())
	assert.Equal(t, user, cfg.LoadedUserConfigPath)
	assert.Empty(t, cfg.Definition.SecretStores)
}

func TestLoad_UserConfig_DisallowedSections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		keys []string
	}{
		{"envs", "envs:\n  dev: {}\n", []string{"envs"}},
		{"services", "services:\n  pg:\n    type: postgresql\n", []string{"services"}},
		{"templates", "templates:\n  - path: x\n", []string{"templates"}},
		{"transforms", "transforms:\n  t: [trim]\n", []string{"transforms"}},
		{"policies", "policies:\n  x: y\n", []string{"policies"}},
		{"notifications", "notifications:\n  x: y\n", []string{"notifications"}},
		{"metrics", "metrics:\n  x: y\n", []string{"metrics"}},
		{"typo secretstores", "secretstores:\n  a:\n    type: literal\n", []string{"secretstores"}},
		{"multiple", "envs: {}\ntemplates: []\n", []string{"envs", "templates"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
			user := writeTempFile(t, dir, "user.yaml", "version: 0\nsecretStores:\n  lit:\n    type: literal\n"+tt.body)
			cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginFlag}}
			err := cfg.Load()
			var ce dserrors.ConfigError
			require.ErrorAs(t, err, &ce)
			for _, k := range tt.keys {
				assert.Contains(t, err.Error(), "'"+k+"'")
			}
			assert.Contains(t, err.Error(), user)
			assert.Contains(t, err.Error(), "secretStores")
			assert.Nil(t, cfg.Definition, "a rejected user config must not leave a half-loaded definition")
		})
	}
}

func TestLoad_UserConfig_MalformedYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	user := writeTempFile(t, dir, "user.yaml", "secretStores: [\n")
	cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginFlag}}
	err := cfg.Load()
	var ce dserrors.ConfigError
	require.ErrorAs(t, err, &ce)
	assert.Contains(t, err.Error(), user)
}

func TestLoad_UserConfig_NotAMapping(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	user := writeTempFile(t, dir, "user.yaml", "- just\n- a list\n")
	cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginFlag}}
	err := cfg.Load()
	var ce dserrors.ConfigError
	require.ErrorAs(t, err, &ce)
}

func TestLoad_UserConfig_VersionMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	user := writeTempFile(t, dir, "user.yaml", "version: 7\n")
	cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginFlag}}
	err := cfg.Load()
	var ce dserrors.ConfigError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, "version", ce.Field)
	assert.Contains(t, err.Error(), user)
}

func TestLoad_UserConfig_PathIsDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: dir, Origin: UserConfigOriginEnv}}
	err := cfg.Load()
	var ce dserrors.ConfigError
	require.ErrorAs(t, err, &ce)
	assert.Contains(t, err.Error(), "directory")
}

func TestLoad_UserConfig_Unreadable(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits are not enforced for root or on windows")
	}
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	user := writeTempFile(t, dir, "user.yaml", "version: 0\n")
	require.NoError(t, os.Chmod(user, 0o000))
	t.Cleanup(func() { _ = os.Chmod(user, 0o600) })
	cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginFlag}}
	err := cfg.Load()
	var ue dserrors.UserError
	require.ErrorAs(t, err, &ue)
	assert.Contains(t, err.Error(), user)
}

func TestLoad_UserConfig_DuplicateNameAcrossSections(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	user := writeTempFile(t, dir, "user.yaml", `version: 0
secretStores:
  dup:
    type: literal
providers:
  dup:
    type: literal
`)
	cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginFlag}}
	err := cfg.Load()
	var ce dserrors.ConfigError
	require.ErrorAs(t, err, &ce)
	assert.Contains(t, err.Error(), "dup")
}

func TestLoad_UserConfig_ProjectMissingStillErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	user := writeTempFile(t, dir, "user.yaml", "version: 0\nsecretStores:\n  lit:\n    type: literal\n")
	cfg := &Config{Path: filepath.Join(dir, "dsops.yaml"), UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginFlag}}
	err := cfg.Load()
	var ce dserrors.ConfigError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, "path", ce.Field, "the user file is never a substitute for the project file")
}

func TestLoad_UserConfig_WarningsRecorded_NilLoggerSafe(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not meaningful on windows")
	}
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	user := writeTempFile(t, dir, "user.yaml", "version: 0\n")
	require.NoError(t, os.Chmod(user, 0o666))

	// Logger deliberately nil: Load must not panic.
	cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginFlag}}
	require.NoError(t, cfg.Load())
	require.Len(t, cfg.LoadWarnings, 1)
	assert.Contains(t, cfg.LoadWarnings[0], user)
	assert.Contains(t, cfg.LoadWarnings[0], "writable")

	// With a logger it still works (output goes to stderr, not asserted).
	cfg2 := &Config{Path: project, Logger: logging.New(true, true), UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginFlag}}
	require.NoError(t, cfg2.Load())
	assert.Len(t, cfg2.LoadWarnings, 1)
}

func TestLoad_ResetsStateBetweenCalls(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	user := writeTempFile(t, dir, "user.yaml", "version: 0\nsecretStores:\n  lit:\n    type: literal\n")
	cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: user, Origin: UserConfigOriginFlag}}
	require.NoError(t, cfg.Load())
	require.Contains(t, cfg.Definition.SecretStores, "lit")

	cfg.UserConfig = UserConfigSpec{}
	require.NoError(t, cfg.Load())
	assert.NotContains(t, cfg.Definition.SecretStores, "lit")
	assert.Empty(t, cfg.LoadedUserConfigPath)
	_, ok := cfg.StoreSource("lit")
	assert.False(t, ok)
}

// --- permission warnings ---------------------------------------------------

func TestUserConfigPermissionWarnings(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not meaningful on windows")
	}

	newFile := func(t *testing.T, dirMode, fileMode os.FileMode) string {
		t.Helper()
		parent := filepath.Join(t.TempDir(), "cfgdir")
		require.NoError(t, os.Mkdir(parent, 0o700))
		path := filepath.Join(parent, "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte("version: 0\n"), 0o600))
		require.NoError(t, os.Chmod(path, fileMode))
		require.NoError(t, os.Chmod(parent, dirMode))
		t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })
		return path
	}

	tests := []struct {
		name     string
		dirMode  os.FileMode
		fileMode os.FileMode
		want     int
	}{
		{"0600 file in 0700 dir", 0o700, 0o600, 0},
		{"0644 file in 0755 dir", 0o755, 0o644, 0},
		{"0444 nix-store style", 0o555, 0o444, 0},
		{"0664 group-writable file", 0o755, 0o664, 1},
		{"0666 world-writable file", 0o755, 0o666, 1},
		{"0622 world-writable only", 0o755, 0o622, 1},
		{"world-writable parent dir", 0o777, 0o644, 1},
		{"both writable", 0o777, 0o666, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := newFile(t, tt.dirMode, tt.fileMode)
			got := userConfigPermissionWarnings(path, os.Stat, "linux")
			assert.Len(t, got, tt.want, "%v", got)
			for _, w := range got {
				assert.Contains(t, w, "writable")
			}
		})
	}

	t.Run("symlink is checked at its target", func(t *testing.T) {
		t.Parallel()
		safe := newFile(t, 0o755, 0o444)
		unsafe := newFile(t, 0o755, 0o666)
		linkDir := t.TempDir()

		safeLink := filepath.Join(linkDir, "safe.yaml")
		require.NoError(t, os.Symlink(safe, safeLink))
		assert.Empty(t, userConfigPermissionWarnings(safeLink, os.Stat, "linux"))

		unsafeLink := filepath.Join(linkDir, "unsafe.yaml")
		require.NoError(t, os.Symlink(unsafe, unsafeLink))
		assert.Len(t, userConfigPermissionWarnings(unsafeLink, os.Stat, "linux"), 1)
	})

	t.Run("windows never warns", func(t *testing.T) {
		t.Parallel()
		path := newFile(t, 0o777, 0o666)
		assert.Empty(t, userConfigPermissionWarnings(path, os.Stat, "windows"))
	})

	t.Run("stat failure yields no warning", func(t *testing.T) {
		t.Parallel()
		failing := func(string) (os.FileInfo, error) { return nil, errors.New("boom") }
		assert.Empty(t, userConfigPermissionWarnings("/does/not/matter", failing, "linux"))
	})
}

// --- suggestions -----------------------------------------------------------

func TestMissingProviderSuggestion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	abs, _ := filepath.Abs(project)

	cfg := &Config{Path: project}
	s := cfg.MissingProviderSuggestion("work-vault")
	assert.Contains(t, s, "'work-vault'")
	assert.Contains(t, s, "secretStores:")
	assert.Contains(t, s, abs)
	assert.NotContains(t, s, "user config")

	userPath := filepath.Join(dir, "not-yet-created.yaml")
	cfg.UserConfig = UserConfigSpec{Path: userPath, Origin: UserConfigOriginDefault}
	s = cfg.MissingProviderSuggestion("work-vault")
	assert.Contains(t, s, userPath, "the hint must tell the user where to create the file even if it does not exist yet")
	assert.Contains(t, s, "user config")
}

func TestGetProvider_NotFound_MentionsUserConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	project := writeTempFile(t, dir, "dsops.yaml", minimalProject)
	userPath := filepath.Join(dir, "user.yaml")
	cfg := &Config{Path: project, UserConfig: UserConfigSpec{Path: userPath, Origin: UserConfigOriginDefault}}
	require.NoError(t, cfg.Load())

	_, err := cfg.GetProvider("work-vault")
	require.Error(t, err)
	assert.Contains(t, err.Error(), userPath)
}
