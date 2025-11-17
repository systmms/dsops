package exec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealCommandExecutor_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		command     string
		args        []string
		wantSuccess bool
		wantOutput  string
	}{
		{
			name:        "echo command",
			command:     "echo",
			args:        []string{"hello"},
			wantSuccess: true,
			wantOutput:  "hello\n",
		},
		{
			name:        "command with multiple args",
			command:     "echo",
			args:        []string{"hello", "world"},
			wantSuccess: true,
			wantOutput:  "hello world\n",
		},
		{
			name:        "command without args",
			command:     "echo",
			args:        []string{},
			wantSuccess: true,
			wantOutput:  "\n",
		},
		{
			name:        "invalid command",
			command:     "nonexistent_command_xyz123",
			args:        []string{},
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			executor := &RealCommandExecutor{}
			ctx := context.Background()

			stdout, stderr, err := executor.Execute(ctx, tt.command, tt.args...)

			if tt.wantSuccess {
				require.NoError(t, err)
				assert.Equal(t, tt.wantOutput, string(stdout))
				assert.Empty(t, stderr)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestRealCommandExecutor_ContextCancellation(t *testing.T) {
	t.Parallel()

	executor := &RealCommandExecutor{}
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately
	cancel()

	// Execute should fail due to canceled context
	_, _, err := executor.Execute(ctx, "sleep", "10")
	assert.Error(t, err)
}

func TestDefaultExecutor(t *testing.T) {
	t.Parallel()

	executor := DefaultExecutor()
	require.NotNil(t, executor)

	// Verify it's a RealCommandExecutor
	_, ok := executor.(*RealCommandExecutor)
	assert.True(t, ok, "DefaultExecutor should return a *RealCommandExecutor")
}

func TestCommandExecutorInterface(t *testing.T) {
	t.Parallel()

	// Verify that RealCommandExecutor implements CommandExecutor
	var _ CommandExecutor = &RealCommandExecutor{}
	var _ CommandExecutor = (*RealCommandExecutor)(nil)
}

func TestRealCommandExecutor_StderrCapture(t *testing.T) {
	t.Parallel()

	executor := &RealCommandExecutor{}
	ctx := context.Background()

	// Use a command that writes to stderr
	// 'sh -c' allows us to redirect output
	stdout, stderr, err := executor.Execute(ctx, "sh", "-c", "echo 'stdout' && echo 'stderr' >&2")

	require.NoError(t, err)
	assert.Equal(t, "stdout\n", string(stdout))
	assert.Equal(t, "stderr\n", string(stderr))
}
