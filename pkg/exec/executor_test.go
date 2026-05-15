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

func TestRealCommandExecutor_ExecuteWithEnv(t *testing.T) {
	// Note: t.Parallel is NOT called on this top-level test because the
	// "preserves parent env" and "overrides parent env" subtests rely on
	// t.Setenv, which is incompatible with parallel execution.

	executor := &RealCommandExecutor{}
	ctx := context.Background()

	t.Run("injects new env var", func(t *testing.T) {
		stdout, _, err := executor.ExecuteWithEnv(
			ctx,
			[]string{"DSOPS_TEST_VAR=hello"},
			"sh", "-c", "echo $DSOPS_TEST_VAR",
		)
		require.NoError(t, err)
		assert.Equal(t, "hello\n", string(stdout))
	})

	t.Run("preserves parent env when injecting", func(t *testing.T) {
		t.Setenv("DSOPS_PARENT_VAR", "parent_value")
		stdout, _, err := executor.ExecuteWithEnv(
			ctx,
			[]string{"DSOPS_TEST_VAR=child"},
			"sh", "-c", "echo $DSOPS_PARENT_VAR:$DSOPS_TEST_VAR",
		)
		require.NoError(t, err)
		assert.Equal(t, "parent_value:child\n", string(stdout))
	})

	t.Run("overrides parent env var", func(t *testing.T) {
		t.Setenv("DSOPS_OVERRIDE_VAR", "from_parent")
		stdout, _, err := executor.ExecuteWithEnv(
			ctx,
			[]string{"DSOPS_OVERRIDE_VAR=from_child"},
			"sh", "-c", "echo $DSOPS_OVERRIDE_VAR",
		)
		require.NoError(t, err)
		assert.Equal(t, "from_child\n", string(stdout))
	})

	t.Run("empty env behaves like Execute", func(t *testing.T) {
		stdout, _, err := executor.ExecuteWithEnv(ctx, nil, "echo", "hello")
		require.NoError(t, err)
		assert.Equal(t, "hello\n", string(stdout))
	})
}

func TestEnvCommandExecutorInterface(t *testing.T) {
	t.Parallel()

	// Verify that RealCommandExecutor implements EnvCommandExecutor
	var _ EnvCommandExecutor = &RealCommandExecutor{}
	var _ EnvCommandExecutor = (*RealCommandExecutor)(nil)
}
