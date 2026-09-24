package cli_test

import (
	"bytes"
	"context"
	"errors"
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

func TestCommandsOutsideGit(t *testing.T) {
	for _, command := range []string{"new", "run"} {
		t.Run(command, func(t *testing.T) {
			root := t.TempDir()
			var stdout bytes.Buffer
			err := cli.Run(context.Background(), []string{command, "my-idea"}, root, time.Now(), cli.Streams{Out: &stdout})
			if err == nil {
				t.Fatal("expected error outside a Git worktree")
			}
			for _, want := range []string{root, "Git repository", "existing", "git init"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("outside-repository error %q does not contain %q", err, want)
				}
			}
			if strings.Contains(err.Error(), "exit status 128") {
				t.Errorf("outside-repository error exposes Git's exit status: %v", err)
			}
			if stdout.Len() != 0 {
				t.Errorf("failed command wrote to stdout: %q", stdout.String())
			}
			assertEmptyDirectory(t, root)
		})
	}
}

func TestNewWithoutGit(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	var stdout bytes.Buffer
	err := cli.Run(context.Background(), []string{"new", "my-idea"}, root, time.Now(), cli.Streams{Out: &stdout})
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("missing-Git error = %v, want wrapped exec.ErrNotFound", err)
	}
	if strings.Contains(err.Error(), "git init") {
		t.Errorf("missing-Git error incorrectly suggests initializing a repository: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("failed command wrote to stdout: %q", stdout.String())
	}
	assertEmptyDirectory(t, root)
}

func TestNewInBareRepository(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--bare", "--quiet")
	var stdout bytes.Buffer
	err := cli.Run(context.Background(), []string{"new", "my-idea"}, root, time.Now(), cli.Streams{Out: &stdout})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("bare-repository error = %v, want wrapped exec.ExitError", err)
	}
	if !strings.Contains(err.Error(), "work tree") {
		t.Errorf("bare-repository error omits Git's working-tree diagnostic: %v", err)
	}
	if strings.Contains(err.Error(), "git init") || strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("bare-repository error incorrectly describes a missing repository: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("failed command wrote to stdout: %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, "experiments")); !os.IsNotExist(err) {
		t.Fatalf("failed command should not create experiments directory; stat error: %v", err)
	}
}

func TestNewWithInvalidGitDirectory(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	missing := filepath.Join(root, "missing.git")
	t.Setenv("GIT_DIR", missing)
	var stdout bytes.Buffer
	err := cli.Run(context.Background(), []string{"new", "my-idea"}, root, time.Now(), cli.Streams{Out: &stdout})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("invalid-GIT_DIR error = %v, want wrapped exec.ExitError", err)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("invalid-GIT_DIR error omits the invalid path: %v", err)
	}
	if strings.Contains(err.Error(), "no Git repository found") || strings.Contains(err.Error(), "git init") {
		t.Errorf("invalid-GIT_DIR error incorrectly suggests creating a repository: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("failed command wrote to stdout: %q", stdout.String())
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
			t.Setenv("PATH", t.TempDir())
			var stdout bytes.Buffer
			if err := cli.Run(context.Background(), tt.args, root, time.Now(), cli.Streams{Out: &stdout}); err != nil {
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

func TestCommandHelp(t *testing.T) {
	for _, command := range []struct {
		name, argument string
		helpText       []string
	}{
		{name: "new", argument: "<slug>"},
		{name: "validate", argument: "<id>"},
		{name: "run", argument: "<id>", helpText: []string{"run.sh"}},
		{name: "list"},
		{name: "build", helpText: []string{"--output", "dist", "index.html", "snapshot"}},
		{name: "serve", helpText: []string{"--port", "127.0.0.1", "Ctrl+C"}},
	} {
		for _, args := range [][]string{
			{"help", command.name},
			{command.name, "--help"},
			{command.name, "-h"},
			{command.name, "example", "--help"},
		} {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				root := t.TempDir()
				t.Setenv("PATH", t.TempDir())
				var stdout bytes.Buffer
				if err := cli.Run(context.Background(), args, root, time.Now(), cli.Streams{Out: &stdout}); err != nil {
					t.Fatal(err)
				}
				usage := strings.TrimSpace("expledger " + command.name + " " + command.argument)
				for _, want := range append([]string{"Usage:", usage}, command.helpText...) {
					if !strings.Contains(stdout.String(), want) {
						t.Errorf("help missing %q: %s", want, stdout.String())
					}
				}
				if command.name == "run" && (strings.Contains(stdout.String(), "--at") || strings.Contains(stdout.String(), "EXPLEDGER_")) {
					t.Errorf("run help still advertises revision selection or managed output paths: %s", &stdout)
				}
				assertEmptyDirectory(t, root)
			})
		}
	}
}

func TestRunDoesNotRetainCommandState(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"new", "--help"}, root, time.Now(), cli.Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}

	assertNew(t, root, root)

	stdout.Reset()
	if err := cli.Run(context.Background(), nil, root, time.Now(), cli.Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); !strings.Contains(got, "Available Commands:") {
		t.Fatalf("no-argument invocation did not reset to root help: %q", got)
	}
}

func TestUsageErrorsPrecedeGitLookup(t *testing.T) {
	for _, args := range [][]string{
		{"unknown"},
		{"--unknown"},
		{"new", "my-idea", "--unknown"},
		{"new"},
		{"new", "my-idea", "another-idea"},
		{"new", "my-idea", "--title", ""},
		{"new", "my-idea", "--title", " \t "},
		{"validate"},
		{"validate", "selected", "another-id"},
		{"validate", "selected", "--unknown"},
		{"list", "extra"},
		{"list", "--unknown"},
		{"build", "extra"},
		{"build", "--output="},
		{"build", "--output"},
		{"build", "--unknown"},
		{"serve", "extra"},
		{"serve", "--port=-1"},
		{"serve", "--port=65536"},
		{"run"},
		{"run", "selected", "another-id"},
		{"run", "selected", "--", "seed=7"},
		{"run", "selected", "--seed", "7"},
		{"run", "selected", "--at", "HEAD"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("PATH", t.TempDir())
			var stdout bytes.Buffer
			err := cli.Run(context.Background(), args, root, time.Now(), cli.Streams{Out: &stdout})
			if err == nil {
				t.Fatalf("expected a usage error for %q", args)
			}
			if errors.Is(err, exec.ErrNotFound) || strings.Contains(err.Error(), "find Git") {
				t.Fatalf("Git lookup ran before argument validation: %v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("invalid command printed output: %q", stdout.String())
			}
			assertEmptyDirectory(t, root)
		})
	}
}

func TestRunDoesNotRetainRepositoryRoot(t *testing.T) {
	for range 2 {
		root := t.TempDir()
		git(t, root, "init", "--quiet")
		assertNew(t, root, root)
	}
}

func TestRunCanceledBeforeGitLookup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout bytes.Buffer
	if err := cli.Run(ctx, []string{"new", "my-idea"}, root, time.Now(), cli.Streams{Out: &stdout}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Run error = %v, want context.Canceled", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("canceled command printed output: %q", stdout.String())
	}
	assertEmptyDirectory(t, root)
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
	if err := cli.Run(context.Background(), []string{"new", "my-idea"}, cwd, now, cli.Streams{Out: &stdout}); err != nil {
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
	for _, filename := range []string{"expledger.yaml", "README.md"} {
		if _, err := os.Stat(filepath.Join(want, filename)); err != nil {
			t.Fatalf("experiment %s: %v", filename, err)
		}
	}
}

func git(t *testing.T, cwd string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeREADME(t *testing.T, root, id string, data []byte) string {
	t.Helper()
	dir := filepath.Join(root, "experiments", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return readme
}

func writeMetadata(t *testing.T, root, id string, data []byte) string {
	t.Helper()
	dir := filepath.Join(root, "experiments", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(dir, "expledger.yaml")
	if err := os.WriteFile(metadata, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return metadata
}
