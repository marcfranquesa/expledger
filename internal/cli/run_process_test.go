//go:build darwin || linux

package cli

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestExecuteRunPreservesSignalExit(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "kill -TERM $$")
	if err := executeRun(context.Background(), cmd); ExitCode(err) != 143 {
		t.Fatal(err)
	}
}

func TestExecuteRunCanceledBeforeLaunch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := executeRun(ctx, cmd); !errors.Is(err, context.Canceled) || cmd.Process != nil {
		t.Fatalf("err=%v process=%v", err, cmd.Process)
	}
}

func TestExecuteRunCancellationStopsIgnoringChild(t *testing.T) {
	ready, writer := io.Pipe()
	defer ready.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.Command("/bin/sh", "-c", "trap '' INT; printf ready; exec sleep 30")
	cmd.Stdout = writer
	go func() { data := make([]byte, 5); _, _ = io.ReadFull(ready, data); cancel() }()
	if err := executeRun(ctx, cmd); ExitCode(err) != 130 {
		t.Fatal(err)
	}
	if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); !ok || status.Signal() != syscall.SIGKILL {
		t.Fatalf("ignored interrupt but not killed: %v", cmd.ProcessState)
	}
}

func TestExecuteRunCancellationWithCallerOwnedInput(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := context.AfterFunc(ctx, func() { reader.CloseWithError(ctx.Err()) })
	defer stop()
	cmd := exec.Command("/bin/sh", "-c", "printf ready; exec sleep 30")
	ready := make(startupOutput, 1)
	cmd.Stdout = ready
	cmd.Stdin = reader
	done := make(chan struct{})
	var err error
	go func() {
		err = executeRun(ctx, cmd)
		close(done)
	}()
	select {
	case <-ready:
	case <-done:
		t.Fatalf("workload exited before readiness: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("workload did not start")
	}
	cancel()
	select {
	case <-done:
		if ExitCode(err) != 130 {
			t.Fatalf("cancellation with caller-owned input: err=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("workload did not stop after caller closed input")
	}
}

func TestZombieGroupCheckDoesNotHideLiveOrUnknownProcesses(t *testing.T) {
	for _, tt := range []struct {
		snapshot string
		safe     bool
	}{
		{"", false}, {"7 S\n42 Z\n42 Z+\n", true}, {"7 R\n", false},
		{"42 S\n", false}, {"42 Z\n42 R+\n", false},
		{"42\n", false}, {"unknown Z\n", false},
	} {
		if got := onlyZombieGroup(tt.snapshot, 42); got != tt.safe {
			t.Errorf("snapshot %q: safe=%v, want %v", tt.snapshot, got, tt.safe)
		}
	}
}
