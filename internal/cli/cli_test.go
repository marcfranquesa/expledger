package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
)

func TestNewFromNestedDirectory(t *testing.T) {
	root := t.TempDir()
	initProject(t, root)
	cwd := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	assertNew(t, cwd, root)
}

func TestCommandsWithoutProject(t *testing.T) {
	for _, args := range [][]string{{"new", "idea"}, {"run", "idea"}, {"list"}, {"validate", "idea"}, {"build"}, {"serve"}} {
		root := t.TempDir()
		err := cli.Run(context.Background(), args, root, time.Now(), cli.Streams{})
		if err == nil || !strings.Contains(err.Error(), "expledger init") {
			t.Fatalf("%v: %v", args, err)
		}
		assertEmptyDirectory(t, root)
	}
}

func TestNewWithoutGit(t *testing.T) {
	root := t.TempDir()
	initProject(t, root)
	t.Setenv("PATH", t.TempDir())
	assertNew(t, root, root)
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
		{name: "init", helpText: []string{"--remote-url"}},
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
	initProject(t, root)
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

func TestUsageErrorsPrecedeProjectLookup(t *testing.T) {
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
			if strings.Contains(err.Error(), "expledger init") {
				t.Fatalf("Project lookup ran before argument validation: %v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("invalid command printed output: %q", stdout.String())
			}
			assertEmptyDirectory(t, root)
		})
	}
}

func TestRunDoesNotRetainProjectRoot(t *testing.T) {
	for range 2 {
		root := t.TempDir()
		initProject(t, root)
		assertNew(t, root, root)
	}
}

func TestRunCanceledBeforeProjectLookup(t *testing.T) {
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
	for _, filename := range []string{"experiment.yaml", "README.md"} {
		if _, err := os.Stat(filepath.Join(want, filename)); err != nil {
			t.Fatalf("experiment %s: %v", filename, err)
		}
	}
}

func initProject(t *testing.T, root string) {
	t.Helper()
	if err := cli.Run(context.Background(), []string{"init"}, root, time.Time{}, cli.Streams{}); err != nil {
		t.Fatal(err)
	}
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
	metadata := filepath.Join(dir, "experiment.yaml")
	if err := os.WriteFile(metadata, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return metadata
}

func TestVersionWithoutProjectOrGit(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"--version"}, root, time.Now(), cli.Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "expledger version devel\n" {
		t.Fatalf("version output: %q", got)
	}
	assertEmptyDirectory(t, root)
}
