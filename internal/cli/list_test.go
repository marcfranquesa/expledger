package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
	"github.com/marcfranquesa/expledger/internal/experiment"
)

func TestListFromNestedDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "project"))); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "--quiet")
	cwd := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"list"}, cwd, time.Time{}, &stdout); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	want := []string{
		"ID TITLE",
		"20260924-long-title Unicode & HTML: comparing café embeddings with α < β across a deliberately long experiment title",
		"20260924-variant Lower learning rate",
		"20260924-baseline Baseline model",
	}
	if len(lines) != len(want) {
		t.Fatalf("list output = %q, want %d lines", stdout.String(), len(want))
	}
	for i, line := range lines {
		if got := strings.Join(strings.Fields(line), " "); got != want[i] {
			t.Errorf("line %d = %q, want %q", i+1, got, want[i])
		}
	}
}

func TestListNormalizesWhitespace(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	writeListRecord(t, root, "20260924-record", experiment.Record{ID: "20260924-record", Title: "Messy\t title\ncontinued", CreatedAt: metadataTestTime()})
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"list"}, root, time.Time{}, &stdout); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 || strings.Join(strings.Fields(lines[1]), " ") != "20260924-record Messy title continued" {
		t.Fatalf("list did not keep the record on one line: %q", stdout.String())
	}
}

func TestListEmpty(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "missing directory"
		if existing {
			name = "empty directory"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			dir := filepath.Join(root, "experiments")
			if existing {
				if err := os.Mkdir(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			var stdout bytes.Buffer
			if err := cli.Run(context.Background(), []string{"list"}, root, time.Time{}, &stdout); err != nil {
				t.Fatal(err)
			}
			if got, want := stdout.String(), "No experiments found.\n"; got != want {
				t.Fatalf("list output = %q, want %q", got, want)
			}
			if existing {
				assertEmptyDirectory(t, dir)
			} else if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatalf("list created experiments directory; stat error: %v", err)
			}
		})
	}
}

func TestListInvalidReadme(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		name := "missing README"
		if malformed {
			name = "malformed README"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			writeListRecord(t, root, "20260924-valid", experiment.Record{ID: "20260924-valid", Title: "Valid", CreatedAt: metadataTestTime()})
			dir := filepath.Join(root, "experiments", "z-invalid")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if malformed {
				if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("---\nid: [invalid]\ntitle: Invalid\ncreated_at: 2026-09-24T12:00:00Z\n---\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var stdout bytes.Buffer
			err := cli.Run(context.Background(), []string{"list"}, root, time.Time{}, &stdout)
			if err == nil || !strings.Contains(err.Error(), filepath.Join("z-invalid", "README.md")) {
				t.Fatalf("expected invalid README path in error, got %v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("failed list printed partial output: %q", stdout.String())
			}
		})
	}
}

func TestListRejectsMismatchedID(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	writeListRecord(t, root, "20260922-valid", experiment.Record{ID: "20260922-valid", Title: "Valid", CreatedAt: metadataTestTime()})
	const folder = "20260924-renamed"
	const id = "20260924-original"
	writeListRecord(t, root, folder, experiment.Record{ID: id, Title: "Renamed experiment", CreatedAt: metadataTestTime()})
	var stdout bytes.Buffer
	err := cli.Run(context.Background(), []string{"list"}, root, time.Time{}, &stdout)
	if err == nil {
		t.Fatal("expected error for experiment ID differing from its folder")
	}
	for _, want := range []string{filepath.Join("experiments", folder, "README.md"), folder, id} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want %q", err, want)
		}
	}
	if stdout.Len() != 0 {
		t.Fatalf("failed list printed partial output: %q", stdout.String())
	}
}

func TestListHelp(t *testing.T) {
	for _, args := range [][]string{{"list", "--help"}, {"list", "-h"}, {"help", "list"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("PATH", t.TempDir())
			var stdout bytes.Buffer
			if err := cli.Run(context.Background(), args, root, time.Time{}, &stdout); err != nil {
				t.Fatal(err)
			}
			if got := stdout.String(); !strings.Contains(got, "Usage:") || !strings.Contains(got, "expledger list") {
				t.Fatalf("list help does not describe usage: %q", got)
			}
			assertEmptyDirectory(t, root)
		})
	}
}

func TestListInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"list", "extra"}, {"list", "--unknown"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("PATH", t.TempDir())
			var stdout bytes.Buffer
			err := cli.Run(context.Background(), args, root, time.Time{}, &stdout)
			if err == nil || strings.Contains(err.Error(), "find Git working tree") {
				t.Fatalf("expected usage error before Git lookup, got %v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("invalid command printed output: %q", stdout.String())
			}
			assertEmptyDirectory(t, root)
		})
	}
}

func TestListOutsideGit(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"list"}, root, time.Time{}, &stdout); err == nil {
		t.Fatal("expected error outside a Git worktree")
	}
	if stdout.Len() != 0 {
		t.Fatalf("failed list printed output: %q", stdout.String())
	}
	assertEmptyDirectory(t, root)
}

func writeListRecord(t *testing.T, root, name string, record experiment.Record) {
	t.Helper()
	data, err := record.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	writeREADME(t, root, name, data)
}
