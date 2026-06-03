package main

import (
	"context"
	"os/exec"
	"syscall"
)

func hiddenProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

func commandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.SysProcAttr = hiddenProcAttr()
	return cmd
}

func startCommand(ctx context.Context, cmd *exec.Cmd) (func(), error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		case <-done:
		}
	}()
	return func() { close(done) }, nil
}

func commandOutput(ctx context.Context, name string, arg ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.SysProcAttr = hiddenProcAttr()
	return cmd.Output()
}

func command(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = hiddenProcAttr()
	return cmd
}
