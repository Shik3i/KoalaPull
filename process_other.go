package main

import (
	"context"
	"os/exec"
)

func commandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, arg...)
}

func command(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}
