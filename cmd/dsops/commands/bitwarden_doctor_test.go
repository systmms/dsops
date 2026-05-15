package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/internal/resolve"
	"github.com/systmms/dsops/pkg/adapter"
)

func TestRenderBitwardenAccounts_SharedAppDataDir_Warns(t *testing.T) {
	infos := []providers.BitwardenAccountInfo{
		{
			Name:           "bw-personal",
			AppDataDir:     "/home/op/.config/dsops/shared",
			Server:         "https://vault.bitwarden.com",
			Email:          "alice@example.com",
			ObservedEmail:  "alice@example.com",
			ObservedStatus: "unlocked",
		},
		{
			Name:           "bw-work",
			AppDataDir:     "/home/op/.config/dsops/shared",
			Server:         "https://vw.example.com",
			Email:          "alice@corp.example.com",
			ObservedEmail:  "alice@corp.example.com",
			ObservedStatus: "unlocked",
		},
	}

	var buf bytes.Buffer
	renderBitwardenAccounts(&buf, infos)
	out := buf.String()

	assert.Contains(t, out, "provider bw-personal")
	assert.Contains(t, out, "provider bw-work")
	assert.Contains(t, out, "/home/op/.config/dsops/shared")
	assert.Contains(t, out, "https://vault.bitwarden.com")
	assert.Contains(t, out, "https://vw.example.com")
	assert.Contains(t, out, "alice@example.com")
	assert.Contains(t, out, "(status: unlocked)")

	// Both providers must reference each other as colliders.
	// Personal block warns about work; work block warns about personal.
	bwPersonalIdx := strings.Index(out, "provider bw-personal")
	bwWorkIdx := strings.Index(out, "provider bw-work")
	personalBlock := out[bwPersonalIdx:bwWorkIdx]
	workBlock := out[bwWorkIdx:]

	assert.Contains(t, personalBlock, "⚠ shared with: bw-work")
	assert.Contains(t, workBlock, "⚠ shared with: bw-personal")
}

func TestRenderBitwardenAccounts_DistinctAppDataDirs_NoWarning(t *testing.T) {
	infos := []providers.BitwardenAccountInfo{
		{
			Name:       "bw-a",
			AppDataDir: "/tmp/a",
			Server:     "https://vault.bitwarden.com",
		},
		{
			Name:       "bw-b",
			AppDataDir: "/tmp/b",
			Server:     "https://vault.bitwarden.com",
		},
	}

	var buf bytes.Buffer
	renderBitwardenAccounts(&buf, infos)
	out := buf.String()

	assert.NotContains(t, out, "⚠ shared with")
	assert.Contains(t, out, "/tmp/a")
	assert.Contains(t, out, "/tmp/b")
}

func TestRenderBitwardenAccounts_UnsetFields_RenderAsPlaceholder(t *testing.T) {
	infos := []providers.BitwardenAccountInfo{
		{Name: "bw-default"},
	}

	var buf bytes.Buffer
	renderBitwardenAccounts(&buf, infos)
	out := buf.String()

	assert.Contains(t, out, "appDataDir: <unset>")
	assert.Contains(t, out, "server:     <unset>")
	assert.Contains(t, out, "email:      <unset>")
}

func TestRenderBitwardenAccounts_NoBitwardenProviders_NoOutput(t *testing.T) {
	var buf bytes.Buffer
	renderBitwardenAccounts(&buf, nil)
	assert.Empty(t, buf.String())
}

// TestRenderBitwardenAccounts_ObservedEmailPreferredOverConfigured verifies
// that when only the observed email is known (configured is unset), doctor
// still surfaces the authenticated identity rather than rendering <unset>.
// Codex finding on bitwarden_doctor.go:71.
func TestRenderBitwardenAccounts_ObservedEmailPreferredOverConfigured(t *testing.T) {
	infos := []providers.BitwardenAccountInfo{
		{
			Name:           "bw-default",
			AppDataDir:     "/tmp/a",
			ObservedEmail:  "observed-only@example.com",
			ObservedStatus: "unlocked",
			// Email (configured) intentionally left empty.
		},
	}

	var buf bytes.Buffer
	renderBitwardenAccounts(&buf, infos)
	out := buf.String()

	assert.Contains(t, out, "observed-only@example.com", "observed email must be surfaced when configured email is unset")
	assert.NotContains(t, out, "email:      <unset>", "must not render <unset> when bw status knows the user's email")
}

// TestCollectBitwardenAccountInfo_DeduplicatesNames verifies that a name
// declared in both SecretStores AND Providers maps appears once, not twice.
// Gemini finding on bitwarden_doctor.go:40.
func TestCollectBitwardenAccountInfo_DeduplicatesNames(t *testing.T) {
	cfg := &config.Config{
		Definition: &config.Definition{
			SecretStores: map[string]config.SecretStoreConfig{
				"bw-shared": {Type: "bitwarden"},
			},
			Providers: map[string]config.ProviderConfig{
				"bw-shared": {Type: "bitwarden"},
			},
		},
	}

	resolver := resolve.New(&config.Config{Definition: cfg.Definition, Logger: logging.New(false, true)})
	tmp := t.TempDir()
	require.NoError(t, ensureTestDir(tmp))
	dir := tmp + "/leaf"
	require.NoError(t, ensureTestDirParent(dir))
	p, err := providers.NewBitwardenProviderFactory("bw-shared", map[string]any{
		"appDataDir": dir,
	})
	require.NoError(t, err)
	resolver.RegisterProvider("bw-shared", p)

	infos := collectBitwardenAccountInfo(resolver, cfg)
	assert.Len(t, infos, 1, "names duplicated across SecretStores and Providers must dedupe to one entry")
}

// TestCollectBitwardenAccountInfo_UnwrapsSecretStoreAdapter verifies that a
// Bitwarden provider registered under `secretStores:` (which gets wrapped by
// SecretStoreToProviderAdapter -> ProviderToSecretStoreAdapter -> BitwardenProvider)
// still shows up in the doctor block. Codex finding on bitwarden_doctor.go:37.
func TestCollectBitwardenAccountInfo_UnwrapsSecretStoreAdapter(t *testing.T) {
	cfg := &config.Config{
		Definition: &config.Definition{
			SecretStores: map[string]config.SecretStoreConfig{
				"bw-via-secretstores": {Type: "bitwarden"},
			},
		},
	}

	resolver := resolve.New(&config.Config{Definition: cfg.Definition, Logger: logging.New(false, true)})
	tmp := t.TempDir()
	require.NoError(t, ensureTestDir(tmp))
	dir := tmp + "/leaf"
	require.NoError(t, ensureTestDirParent(dir))
	bw, err := providers.NewBitwardenProviderFactory("bw-via-secretstores", map[string]any{
		"appDataDir": dir,
		"email":      "alice@example.com",
	})
	require.NoError(t, err)
	// Reproduce the wrap chain that the secretStores registry applies.
	wrapped := adapter.NewSecretStoreToProviderAdapter(adapter.NewProviderToSecretStoreAdapter(bw))
	resolver.RegisterProvider("bw-via-secretstores", wrapped)

	infos := collectBitwardenAccountInfo(resolver, cfg)
	require.Len(t, infos, 1, "adapter-wrapped Bitwarden provider must be discoverable")
	assert.Equal(t, "bw-via-secretstores", infos[0].Name)
	assert.Equal(t, "alice@example.com", infos[0].Email)
}

// ensureTestDir is a tiny helper for tests in this file that don't need the
// full t.TempDir lifecycle; it just ensures the path exists.
func ensureTestDir(path string) error {
	// t.TempDir paths always exist; this stays a no-op so the helper has a
	// stable signature if we want to extend later.
	_ = path
	return nil
}

func ensureTestDirParent(_ string) error { return nil }
