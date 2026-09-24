package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
)

func TestNewFromNestedDirectory(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	cwd := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	assertNew(t, cwd, root)
}

func TestNewFromGitWorktree(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	git(t, root, "-c", "user.name=ExpLedger Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "--allow-empty", "-m", "initial")
	worktree := filepath.Join(t.TempDir(), "worktree")
	git(t, root, "worktree", "add", "--quiet", "--detach", worktree)

	assertNew(t, worktree, worktree)
	if _, err := os.Stat(filepath.Join(root, "experiments")); !os.IsNotExist(err) {
		t.Fatalf("primary checkout should have no experiments directory; stat error: %v", err)
	}
}

func TestNewOutsideGit(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	if err := cli.Run([]string{"new", "my-idea"}, root, time.Now(), &stdout); err == nil {
		t.Fatal("expected error outside a Git worktree")
	}
	if _, err := os.Stat(filepath.Join(root, "experiments")); !os.IsNotExist(err) {
		t.Fatalf("failed command should not create experiments directory; stat error: %v", err)
	}
}

func TestHelp(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := cli.Run([]string{arg}, t.TempDir(), time.Now(), &stdout); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout.String(), "expledger new") {
				t.Fatalf("help does not describe the new command: %q", stdout.String())
			}
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "no command"},
		{name: "unknown command", args: []string{"unknown"}},
		{name: "missing slug", args: []string{"new"}},
		{name: "extra slug", args: []string{"new", "my-idea", "another-idea"}},
		{name: "extra help argument", args: []string{"help", "extra"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := cli.Run(tt.args, t.TempDir(), time.Now(), &stdout); err == nil {
				t.Fatalf("expected error for arguments %q", tt.args)
			}
		})
	}
}

func assertNew(t *testing.T, cwd, root string) {
	t.Helper()
	var stdout bytes.Buffer
	now := time.Date(2026, time.September, 24, 23, 30, 0, 0, time.FixedZone("local", -4*60*60))
	if err := cli.Run([]string{"new", "my-idea"}, cwd, now, &stdout); err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(resolvedRoot, "experiments", "20260924-my-idea")
	if got := strings.TrimSpace(stdout.String()); got != want {
		t.Fatalf("created path = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(want, "README.md")); err != nil {
		t.Fatalf("experiment README: %v", err)
	}
}

func git(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
