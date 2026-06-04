//go:build !windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
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
	var doneOnce sync.Once
	cleanupDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-done:
		}
		close(cleanupDone)
	}()
	return func() {
		doneOnce.Do(func() { close(done) })
		<-cleanupDone
	}, nil
}

func commandOutput(ctx context.Context, name string, arg ...string) ([]byte, error) {
	return commandOutputLimited(ctx, maxCommandOutputBytes, name, arg...)
}

func commandOutputLimited(ctx context.Context, maxBytes int64, name string, arg ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout := newLimitedBuffer(maxBytes)
	stderr := newLimitedBuffer(maxBytes)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	cleanupDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-done:
		}
		close(cleanupDone)
	}()
	err := cmd.Wait()
	close(done)
	<-cleanupDone
	if stdout.Exceeded() || stderr.Exceeded() {
		err = errors.Join(err, errCommandOutputTooLarge)
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitErr.Stderr = stderr.Bytes()
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

func command(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

func (a *App) IsBrowserRunning(browser string) (bool, error) {
	processes, ok := browserProcessNames[strings.ToLower(browser)]
	if !ok {
		return false, fmt.Errorf("unknown browser: %s", browser)
	}
	for _, proc := range processes.unix {
		ctx, cancel := context.WithTimeout(a.appContext(), 3*time.Second)
		out, err := commandOutput(ctx, "pgrep", "-x", proc)
		cancel()
		if err == nil {
			if len(strings.TrimSpace(string(out))) > 0 {
				return true, nil
			}
			continue
		}
		if !isNoMatchingProcessError(err) {
			return false, fmt.Errorf("check browser process %s: %w", proc, err)
		}
	}
	return false, nil
}

func isNoMatchingProcessError(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}

func (a *App) KillBrowser(browser string) error {
	processes, ok := browserProcessNames[strings.ToLower(browser)]
	if !ok {
		return fmt.Errorf("unknown browser: %s", browser)
	}
	var killErrors []error
	for _, proc := range processes.unix {
		ctx, cancel := context.WithTimeout(a.appContext(), 5*time.Second)
		_, err := commandOutput(ctx, "pkill", "-x", proc)
		cancel()
		if err != nil && !isNoMatchingProcessError(err) {
			killErrors = append(killErrors, fmt.Errorf("kill browser process %s: %w", proc, err))
		}
	}
	return errors.Join(killErrors...)
}
