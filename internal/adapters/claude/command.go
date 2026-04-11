package claude

import (
	"context"
	"os/exec"
)

func commandArgs() []string {
	return []string{"-p", "--output-format", "json"}
}

func newCommand(ctx context.Context, binaryPath string) *exec.Cmd {
	return exec.CommandContext(ctx, binaryPath, commandArgs()...)
}
