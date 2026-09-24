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
)

const runnerID = "20260924-runner"

func TestRunReplaysProjectRevisionWithCurrentExperiment(t *testing.T) {
	root, first := runnerFixture(t)
	runnerWrite(t, filepath.Join(root, "source.txt"), "version B\n", 0o644)
	second := runnerCommit(t, root)
	runnerWrite(t, filepath.Join(root, "source.txt"), "uncommitted project code\n", 0o644)
	runnerWrite(t, filepath.Join(root, "keep.txt"), "untracked project file\n", 0o644)
	runnerWrite(t, runnerPath(root, "definition.txt"), "current definition one\n", 0o644)
	runnerWrite(t, runnerPath(root, "run.sh"), `#!/bin/sh
set -eu
cat "$EXPLEDGER_PROJECT_DIR/source.txt"
cat definition.txt
printf '%s\n' "$EXPLEDGER_OUTPUT_DIR"
printf 'saved output\n' > "$EXPLEDGER_OUTPUT_DIR/result.txt"
pwd > "$EXPLEDGER_OUTPUT_DIR/working-directory.txt"
`, 0o755)

	var outputPaths []string
	for index, tt := range []struct {
		args       []string
		commit     string
		code       string
		definition string
	}{
		{[]string{"run", runnerID, "--at", first}, first, "version A", "current definition one"},
		{[]string{"run", runnerID}, first, "version A", "current definition two"},
		{[]string{"run", runnerID, "--at", "HEAD"}, second, "version B", "current definition two"},
	} {
		if index == 1 {
			runnerWrite(t, runnerPath(root, "definition.txt"), "current definition two\n", 0o644)
		}
		started := time.Now().UTC()
		var stdout, stderr bytes.Buffer
		if err := cli.Run(context.Background(), tt.args, root, started, cli.Streams{Out: &stdout, Err: &stderr}); err != nil {
			t.Fatalf("run %d: %v\nstderr: %s", index, err, &stderr)
		}
		lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
		if len(lines) != 3 || lines[0] != tt.code || lines[1] != tt.definition {
			t.Fatalf("run %d executed wrong source/definition or polluted stdout: %q", index, stdout.String())
		}
		output := lines[2]
		outputPaths = append(outputPaths, output)
		runnerAssertOutputPath(t, root, output)
		if got := runnerRead(t, filepath.Join(output, "result.txt")); got != "saved output\n" {
			t.Fatalf("run %d output did not survive cleanup: %q", index, got)
		}
		cwd := strings.TrimSpace(runnerRead(t, filepath.Join(output, "working-directory.txt")))
		if cwd == runnerPath(root, "") || !strings.HasSuffix(cwd, filepath.Join("experiments", runnerID)) {
			t.Fatalf("run %d cwd = %q, want experiment directory in a separate checkout", index, cwd)
		}
		if _, err := os.Stat(cwd); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temporary experiment directory remains after run: %s: %v", cwd, err)
		}
		if !strings.Contains(stderr.String(), tt.commit) || !strings.Contains(stderr.String(), output) {
			t.Fatalf("stderr does not identify revision and retained output: %q", stderr.String())
		}
		runnerAssertReceipt(t, root, tt.commit, started)
	}
	for i, output := range outputPaths {
		for _, previous := range outputPaths[:i] {
			if output == previous {
				t.Fatalf("runs reused output directory %s", output)
			}
		}
		if _, err := os.Stat(filepath.Join(output, "result.txt")); err != nil {
			t.Fatalf("later run removed previous output: %v", err)
		}
	}
	if got := runnerGitOutput(t, root, "rev-parse", "HEAD"); got != second {
		t.Fatalf("primary checkout moved to %s, want %s", got, second)
	}
	if got := runnerRead(t, filepath.Join(root, "source.txt")); got != "uncommitted project code\n" {
		t.Fatalf("primary source edits changed: %q", got)
	}
	if got := runnerRead(t, filepath.Join(root, "keep.txt")); got != "untracked project file\n" {
		t.Fatalf("primary untracked file changed: %q", got)
	}
	runnerAssertWorktreeCount(t, root, 1)
}

func TestRunUsesDirectEntrypointStreamsAndNoArguments(t *testing.T) {
	root, commit := runnerFixture(t)
	runnerWrite(t, runnerPath(root, "run.sh"), `#!/bin/sh
set -eu
test "$#" -eq 0
test -f expledger.yaml
test -x ./run.sh
IFS= read -r input
printf 'input: %s\n' "$input"
printf 'workload stderr\n' >&2
`, 0o755)
	var stdout, stderr bytes.Buffer
	err := cli.Run(context.Background(), []string{"run", runnerID, "--at", commit}, root, metadataTestTime(), cli.Streams{
		In: strings.NewReader("fixed input\n"), Out: &stdout, Err: &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "input: fixed input\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "workload stderr\n") {
		t.Fatalf("child stderr was not forwarded: %q", stderr.String())
	}

	// An executable without a valid shebang must fail to launch, rather than
	// being silently interpreted by a shell chosen by ExpLedger.
	before := runnerRead(t, runnerPath(root, "expledger.yaml"))
	runnerWrite(t, runnerPath(root, "run.sh"), "printf 'unexpected shell fallback\\n'\n", 0o755)
	stdout.Reset()
	err = cli.Run(context.Background(), []string{"run", runnerID}, root, metadataTestTime().Add(time.Hour), cli.Streams{Out: &stdout})
	if err == nil || stdout.Len() != 0 {
		t.Fatalf("entrypoint without shebang unexpectedly ran: err=%v, stdout=%q", err, &stdout)
	}
	if got := runnerRead(t, runnerPath(root, "expledger.yaml")); got != before {
		t.Fatal("failed process launch replaced the last-run receipt")
	}
	runnerAssertWorktreeCount(t, root, 1)
}

func TestRunReplacesHistoricalExperimentDirectory(t *testing.T) {
	root, _ := runnerFixture(t)
	runnerWrite(t, runnerPath(root, "stale.txt"), "old experiment file\n", 0o644)
	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\nexit 91\n", 0o755)
	commit := runnerCommit(t, root)
	if err := os.Remove(runnerPath(root, "stale.txt")); err != nil {
		t.Fatal(err)
	}
	runnerWrite(t, runnerPath(root, "helper.sh"), "#!/bin/sh\nprintf 'current helper\\n'\n", 0o755)
	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\nset -eu\ntest ! -e stale.txt\nexec ./helper.sh\n", 0o755)
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"run", runnerID, "--at", commit}, root, metadataTestTime(), cli.Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "current helper\n" {
		t.Fatalf("current untracked executable helper was not copied: %q", stdout.String())
	}
}

func TestRunIgnoresInheritedGitLocationAndRunnerVariables(t *testing.T) {
	root, commit := runnerFixture(t)
	runnerWrite(t, runnerPath(root, "run.sh"), `#!/bin/sh
set -eu
test "$(git rev-parse --show-toplevel)" = "$EXPLEDGER_PROJECT_DIR"
test "$EXPLEDGER_OUTPUT_DIR" != inherited-output
test "$GIT_SSH_COMMAND" = kept-transport
test "$GIT_ASKPASS" = kept-auth
cat "$EXPLEDGER_PROJECT_DIR/source.txt"
`, 0o755)
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "missing-git-directory"))
	t.Setenv("GIT_WORK_TREE", t.TempDir())
	t.Setenv("GIT_SSH_COMMAND", "kept-transport")
	t.Setenv("GIT_ASKPASS", "kept-auth")
	t.Setenv("EXPLEDGER_PROJECT_DIR", "inherited-project")
	t.Setenv("EXPLEDGER_OUTPUT_DIR", "inherited-output")
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"run", runnerID, "--at", commit}, root, metadataTestTime(), cli.Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "version A\n" {
		t.Fatalf("inherited environment redirected the run: %q", stdout.String())
	}
}

func TestRunFailedWorkloadRecordsRevisionAndReturnsExitCode(t *testing.T) {
	root, first := runnerFixture(t)
	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\nexit 0\n", 0o755)
	if err := cli.Run(context.Background(), []string{"run", runnerID, "--at", first}, root, metadataTestTime(), cli.Streams{}); err != nil {
		t.Fatal(err)
	}
	runnerWrite(t, filepath.Join(root, "source.txt"), "version B\n", 0o644)
	second := runnerCommit(t, root)
	runnerWrite(t, runnerPath(root, "run.sh"), `#!/bin/sh
printf '%s\n' "$EXPLEDGER_OUTPUT_DIR"
printf 'failure details\n' > "$EXPLEDGER_OUTPUT_DIR/failure.txt"
exit 42
`, 0o755)
	var stdout bytes.Buffer
	started := time.Now().UTC()
	err := cli.Run(context.Background(), []string{"run", runnerID, "--at", second}, root, started, cli.Streams{Out: &stdout})
	if err == nil || cli.ExitCode(err) != 42 {
		t.Fatalf("workload exit = %v (%d), want 42", err, cli.ExitCode(err))
	}
	runnerAssertReceipt(t, root, second, started)
	output := strings.TrimSpace(stdout.String())
	if got := runnerRead(t, filepath.Join(output, "failure.txt")); got != "failure details\n" {
		t.Fatalf("failed workload output was not retained: %q", got)
	}
	runnerAssertWorktreeCount(t, root, 1)
}

func TestRunRejectsPreparationFailuresWithoutChangingReceipt(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*testing.T, string)
		args   []string
	}{
		{name: "missing runner", change: func(t *testing.T, root string) {
			if err := os.Remove(runnerPath(root, "run.sh")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "nonexecutable runner", change: func(t *testing.T, root string) {
			if err := os.Chmod(runnerPath(root, "run.sh"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "missing interpreter", change: func(t *testing.T, root string) {
			runnerWrite(t, runnerPath(root, "run.sh"), "#!/nonexistent-expledger-test-interpreter\n", 0o755)
		}},
		{name: "symlink in experiment", change: func(t *testing.T, root string) {
			if err := os.Symlink("README.md", runnerPath(root, "linked-notes")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "missing source revision", args: []string{"run", runnerID, "--at", "does-not-exist"}},
		{name: "explicit empty revision", args: []string{"run", runnerID, "--at="}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, commit := runnerFixture(t)
			runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\nexit 0\n", 0o755)
			if err := cli.Run(context.Background(), []string{"run", runnerID, "--at", commit}, root, metadataTestTime(), cli.Streams{}); err != nil {
				t.Fatal(err)
			}
			before := runnerRead(t, runnerPath(root, "expledger.yaml"))
			if tt.change != nil {
				tt.change(t, root)
			}
			args := tt.args
			if args == nil {
				args = []string{"run", runnerID}
			}
			var stdout bytes.Buffer
			err := cli.Run(context.Background(), args, root, metadataTestTime().Add(time.Hour), cli.Streams{Out: &stdout})
			if err == nil || cli.ExitCode(err) != 1 {
				t.Fatalf("preparation error = %v (%d), want orchestration failure", err, cli.ExitCode(err))
			}
			if stdout.Len() != 0 {
				t.Fatalf("failed preparation wrote to stdout: %q", stdout.String())
			}
			if got := runnerRead(t, runnerPath(root, "expledger.yaml")); got != before {
				t.Fatal("failed preparation changed the previous run receipt")
			}
			runnerAssertWorktreeCount(t, root, 1)
		})
	}
}

func TestRunRequiresExplicitFirstRevision(t *testing.T) {
	root, _ := runnerFixture(t)
	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\nprintf 'unexpected execution\\n'\n", 0o755)
	before := runnerRead(t, runnerPath(root, "expledger.yaml"))
	var stdout bytes.Buffer
	err := cli.Run(context.Background(), []string{"run", runnerID}, root, metadataTestTime(), cli.Streams{Out: &stdout})
	if err == nil || !strings.Contains(err.Error(), "--at") {
		t.Fatalf("missing first revision error = %v, want guidance to --at", err)
	}
	if stdout.Len() != 0 || runnerRead(t, runnerPath(root, "expledger.yaml")) != before {
		t.Fatal("missing first revision executed or changed the experiment")
	}
	runnerAssertWorktreeCount(t, root, 1)
}

func TestRunBinaryPropagatesWorkloadExitStatus(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "expledger")
	build := exec.Command("go", "build", "-o", binary, "./cmd/expledger")
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build expledger: %v\n%s", err, output)
	}
	root, commit := runnerFixture(t)
	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\nprintf 'workload output\\n'\nexit 42\n", 0o755)
	for _, tt := range []struct {
		name string
		args []string
		code int
		out  string
	}{
		{"workload exit", []string{"run", runnerID, "--at", commit}, 42, "workload output\n"},
		{"usage exit", []string{"run", runnerID, "--", "seed=7"}, 1, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binary, tt.args...)
			cmd.Dir = root
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != tt.code {
				t.Fatalf("binary error = %v, want exit %d; stderr: %s", err, tt.code, &stderr)
			}
			if stdout.String() != tt.out {
				t.Fatalf("binary stdout = %q, want %q", stdout.String(), tt.out)
			}
		})
	}
	t.Run("interrupt", func(t *testing.T) {
		runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\nprintf ready > \"$EXPLEDGER_OUTPUT_DIR/ready.txt\"\nexec sleep 30\n", 0o755)
		cmd := exec.Command(binary, "run", runnerID)
		cmd.Dir = root
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill() })
		finished := make(chan error, 1)
		go func() { finished <- cmd.Wait() }()
		runnerWaitForOutput(t, root, "ready.txt", finished)
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-finished:
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 130 {
				t.Fatalf("interrupt exit=%v; stderr: %s", err, &stderr)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("binary did not stop on SIGINT")
		}
		runnerAssertWorktreeCount(t, root, 1)
	})
}

func TestRunOutputsSurviveRemovalOfInvokingLinkedWorktree(t *testing.T) {
	root, commit := runnerFixture(t)
	linked := filepath.Join(t.TempDir(), "linked checkout")
	git(t, root, "worktree", "add", "--quiet", "--detach", linked, commit)
	resolved, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	linked = resolved
	if _, err := catalog.Create(linked, "runner", metadataTestTime(), catalog.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	runnerWrite(t, runnerPath(linked, "run.sh"), "#!/bin/sh\nprintf '%s\\n' \"$EXPLEDGER_OUTPUT_DIR\"\nprintf 'retained result\\n' > \"$EXPLEDGER_OUTPUT_DIR/result.txt\"\n", 0o755)
	var stdout bytes.Buffer
	started := time.Now().UTC()
	if err := cli.Run(context.Background(), []string{"run", runnerID, "--at", commit}, runnerPath(linked, ""), metadataTestTime(), cli.Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}
	output := strings.TrimSpace(stdout.String())
	runnerAssertOutputPath(t, linked, output)
	runnerAssertReceipt(t, linked, commit, started)
	primary, err := catalog.Read(root, runnerID)
	if err != nil {
		t.Fatal(err)
	}
	if primary.LastRun != nil {
		t.Fatal("running from linked worktree updated primary checkout metadata")
	}
	runnerAssertWorktreeCount(t, root, 2)
	git(t, root, "worktree", "remove", "--force", "--", linked)
	runnerAssertWorktreeCount(t, root, 1)
	runnerAssertOutputPath(t, root, output)
	if got := runnerRead(t, filepath.Join(output, "result.txt")); got != "retained result\n" {
		t.Fatalf("removing invoking worktree changed experiment output: %q", got)
	}
}

func TestRunCancellationStopsChildrenAndReleasesExperimentLock(t *testing.T) {
	root, commit := runnerFixture(t)
	runnerWrite(t, runnerPath(root, "run.sh"), `#!/bin/sh
set -eu
(sleep 2; printf 'child outlived cancellation\n' > "$EXPLEDGER_OUTPUT_DIR/leaked.txt") &
printf 'ready\n' > "$EXPLEDGER_OUTPUT_DIR/ready.txt"
wait
`, 0o755)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	done := make(chan struct{})
	started := time.Now().UTC()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("runner goroutine did not finish during cleanup")
		}
	})
	go func() {
		defer close(done)
		finished <- cli.Run(ctx, []string{"run", runnerID, "--at", commit}, root, metadataTestTime(), cli.Streams{})
	}()
	output := runnerWaitForOutput(t, root, "ready.txt", finished)
	secondCtx, secondCancel := context.WithTimeout(context.Background(), time.Second)
	defer secondCancel()
	err := cli.Run(secondCtx, []string{"run", runnerID, "--at", commit}, root, metadataTestTime().Add(time.Hour), cli.Streams{})
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("concurrent invocation should immediately reject a held experiment lock: %v", err)
	}
	cancel()
	select {
	case err := <-finished:
		if cli.ExitCode(err) != 130 {
			t.Fatalf("canceled execution exit = %v (%d), want 130", err, cli.ExitCode(err))
		}
		if strings.Contains(err.Error(), "cleanup") || strings.Contains(err.Error(), "retained") {
			t.Fatalf("canceled execution could not clean up its checkout: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not stop the workload promptly")
	}
	runnerAssertReceipt(t, root, commit, started)
	time.Sleep(2200 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(output, "leaked.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a workload child survived cancellation: %v", err)
	}
	runnerAssertWorktreeCount(t, root, 1)
	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\nexit 0\n", 0o755)
	if err := cli.Run(context.Background(), []string{"run", runnerID}, root, metadataTestTime().Add(2*time.Hour), cli.Streams{}); err != nil {
		t.Fatalf("cancellation left the experiment locked: %v", err)
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
	runnerWrite(t, filepath.Join(root, "source.txt"), "version A\n", 0o644)
	commit := runnerCommit(t, root)
	if _, err := catalog.Create(root, "runner", metadataTestTime(), catalog.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	return root, commit
}

func runnerCommit(t *testing.T, root string) string {
	t.Helper()
	git(t, root, "add", "--all")
	git(t, root, "commit", "--quiet", "-m", "test fixture")
	return runnerGitOutput(t, root, "rev-parse", "HEAD")
}

func runnerGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, data)
	}
	return strings.TrimSpace(string(data))
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

func runnerAssertReceipt(t *testing.T, root, commit string, started time.Time) {
	t.Helper()
	record, err := catalog.Read(root, runnerID)
	if err != nil {
		t.Fatal(err)
	}
	if record.LastRun == nil || record.LastRun.ProjectCommit != commit || record.LastRun.StartedAt.Before(started) || record.LastRun.StartedAt.After(time.Now()) {
		t.Fatalf("last_run = %+v, want commit %s and a start between %s and now", record.LastRun, commit, started)
	}
	if _, offset := record.LastRun.StartedAt.Zone(); offset != 0 {
		t.Fatalf("last_run start = %s, want a UTC timestamp", record.LastRun.StartedAt)
	}
}

func runnerAssertOutputPath(t *testing.T, root, output string) {
	t.Helper()
	gitDir := runnerGitOutput(t, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	wantParent := filepath.Join(gitDir, "expledger", "runs", runnerID)
	if !filepath.IsAbs(output) || filepath.Dir(output) != wantParent {
		t.Fatalf("output directory %q is not a child of shared Git directory %q", output, wantParent)
	}
	info, err := os.Stat(output)
	if err != nil || !info.IsDir() {
		t.Fatalf("output directory did not survive cleanup: info=%v err=%v", info, err)
	}
}

func runnerAssertWorktreeCount(t *testing.T, root string, want int) {
	t.Helper()
	listed := runnerGitOutput(t, root, "worktree", "list", "--porcelain")
	if got := strings.Count("\n"+listed, "\nworktree "); got != want {
		t.Fatalf("registered worktrees = %d, want %d:\n%s", got, want, listed)
	}
}

func runnerWaitForOutput(t *testing.T, root, filename string, finished <-chan error) string {
	t.Helper()
	gitDir := runnerGitOutput(t, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	pattern := filepath.Join(gitDir, "expledger", "runs", runnerID, "*", filename)
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) == 1 {
			return filepath.Dir(paths[0])
		}
		select {
		case err := <-finished:
			t.Fatalf("workload finished before readiness: %v", err)
		case <-deadline:
			t.Fatalf("workload did not create %s", pattern)
		case <-ticker.C:
		}
	}
}
