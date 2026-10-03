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

func TestRunUsesCurrentFilesAndPreservesMetadata(t *testing.T) {
	root := runnerFixture(t)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("EXPERIMENT_SETTING", "caller setting")
	runnerWrite(t, runnerPath(root, "run.sh"), `#!/bin/sh
set -eu
test "$#" -eq 0
IFS= read -r input
printf '%s\n' "$PWD" "$EXPERIMENT_SETTING" "$input"
printf 'workload stderr\n' >&2
printf 'local result\n' > result.txt
`, 0755)
	path := runnerPath(root, "experiment.yaml")
	original := runnerRead(t, path)
	for _, history := range []string{"", "last_run: historical custom value\n", "last_run:\n  project_commit: old\n  project_dirty: true\n  started_at: 2020-01-01T00:00:00Z\n"} {
		before := original + history
		runnerWrite(t, path, before, 0444)
		var stdout, stderr bytes.Buffer
		err := cli.Run(context.Background(), []string{"run", runnerID}, runnerPath(root, ""), metadataTestTime(), cli.Streams{In: strings.NewReader("fixed input\n"), Out: &stdout, Err: &stderr})
		if err != nil {
			t.Fatal(err)
		}
		if stdout.String() != runnerPath(root, "")+"\ncaller setting\nfixed input\n" || stderr.String() != "workload stderr\n" {
			t.Fatalf("streams: %q, %q", &stdout, &stderr)
		}
		if runnerRead(t, path) != before {
			t.Fatal("run changed metadata")
		}
		if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if runnerRead(t, runnerPath(root, "result.txt")) != "local result\n" {
		t.Fatal("result not in experiment directory")
	}
}

func TestRunPrelaunchFailuresPreserveMetadata(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*testing.T, string)
	}{
		{"symlinked runner", func(t *testing.T, root string) {
			if err := os.Rename(runnerPath(root, "run.sh"), runnerPath(root, "actual.sh")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("actual.sh", runnerPath(root, "run.sh")); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlinked metadata", func(t *testing.T, root string) {
			if err := os.Rename(runnerPath(root, "experiment.yaml"), runnerPath(root, "actual.yaml")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("actual.yaml", runnerPath(root, "experiment.yaml")); err != nil {
				t.Fatal(err)
			}
		}},
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
			root := runnerFixture(t)
			if err := cli.Run(context.Background(), []string{"run", runnerID}, root, metadataTestTime(), cli.Streams{}); err != nil {
				t.Fatal(err)
			}
			before := runnerRead(t, runnerPath(root, "experiment.yaml"))
			tt.change(t, root)
			var stdout bytes.Buffer
			err := cli.Run(context.Background(), []string{"run", runnerID}, root, metadataTestTime(), cli.Streams{Out: &stdout})
			if err == nil || cli.ExitCode(err) != 1 || stdout.Len() != 0 {
				t.Fatalf("prelaunch failure = %v, exit=%d, stdout=%q", err, cli.ExitCode(err), &stdout)
			}
			if got := runnerRead(t, runnerPath(root, "experiment.yaml")); got != before {
				t.Fatal("failed process launch changed metadata")
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
	root := runnerFixture(t)
	before := runnerRead(t, runnerPath(root, "experiment.yaml"))
	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\nprintf 'workload output\\n'\nexit 42\n", 0o755)
	command := exec.Command(binary, "run", runnerID)
	command.Dir = root
	var stdout bytes.Buffer
	command.Stdout = &stdout
	err := command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 42 || stdout.String() != "workload output\n" {
		t.Fatalf("binary result: err=%v, stdout=%q; want workload output and exit 42", err, &stdout)
	}
	if runnerRead(t, runnerPath(root, "experiment.yaml")) != before {
		t.Fatal("run changed metadata")
	}

	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\n(sleep 2; printf survived > leaked.txt) &\nprintf ready > ready.txt\nwait\n", 0o755)
	command = exec.Command(binary, "run", runnerID)
	command.Dir = root
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
	if runnerRead(t, runnerPath(root, "experiment.yaml")) != before {
		t.Fatal("run changed metadata")
	}
	if got := runnerRead(t, runnerPath(root, "ready.txt")); got != "ready" {
		t.Fatalf("cancellation changed experiment files: %q", got)
	}
	time.Sleep(2200 * time.Millisecond)
	if _, err := os.Stat(runnerPath(root, "leaked.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an ordinary child survived cancellation: %v", err)
	}
}

func runnerFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project with spaces")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	initProject(t, root)
	if _, err := catalog.Create(root, "runner", metadataTestTime(), catalog.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	runnerWrite(t, runnerPath(root, "run.sh"), "#!/bin/sh\nexit 0\n", 0o755)
	return root
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
