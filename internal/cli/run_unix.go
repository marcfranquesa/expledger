//go:build darwin || linux

package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func executeRun(ctx context.Context, cmd *exec.Cmd, launched func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Bound waits for os/exec's pipes if a descendant retains an output stream.
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch run.sh: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	interruption := launched()
	var waitErr error
	if interruption == nil {
		select {
		case waitErr = <-done:
		case <-ctx.Done():
			interruption = &workloadExit{130}
		}
	}
	groupStopped := false
	if interruption != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
		timer := time.NewTimer(time.Second)
		select {
		case waitErr = <-done:
		case <-timer.C:
			if err := stopRunGroup(cmd.Process.Pid); err != nil {
				return errors.Join(interruption, err)
			}
			groupStopped = true
			waitErr = <-done
		}
		timer.Stop()
	}
	var groupErr error
	if !groupStopped {
		groupErr = stopRunGroup(cmd.Process.Pid)
	}
	if interruption != nil {
		return errors.Join(interruption, groupErr)
	}
	if waitErr != nil {
		var exit *exec.ExitError
		if errors.As(waitErr, &exit) {
			code := exit.ExitCode()
			if status, ok := exit.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				code = 128 + int(status.Signal())
			}
			waitErr = &workloadExit{code}
		}
	}
	return errors.Join(waitErr, groupErr)
}

func stopRunGroup(pid int) error {
	// Signal ordinary descendants that remain after the entrypoint exits.
	if err := syscall.Kill(-pid, syscall.SIGKILL); errors.Is(err, syscall.ESRCH) {
		return nil
	} else if err != nil {
		// macOS can report EPERM for an orphaned group containing only zombies.
		// Do not mask failures to stop live processes with different owners.
		if runtime.GOOS == "darwin" && errors.Is(err, syscall.EPERM) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			probe := exec.CommandContext(ctx, "/bin/ps", "-axo", "pgid=,stat=")
			if output, probeErr := probe.Output(); probeErr == nil && onlyZombieGroup(string(output), pid) {
				return nil
			}
			if errors.Is(syscall.Kill(-pid, 0), syscall.ESRCH) {
				return nil
			}
		}
		return fmt.Errorf("stop workload process group: %w", err)
	}
	return nil
}

func onlyZombieGroup(snapshot string, pid int) bool {
	found := false
	for _, line := range strings.Split(strings.TrimSpace(snapshot), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return false
		}
		group, err := strconv.Atoi(fields[0])
		if err != nil {
			return false
		}
		if group == pid {
			found = true
			if !strings.HasPrefix(fields[1], "Z") {
				return false
			}
		}
	}
	return found
}
