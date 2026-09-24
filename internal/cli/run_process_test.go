//go:build darwin || linux

package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestExecuteRunPreparationDoesNotPublish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tt := range []struct {
		name string
		ctx  context.Context
		cmd  *exec.Cmd
	}{
		{"canceled", ctx, exec.Command("/bin/sh", "-c", "exit 0")},
		{"missing executable", context.Background(), exec.Command("/expledger-test-no-such-entrypoint")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			published := false
			err := executeRun(tt.ctx, tt.cmd, func() error { published = true; return nil })
			if err == nil || published {
				t.Fatalf("err=%v, published=%v", err, published)
			}
		})
	}
}

func TestExecuteRunReceiptFailureStopsWorkload(t *testing.T) {
	for _, tt := range []struct{ name, prefix string }{
		{"interruptible", ""},
		{"ignores SIGINT", "trap '' INT; "},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ready, writer := io.Pipe()
			defer ready.Close()
			defer writer.Close()
			cmd := exec.Command("/bin/sh", "-c", tt.prefix+"printf ready; exec sleep 30")
			cmd.Stdout = writer
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			publishErr := errors.New("metadata is no longer writable")
			err := executeRun(ctx, cmd, func() error {
				data := make([]byte, 5)
				if _, err := io.ReadFull(ready, data); err != nil {
					return err
				}
				return publishErr
			})
			if !errors.Is(err, publishErr) || ExitCode(err) != 1 {
				t.Fatalf("receipt failure: err=%v, code=%d", err, ExitCode(err))
			}
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() && cmd.ProcessState.ExitCode() != -1 {
				t.Fatal("workload was not waited for")
			}
			if tt.prefix != "" {
				if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); !ok || status.Signal() != syscall.SIGKILL {
					t.Fatalf("workload ignored SIGINT but was not killed: %v", cmd.ProcessState)
				}
			}
		})
	}
}

func TestExecuteRunPreservesSignalExit(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "kill -TERM $$")
	published := false
	err := executeRun(context.Background(), cmd, func() error { published = true; return nil })
	if !published || ExitCode(err) != 143 {
		t.Fatalf("published=%v, err=%v", published, err)
	}
}

func TestRunGitPreservesCancellationDuringPreparation(t *testing.T) {
	root := t.TempDir()
	ready := filepath.Join(root, "ready")
	if err := os.WriteFile(filepath.Join(root, "git"), []byte("#!/bin/sh\nprintf ready > \"$1\"\nexec /bin/sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := runGit(ctx, root, ready)
		done <- err
	}()
	deadline := time.After(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("Git exited before cancellation: %v", err)
		case <-deadline:
			t.Fatal("Git did not start")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || ExitCode(err) != 130 {
			t.Fatalf("canceled Git command: %v (exit %d)", err, ExitCode(err))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Git did not stop after cancellation")
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
	cmd := exec.Command("/bin/sh", "-c", "exec sleep 30")
	cmd.Stdin = reader
	done := make(chan struct{})
	var err error
	go func() {
		err = executeRun(ctx, cmd, func() error { cancel(); return nil })
		close(done)
	}()
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
