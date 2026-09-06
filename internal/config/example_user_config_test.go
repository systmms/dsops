package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/logging"
)

// TestConfig_LoadsUserConfigExample verifies that the committed
// examples/user-config.yaml parses through the same loader the CLI uses and
// that the portable project example resolves its store names from it.
func TestConfig_LoadsUserConfigExample(t *testing.T) {
	t.Parallel()

	root, err := findRepoRoot()
	require.NoError(t, err)

	projectPath := filepath.Join(root, "examples", "portable-project.yaml")
	userPath := filepath.Join(root, "examples", "user-config.yaml")

	cfg := &Config{
		Path:       projectPath,
		Logger:     logging.New(false, false),
		UserConfig: UserConfigSpec{Path: userPath, Origin: UserConfigOriginFlag},
	}
	require.NoError(t, cfg.Load(), "examples must parse cleanly")

	require.NotNil(t, cfg.Definition)
	assert.Equal(t, userPath, cfg.LoadedUserConfigPath)
	assert.Empty(t, cfg.ShadowedUserStores)

	for _, name := range []string{"work-vault", "bw-work", "aws-work"} {
		store, ok := cfg.Definition.SecretStores[name]
		require.True(t, ok, "store %s should come from the user config", name)
		assert.NotEmpty(t, store.Type)
		src, ok := cfg.StoreSource(name)
		require.True(t, ok)
		assert.Equal(t, StoreScopeUser, src.Scope)
	}
	assert.Equal(t, "keychain", cfg.Definition.Providers["local-keychain"].Type)

	// Every store referenced by the portable project is now resolvable.
	env, err := cfg.GetEnvironment("dev")
	require.NoError(t, err)
	for varName, variable := range env {
		if variable.From == nil {
			continue
		}
		_, err := cfg.GetProvider(variable.From.GetEffectiveProvider())
		assert.NoError(t, err, "variable %s", varName)
	}
}

// TestConfig_PortableProjectWithoutUserConfig_NamesTheUserFile verifies the
// contributor experience when no machine-level config exists yet: the error
// tells them where to create it.
func TestConfig_PortableProjectWithoutUserConfig_NamesTheUserFile(t *testing.T) {
	t.Parallel()

	root, err := findRepoRoot()
	require.NoError(t, err)

	missing := filepath.Join(t.TempDir(), "config.yaml")
	cfg := &Config{
		Path:       filepath.Join(root, "examples", "portable-project.yaml"),
		UserConfig: UserConfigSpec{Path: missing, Origin: UserConfigOriginDefault},
	}
	require.NoError(t, cfg.Load())

	_, err = cfg.GetProvider("work-vault")
	require.Error(t, err)
	assert.Contains(t, err.Error(), missing)
}
