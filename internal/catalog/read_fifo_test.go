//go:build darwin || linux

package catalog

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRejectsFIFOMetadata(t *testing.T) {
	if root := os.Getenv("EXPLEDGER_TEST_FIFO_ROOT"); root != "" {
		_, readErr := Read(root, "pipe")
		_, listErr := List(root)
		_, createErr := Create(root, "child", time.Now(), CreateOptions{BasedOn: []string{"pipe"}})
		for operation, err := range map[string]error{"read": readErr, "list": listErr, "parent": createErr} {
			if err == nil || !strings.Contains(err.Error(), "regular file") || !strings.Contains(err.Error(), "expledger.yaml") {
				t.Errorf("%s error = %v, want non-regular metadata error", operation, err)
			}
		}
		return
	}
	for _, kind := range []string{"fifo", "relative symlink to fifo"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "experiments", "pipe")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			metadata := filepath.Join(dir, "expledger.yaml")
			fifo := metadata
			if kind != "fifo" {
				fifo = filepath.Join(dir, "metadata.pipe")
				if err := os.Symlink("metadata.pipe", metadata); err != nil {
					t.Fatal(err)
				}
			}
			if err := syscall.Mkfifo(fifo, 0600); err != nil {
				t.Fatal(err)
			}
			// Bound the regression test: opening a FIFO before checking its type blocks.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRejectsFIFOMetadata$")
			cmd.Env = append(os.Environ(), "EXPLEDGER_TEST_FIFO_ROOT="+root)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("FIFO check failed or blocked: %v (%v)\n%s", err, ctx.Err(), output)
			}
		})
	}
}
