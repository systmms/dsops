package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
)

func TestNewSecretsCommand(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewSecretsCommand(cfg)

	assert.Equal(t, "secrets", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify subcommands exist
	subcommands := cmd.Commands()
	assert.GreaterOrEqual(t, len(subcommands), 3)

	// Find expected subcommands
	expectedNames := []string{"rotate", "history", "status"}
	for _, expected := range expectedNames {
		found := false
		for _, sub := range subcommands {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %s should exist", expected)
	}
}
