package toolchain

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const defaultTimeout = 30 * time.Second

// CommandResult holds the output of an executed command.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// RunCommand executes a command with a default 30s timeout and captures output.
func RunCommand(ctx context.Context, name string, args ...string) (*CommandResult, error) {
	return RunCommandEnv(ctx, nil, name, args...)
}

// RunCommandEnv executes a command with additional environment variables.
func RunCommandEnv(ctx context.Context, env map[string]string, name string, args ...string) (*CommandResult, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for key, value := range env {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		return result, fmt.Errorf("command %q failed (exit %d): %w", name, result.ExitCode, err)
	}

	return result, nil
}
