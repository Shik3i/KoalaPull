package main

import (
	"context"
	"os/exec"
	"syscall"
)

func commandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	return cmd
}

func command(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}
