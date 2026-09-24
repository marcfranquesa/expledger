//go:build darwin || linux

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

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/marcfranquesa/expledger/internal/cli"
	"github.com/marcfranquesa/expledger/internal/experiment"
)

const runnerID = "20260924-runner"

func TestRunUsesCurrentFilesInActualExperimentDirectory(t *testing.T) {
	root, initial := runnerFixture(t)
	git(t, root, "checkout", "--quiet", "-b", "experiment-runner")
	runnerWrite(t, runnerPath(root, "run.sh"), `#!/bin/sh
set -eu
test "$#" -eq 0
test -x ./run.sh
IFS= read -r input
printf '%s\n' "$PWD"
cat project-input.txt
printf 'input: %s\n' "$input"
printf 'workload stderr\n' >&2
printf 'local result\n' > result.txt
`, 0o755)
	if err := os.Symlink("../../source.txt", runnerPath(root, "project-input.txt")); err != nil {
		t.Fatal(err)
	}
	current := runnerCommit(t, root)
	runnerWrite(t, filepath.Join(root, "source.txt"), "current uncommitted code\n", 0o644)
	worktrees := git(t, root, "worktree", "list", "--porcelain")
	for _, previous := range []string{"", initial, strings.Repeat("f", 40)} {
		if previous != "" {
			err := catalog.RecordRun(root, runnerID, experiment.RunReceipt{ProjectCommit: previous, StartedAt: metadataTestTime()})
			if err != nil {
				t.Fatal(err)
			}
		}
		var stdout, stderr bytes.Buffer
		started := time.Now().UTC()
		err := cli.Run(context.Background(), []string{"run", runnerID}, runnerPath(root, ""), metadataTestTime(), cli.Streams{
			In: strings.NewReader("fixed input\n"), Out: &stdout, Err: &stderr,
		})
		if err != nil {
			t.Fatalf("run with previous commit %q: %v", previous, err)
		}
		want := runnerPath(root, "") + "\ncurrent uncommitted code\ninput: fixed input\n"
		if stdout.String() != want || !strings.Contains(stderr.String(), "workload stderr\n") {
			t.Fatalf("unexpected workload streams: stdout=%q, stderr=%q", &stdout, &stderr)
		}
		runnerAssertReceipt(t, root, current, true, started)
	}
	if got := runnerRead(t, runnerPath(root, "result.txt")); got != "local result\n" {
		t.Fatalf("result was not written to the actual experiment: %q", got)
	}
	if got := runnerRead(t, filepath.Join(root, "source.txt")); got != "current uncommitted code\n" {
		t.Fatalf("current source changed: %q", got)
	}
	if got := git(t, root, "rev-parse", "HEAD"); got != current {
		t.Fatalf("run moved HEAD to %s, want %s", got, current)
	}
	if got := git(t, root, "worktree", "list", "--porcelain"); got != worktrees {
		t.Fatalf("run changed worktree registrations:\nbefore: %s\nafter: %s", worktrees, got)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "expledger")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("run created runner state in Git's directory: %v", err)
	}
}

func TestRunRecordsProjectDirtinessOutsideExperiments(t *testing.T) {
	for _, tt := range []struct {
		name   string
		dirty  bool
		change func(*testing.T, string)
	}{
		{"experiment-only edits", false, func(t *testing.T, root string) {
			runnerWrite(t, runnerPath(root, "README.md"), "current notes\n", 0o644)
		}},
		{"experiment-only commit", false, func(t *testing.T, root string) { runnerCommit(t, root) }},
		{"unstaged project edit", true, func(t *testing.T, root string) {
			runnerWrite(t, filepath.Join(root, "source.txt"), "changed\n", 0o644)
		}},
		{"staged project edit", true, func(t *testing.T, root string) {
			runnerWrite(t, filepath.Join(root, "source.txt"), "changed\n", 0o644)
			git(t, root, "add", "source.txt")
		}},
		{"untracked project file", true, func(t *testing.T, root string) {
			runnerWrite(t, filepath.Join(root, "untracked.txt"), "new code\n", 0o644)
		}},
		{"ignored project file", false, func(t *testing.T, root string) {
			runnerWrite(t, filepath.Join(root, ".git", "info", "exclude"), "ignored.txt\n", 0o644)
			runnerWrite(t, filepath.Join(root, "ignored.txt"), "local cache\n", 0o644)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := runnerFixture(t)
			tt.change(t, root)
			commit := git(t, root, "rev-parse", "HEAD")
			started := time.Now().UTC()
			if err := cli.Run(context.Background(), []string{"run", runnerID}, root, metadataTestTime(), cli.Streams{}); err != nil {
				t.Fatal(err)
			}
			runnerAssertReceipt(t, root, commit, tt.dirty, started)
		})
	}
}

func TestRunUsesManuallySelectedCheckout(t *testing.T) {
	root, initial := runnerFixture(t)
	runnerWrite(t, filepath.Join(root, "source.txt"), "newer committed code\n", 0o644)
	git(t, root, "add", "source.txt")
	git(t, root, "commit", "--quiet", "-m", "newer project code")
	newer := git(t, root, "rev-parse", "HEAD")
	if err := catalog.RecordRun(root, runnerID, experiment.RunReceipt{ProjectCommit: newer, StartedAt: metadataTestTime()}); err != nil {
		t.Fatal(err)
	}
	git(t, root, "checkout", "--quiet", "--detach", initial)
	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\ncat ../../source.txt\n", 0o755)
	var stdout bytes.Buffer
	started := time.Now().UTC()
	if err := cli.Run(context.Background(), []string{"run", runnerID}, root, metadataTestTime(), cli.Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "committed code\n" || git(t, root, "rev-parse", "HEAD") != initial {
		t.Fatalf("run did not preserve the manually selected checkout: stdout=%q", &stdout)
	}
	runnerAssertReceipt(t, root, initial, false, started)
}

func TestRunPreservesCallerEnvironment(t *testing.T) {
	root, commit := runnerFixture(t)
	other, _ := runnerFixture(t)
	runnerWrite(t, filepath.Join(other, "source.txt"), "other project's code\n", 0o644)
	runnerCommit(t, other)
	gitDir := filepath.Join(other, ".git")
	runnerWrite(t, runnerPath(root, "run.sh"), `#!/bin/sh
set -eu
test "$GIT_LITERAL_PATHSPECS" = 1
cat ../../source.txt
printf '%s\n' "$GIT_DIR" "$EXPERIMENT_SETTING"
`, 0o755)
	t.Setenv("GIT_DIR", gitDir)
	t.Setenv("GIT_WORK_TREE", other)
	t.Setenv("GIT_LITERAL_PATHSPECS", "1")
	t.Setenv("EXPERIMENT_SETTING", "caller setting")
	var stdout bytes.Buffer
	started := time.Now().UTC()
	if err := cli.Run(context.Background(), []string{"run", runnerID}, root, metadataTestTime(), cli.Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "committed code\n"+gitDir+"\ncaller setting\n" {
		t.Fatalf("caller environment changed: %q", &stdout)
	}
	runnerAssertReceipt(t, root, commit, false, started)
}

func TestRunPrelaunchFailuresPreserveReceipt(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*testing.T, string)
	}{
		{"missing runner", func(t *testing.T, root string) {
			if err := os.Remove(runnerPath(root, "run.sh")); err != nil {
				t.Fatal(err)
			}
		}},
		{"nonexecutable runner", func(t *testing.T, root string) {
			if err := os.Chmod(runnerPath(root, "run.sh"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing interpreter", func(t *testing.T, root string) {
			runnerWrite(t, runnerPath(root, "run.sh"), "#!/nonexistent-expledger-test-interpreter\n", 0o755)
		}},
		{"no shebang", func(t *testing.T, root string) {
			runnerWrite(t, runnerPath(root, "run.sh"), "printf 'unexpected shell fallback\\n'\n", 0o755)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := runnerFixture(t)
			if err := cli.Run(context.Background(), []string{"run", runnerID}, root, metadataTestTime(), cli.Streams{}); err != nil {
				t.Fatal(err)
			}
			before := runnerRead(t, runnerPath(root, "expledger.yaml"))
			tt.change(t, root)
			var stdout bytes.Buffer
			err := cli.Run(context.Background(), []string{"run", runnerID}, root, metadataTestTime(), cli.Streams{Out: &stdout})
			if err == nil || cli.ExitCode(err) != 1 || stdout.Len() != 0 {
				t.Fatalf("prelaunch failure = %v, exit=%d, stdout=%q", err, cli.ExitCode(err), &stdout)
			}
			if got := runnerRead(t, runnerPath(root, "expledger.yaml")); got != before {
				t.Fatal("failed process launch replaced the last-run receipt")
			}
		})
	}
}

func TestRunBinaryPreservesExitStatusAndCancellation(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "expledger")
	build := exec.Command("go", "build", "-o", binary, "./cmd/expledger")
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build expledger: %v\n%s", err, output)
	}
	root, commit := runnerFixture(t)
	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\nprintf 'workload output\\n'\nexit 42\n", 0o755)
	started := time.Now().UTC()
	command := exec.Command(binary, "run", runnerID)
	command.Dir = root
	var stdout bytes.Buffer
	command.Stdout = &stdout
	err := command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 42 || stdout.String() != "workload output\n" {
		t.Fatalf("binary result: err=%v, stdout=%q; want workload output and exit 42", err, &stdout)
	}
	runnerAssertReceipt(t, root, commit, false, started)

	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\n(sleep 2; printf survived > leaked.txt) &\nprintf ready > ready.txt\nwait\n", 0o755)
	command = exec.Command(binary, "run", runnerID)
	command.Dir = root
	started = time.Now().UTC()
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		finished <- command.Wait()
	}()
	t.Cleanup(func() {
		_ = command.Process.Kill()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("runner process did not finish during cleanup")
		}
	})
	deadline := time.After(5 * time.Second)
	for {
		if _, err := os.Stat(runnerPath(root, "ready.txt")); err == nil {
			break
		}
		select {
		case err := <-finished:
			t.Fatalf("runner exited before readiness: %v", err)
		case <-deadline:
			t.Fatal("runner did not start")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if !errors.As(err, &exit) || exit.ExitCode() != 130 {
			t.Fatalf("interrupted binary error = %v, want exit 130", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("interrupted workload did not stop promptly")
	}
	runnerAssertReceipt(t, root, commit, false, started)
	if got := runnerRead(t, runnerPath(root, "ready.txt")); got != "ready" {
		t.Fatalf("cancellation changed experiment files: %q", got)
	}
	time.Sleep(2200 * time.Millisecond)
	if _, err := os.Stat(runnerPath(root, "leaked.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an ordinary child survived cancellation: %v", err)
	}
}

func runnerFixture(t *testing.T) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project with spaces")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "--quiet")
	git(t, root, "config", "user.name", "ExpLedger Test")
	git(t, root, "config", "user.email", "test@example.invalid")
	git(t, root, "config", "commit.gpgsign", "false")
	runnerWrite(t, filepath.Join(root, "source.txt"), "committed code\n", 0o644)
	commit := runnerCommit(t, root)
	if _, err := catalog.Create(root, "runner", metadataTestTime(), catalog.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\nexit 0\n", 0o755)
	return root, commit
}

func runnerCommit(t *testing.T, root string) string {
	t.Helper()
	git(t, root, "add", "--all")
	git(t, root, "commit", "--quiet", "-m", "test fixture")
	return git(t, root, "rev-parse", "HEAD")
}

func runnerPath(root, name string) string {
	return filepath.Join(root, "experiments", runnerID, name)
}

func runnerWrite(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func runnerRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runnerAssertReceipt(t *testing.T, root, commit string, dirty bool, started time.Time) {
	t.Helper()
	record, err := catalog.Read(root, runnerID)
	if err != nil {
		t.Fatal(err)
	}
	if record.LastRun == nil || record.LastRun.ProjectCommit != commit || record.LastRun.ProjectDirty != dirty || record.LastRun.StartedAt.Before(started) || record.LastRun.StartedAt.After(time.Now()) {
		t.Fatalf("last_run = %+v, want commit %s, dirty=%v, and a start between %s and now", record.LastRun, commit, dirty, started)
	}
}
