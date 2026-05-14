package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systmms/dsops/internal/providers"
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
