package main

import (
	"bytes"
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = hiddenProcAttr()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	started := make(chan error, 1)
	type result struct {
		output []byte
		err    error
	}
	done := make(chan result, 1)
	// Start and Wait can block on Windows, so keep the cancellation path independent.
	go func() {
		if err := cmd.Start(); err != nil {
			started <- err
			done <- result{err: err}
			return
		}
		started <- nil
		err := cmd.Wait()
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitErr.Stderr = stderr.Bytes()
		}
		done <- result{output: stdout.Bytes(), err: err}
	}()
	select {
	case res := <-done:
		if res.err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, res.err
		}
		return res.output, nil
	case <-ctx.Done():
		go func() {
			if err := <-started; err == nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}()
		return nil, ctx.Err()
	}
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
