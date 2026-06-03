//go:build !windows

package main

import (
	"bytes"
	"context"
	"os/exec"
	"syscall"
)

func commandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-done:
		}
	}()
	return func() { close(done) }, nil
}

func commandOutput(ctx context.Context, name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-done:
		}
	}()
	err := cmd.Wait()
	close(done)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &exec.ExitError{ProcessState: cmd.ProcessState, Stderr: stderr.Bytes()}
	}
	return stdout.Bytes(), nil
}

func command(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}
