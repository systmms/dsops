package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
)

func TestNewLeakCommand(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewLeakCommand(cfg)

	assert.Equal(t, "leak", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify subcommands exist
	subcommands := cmd.Commands()
	assert.GreaterOrEqual(t, len(subcommands), 5)

	// Find expected subcommands
	expectedSubcommands := []string{"report", "list", "show", "update", "resolve"}
	for _, expected := range expectedSubcommands {
		found := false
		for _, sub := range subcommands {
			if sub.Use == expected || sub.Name() == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %s should exist", expected)
	}
}

func TestNewLeakReportCommand(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewLeakReportCommand(cfg)

	assert.Equal(t, "report", cmd.Use)
	assert.NotEmpty(t, cmd.Short)

	// Verify flags exist
	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("type"))
	assert.NotNil(t, flags.Lookup("severity"))
	assert.NotNil(t, flags.Lookup("title"))
	assert.NotNil(t, flags.Lookup("description"))
	assert.NotNil(t, flags.Lookup("file"))
	assert.NotNil(t, flags.Lookup("secret"))
	assert.NotNil(t, flags.Lookup("commit"))
	assert.NotNil(t, flags.Lookup("notify"))
}

func TestNewLeakListCommand(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewLeakListCommand(cfg)

	assert.Equal(t, "list", cmd.Use)
	assert.NotEmpty(t, cmd.Short)

	// Verify flags exist
	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("all"))
	assert.NotNil(t, flags.Lookup("type"))
	assert.NotNil(t, flags.Lookup("status"))
}

func TestNewLeakShowCommand(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewLeakShowCommand(cfg)

	assert.Equal(t, "show [incident-id]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestNewLeakUpdateCommand(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewLeakUpdateCommand(cfg)

	assert.Equal(t, "update [incident-id]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)

	// Verify flags exist
	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("status"))
	assert.NotNil(t, flags.Lookup("action"))
	assert.NotNil(t, flags.Lookup("file"))
	assert.NotNil(t, flags.Lookup("secret"))
	assert.NotNil(t, flags.Lookup("commit"))
	assert.NotNil(t, flags.Lookup("notify"))
}

func TestNewLeakResolveCommand(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewLeakResolveCommand(cfg)

	assert.Equal(t, "resolve [incident-id]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)

	// Verify flags exist
	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("notes"))
	assert.NotNil(t, flags.Lookup("notify"))
}

func TestIsValidSeverity(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		severity string
		expected bool
	}{
		{"critical", true},
		{"high", true},
		{"medium", true},
		{"low", true},
		{"invalid", false},
		{"", false},
		{"HIGH", false}, // case-sensitive
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.severity, func(t *testing.T) {
			t.Parallel()
			result := isValidSeverity(tc.severity)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsValidIncidentType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		incidentType string
		expected     bool
	}{
		{"secret-leak", true},
		{"credential-exposure", true},
		{"policy-violation", true},
		{"suspicious-activity", true},
		{"invalid-type", false},
		{"", false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.incidentType, func(t *testing.T) {
			t.Parallel()
			result := isValidIncidentType(tc.incidentType)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsValidStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		status   string
		expected bool
	}{
		{"open", true},
		{"investigating", true},
		{"resolved", true},
		{"invalid", false},
		{"", false},
		{"OPEN", false}, // case-sensitive
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.status, func(t *testing.T) {
			t.Parallel()
			result := isValidStatus(tc.status)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetStandardActions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		incidentType string
		severity     string
		minActions   int
	}{
		{"secret-leak", "critical", 5},
		{"secret-leak", "high", 5},
		{"secret-leak", "medium", 5},
		{"credential-exposure", "high", 4},
		{"policy-violation", "medium", 4},
		{"suspicious-activity", "low", 4},
	}

	for _, tc := range testCases {
		tc := tc
		name := tc.incidentType + "_" + tc.severity
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			actions := getStandardActions(tc.incidentType, tc.severity)
			assert.GreaterOrEqual(t, len(actions), tc.minActions)
		})
	}
}

func TestGetStandardActions_CriticalIncludesNotify(t *testing.T) {
	t.Parallel()

	actions := getStandardActions("secret-leak", "critical")
	assert.Contains(t, actions[0], "Notify security team")

	actions = getStandardActions("secret-leak", "high")
	assert.Contains(t, actions[0], "Notify security team")
}
