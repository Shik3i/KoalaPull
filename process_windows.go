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
	"unsafe"

	"golang.org/x/sys/windows"
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
	job, err := startCommandInJob(cmd)
	if err != nil {
		return nil, err
	}
	done := make(chan struct{})
	var doneOnce sync.Once
	var jobOnce sync.Once
	closeJob := func() {
		jobOnce.Do(func() { _ = windows.CloseHandle(job) })
	}
	go func() {
		select {
		case <-ctx.Done():
			closeJob()
		case <-done:
		}
	}()
	return func() {
		doneOnce.Do(func() { close(done) })
		closeJob()
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
	cmd.SysProcAttr = hiddenProcAttr()
	stdout := newLimitedBuffer(maxBytes)
	stderr := newLimitedBuffer(maxBytes)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	type result struct {
		output []byte
		err    error
	}
	type startResult struct {
		job windows.Handle
		err error
	}
	started := make(chan startResult, 1)
	done := make(chan result, 1)
	// Start and Wait can block on Windows, so keep cancellation independent.
	go func() {
		job, err := startCommandInJob(cmd)
		started <- startResult{job: job, err: err}
		if err != nil {
			done <- result{err: err}
			return
		}
		err = cmd.Wait()
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitErr.Stderr = stderr.Bytes()
		}
		if stdout.Exceeded() || stderr.Exceeded() {
			err = errors.Join(err, errCommandOutputTooLarge)
		}
		done <- result{output: stdout.Bytes(), err: err}
	}()
	select {
	case res := <-done:
		start := <-started
		if start.job != 0 {
			_ = windows.CloseHandle(start.job)
		}
		if res.err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, res.err
		}
		return res.output, nil
	case <-ctx.Done():
		go func() {
			start := <-started
			if start.job != 0 {
				_ = windows.CloseHandle(start.job)
			}
		}()
		return nil, ctx.Err()
	}
}

func startCommandInJob(cmd *exec.Cmd) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("create job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return 0, fmt.Errorf("configure job object: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	processHandle, openErr := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(cmd.Process.Pid),
	)
	if openErr == nil {
		err = windows.AssignProcessToJobObject(job, processHandle)
		_ = windows.CloseHandle(processHandle)
	} else {
		err = openErr
	}
	if err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		_ = windows.CloseHandle(job)
		return 0, fmt.Errorf("assign process to job object: %w", err)
	}
	return job, nil
}

func command(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = hiddenProcAttr()
	return cmd
}

func openFileWithDefaultApp(path string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	cwd := (*uint16)(nil)
	return windows.ShellExecute(0, verb, file, nil, cwd, 1)
}

func (a *App) IsBrowserRunning(browser string) (bool, error) {
	processes, ok := browserProcessNames[strings.ToLower(browser)]
	if !ok {
		return false, fmt.Errorf("unknown browser: %s", browser)
	}
	for _, proc := range processes.windows {
		running, err := a.isWindowsProcessRunning(proc)
		if err != nil {
			return false, fmt.Errorf("check browser process %s: %w", proc, err)
		}
		if running {
			return true, nil
		}
	}
	return false, nil
}

func (a *App) isWindowsProcessRunning(proc string) (bool, error) {
	ctx, cancel := context.WithTimeout(a.appContext(), 3*time.Second)
	defer cancel()
	out, err := commandOutput(ctx, "tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", proc), "/FO", "CSV", "/NH")
	if err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(proc)), nil
}

func (a *App) KillBrowser(browser string) error {
	if _, ok := browserProcessNames[strings.ToLower(browser)]; !ok {
		return fmt.Errorf("unknown browser: %s", browser)
	}
	return errors.New("automatic browser termination is disabled; close the browser manually")
}

func (a *App) killBrowserForTests(browser string) error {
	processes, ok := browserProcessNames[strings.ToLower(browser)]
	if !ok {
		return fmt.Errorf("unknown browser: %s", browser)
	}
	var killErrors []error
	for _, proc := range processes.windows {
		ctx, cancel := context.WithTimeout(a.appContext(), 5*time.Second)
		_, err := commandOutput(ctx, "taskkill", "/F", "/IM", proc)
		cancel()
		if err == nil {
			continue
		}
		running, checkErr := a.isWindowsProcessRunning(proc)
		if checkErr != nil {
			killErrors = append(killErrors, errors.Join(fmt.Errorf("kill browser process %s: %w", proc, err), checkErr))
		} else if running {
			killErrors = append(killErrors, fmt.Errorf("kill browser process %s: %w", proc, err))
		}
	}
	return errors.Join(killErrors...)
}
