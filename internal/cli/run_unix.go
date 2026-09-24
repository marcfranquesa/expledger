//go:build darwin || linux

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func lockRun(root *os.Root, path string) (func() error, error) {
	f, err := root.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errors.New("another run of this experiment is active")
		}
		return nil, err
	}
	// Keep the inode: unlinking a lock file would allow two independent locks.
	return f.Close, nil
}

// executeRun returns whether the checkout can be removed after waiting for the
// entrypoint and stopping the ordinary children in its process group.
func executeRun(ctx context.Context, cmd *exec.Cmd, launched func() error) (bool, error) {
	if err := ctx.Err(); err != nil {
		return true, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Bound waits for os/exec's pipes if a descendant retains an output stream.
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		return true, fmt.Errorf("launch run.sh: %w", err)
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
				return false, errors.Join(interruption, err)
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
		return groupErr == nil, errors.Join(interruption, groupErr)
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
	return groupErr == nil, errors.Join(waitErr, groupErr)
}

func stopRunGroup(pid int) error {
	// Grandchildren cannot be reaped here. A successful SIGKILL stops their
	// execution; probing group existence would also count already-dead zombies.
	if err := syscall.Kill(-pid, syscall.SIGKILL); errors.Is(err, syscall.ESRCH) {
		return nil
	} else if err != nil {
		// macOS can report EPERM for an orphaned group containing only zombies.
		// Check states before allowing cleanup; a live process with another owner
		// must still produce a failure and keep its checkout.
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
