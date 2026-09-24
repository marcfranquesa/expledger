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

func TestRootHelp(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "nil arguments"},
		{name: "empty arguments", args: []string{}},
		{name: "help command", args: []string{"help"}},
		{name: "help flag", args: []string{"--help"}},
		{name: "short help flag", args: []string{"-h"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			var stdout bytes.Buffer
			if err := cli.Run(tt.args, root, time.Now(), &stdout); err != nil {
				t.Fatal(err)
			}
			if got := stdout.String(); !strings.Contains(got, "Usage:") || !strings.Contains(got, "new") {
				t.Fatalf("root help does not list commands: %q", got)
			}
			if strings.Contains(stdout.String(), "completion") {
				t.Fatalf("root help lists an unsupported completion command: %q", stdout.String())
			}
			assertEmptyDirectory(t, root)
		})
	}
}

func TestNewHelp(t *testing.T) {
	for _, args := range [][]string{
		{"help", "new"},
		{"new", "--help"},
		{"new", "-h"},
		{"new", "my-idea", "--help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := t.TempDir()
			var stdout bytes.Buffer
			if err := cli.Run(args, root, time.Now(), &stdout); err != nil {
				t.Fatal(err)
			}
			if got := stdout.String(); !strings.Contains(got, "Usage:") || !strings.Contains(got, "expledger new <slug>") {
				t.Fatalf("new help does not describe command usage: %q", got)
			}
			assertEmptyDirectory(t, root)
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "unknown command", args: []string{"unknown"}},
		{name: "unknown root flag", args: []string{"--unknown"}},
		{name: "unknown new flag", args: []string{"new", "my-idea", "--unknown"}},
		{name: "missing slug", args: []string{"new"}},
		{name: "extra slug", args: []string{"new", "my-idea", "another-idea"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			var stdout bytes.Buffer
			if err := cli.Run(tt.args, root, time.Now(), &stdout); err == nil {
				t.Fatalf("expected error for arguments %q", tt.args)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != ".git" {
				t.Fatalf("invalid command unexpectedly created files: %v", entries)
			}
		})
	}
}

func TestRunDoesNotRetainCommandState(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	var stdout bytes.Buffer
	if err := cli.Run([]string{"new", "--help"}, root, time.Now(), &stdout); err != nil {
		t.Fatal(err)
	}

	assertNew(t, root, root)

	stdout.Reset()
	if err := cli.Run(nil, root, time.Now(), &stdout); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); !strings.Contains(got, "Available Commands:") {
		t.Fatalf("no-argument invocation did not reset to root help: %q", got)
	}
}

func assertEmptyDirectory(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("command unexpectedly created files in %s: %v", path, entries)
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
