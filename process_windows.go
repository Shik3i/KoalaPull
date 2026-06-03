package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
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

func (a *App) IsBrowserRunning(browser string) (bool, error) {
	processes, ok := browserProcessNames[strings.ToLower(browser)]
	if !ok {
		return false, nil
	}
	for _, proc := range processes.windows {
		ctx, cancel := context.WithTimeout(a.appContext(), 3*time.Second)
		out, err := commandOutput(ctx, "tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", proc), "/FO", "CSV", "/NH")
		cancel()
		if err == nil && strings.Contains(strings.ToLower(string(out)), strings.ToLower(proc)) {
			return true, nil
		}
	}
	return false, nil
}

func (a *App) KillBrowser(browser string) error {
	processes, ok := browserProcessNames[strings.ToLower(browser)]
	if !ok {
		return fmt.Errorf("unknown browser: %s", browser)
	}
	var lastErr error
	for _, proc := range processes.windows {
		ctx, cancel := context.WithTimeout(a.appContext(), 5*time.Second)
		_, err := commandOutput(ctx, "taskkill", "/F", "/IM", proc)
		cancel()
		if err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				lastErr = err
			}
		}
	}
	return lastErr
}
