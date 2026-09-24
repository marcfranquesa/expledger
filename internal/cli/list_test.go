package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
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

func TestListPreservesIDs(t *testing.T) {
	for _, tc := range []struct {
		id, display string
	}{
		{"20260924-record", "20260924-record"},
		{"café-α", "café-α"},
		{"trial one", `"trial one"`},
		{"trial  one", `"trial  one"`},
		{" trial", `" trial"`},
		{"trial ", `"trial "`},
		{"trial\tone", `"trial\tone"`},
		{"trial\none", `"trial\none"`},
		{`trial"one`, `"trial\"one"`},
		{`"trial one"`, `"\"trial one\""`},
		{"trial\x1bone", `"trial\x1bone"`},
	} {
		t.Run(tc.display, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			writeListRecord(t, root, tc.id, experiment.Record{Schema: experiment.Schema, ID: tc.id, Title: "Title", CreatedAt: metadataTestTime()})
			var stdout bytes.Buffer
			if err := cli.Run(context.Background(), []string{"list"}, root, time.Time{}, &stdout); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
			if len(lines) != 2 || !strings.HasSuffix(lines[1], "  Title") {
				t.Fatalf("list output = %q, want a header and one title row", stdout.String())
			}
			got := strings.TrimRight(strings.TrimSuffix(lines[1], "Title"), " ")
			if got != tc.display {
				t.Errorf("displayed ID = %q, want %q", got, tc.display)
			}
			if strings.HasPrefix(got, `"`) {
				decoded, err := strconv.Unquote(got)
				if err != nil || decoded != tc.id {
					t.Errorf("unquoted ID = %q, err = %v; want %q", decoded, err, tc.id)
				}
			}
		})
	}
}

func TestListFormatsTitles(t *testing.T) {
	for _, tc := range []struct {
		title, display string
	}{
		{"Messy\t title\ncontinued", "Messy title continued"},
		{`Café α "quoted" C:\scratch`, `Café α "quoted" C:\scratch`},
		{"Terminal\x1b[2J\a\b\x00\x7f", `Terminal\x1b[2J\a\b\x00\x7f`},
		{"Unicode\u009b\u202e", `Unicode\u009b\u202e`},
	} {
		t.Run(tc.display, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			writeListRecord(t, root, "20260924-record", experiment.Record{Schema: experiment.Schema, ID: "20260924-record", Title: tc.title, CreatedAt: metadataTestTime()})
			var stdout bytes.Buffer
			if err := cli.Run(context.Background(), []string{"list"}, root, time.Time{}, &stdout); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
			if len(lines) != 2 || strings.TrimSpace(strings.TrimPrefix(lines[1], "20260924-record")) != tc.display {
				t.Fatalf("list output = %q, want title %q on one line", stdout.String(), tc.display)
			}
		})
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

func TestListInvalidMetadata(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	writeListRecord(t, root, "20260924-valid", experiment.Record{Schema: experiment.Schema, ID: "20260924-valid", Title: "Valid", CreatedAt: metadataTestTime()})
	writeMetadata(t, root, "z-invalid", []byte("schema: expledger/v1\nid: [invalid]\ntitle: Invalid\ncreated_at: 2026-09-24T12:00:00Z\n"))
	var stdout bytes.Buffer
	err := cli.Run(context.Background(), []string{"list"}, root, time.Time{}, &stdout)
	if err == nil || !strings.Contains(err.Error(), filepath.Join("z-invalid", "expledger.yaml")) {
		t.Fatalf("expected invalid metadata path in error, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("failed list printed partial output: %q", stdout.String())
	}
}

func TestListCoexistsWithOtherTools(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	const id = "20260924-shared"
	writeListRecord(t, root, id, experiment.Record{Schema: experiment.Schema, ID: id, Title: "ExpLedger title", CreatedAt: metadataTestTime()})
	const readmeContent = "---\nlabexp:\n  run_id: abc123\n  title: LabExp title\n---\n# Shared experiment notes\n"
	readme := writeREADME(t, root, id, []byte(readmeContent))
	const otherContent = "---\nid: 20260924-other\ntitle: Other tool's experiment\ncreated_at: 2026-09-24T13:00:00Z\n---\n# Unrelated notes\n"
	otherReadme := writeREADME(t, root, "20260924-other", []byte(otherContent))
	if err := os.Mkdir(filepath.Join(root, "experiments", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"list"}, root, time.Time{}, &stdout); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 || strings.Join(strings.Fields(lines[1]), " ") != id+" ExpLedger title" {
		t.Fatalf("list discovered other tools' records or metadata: %q", stdout.String())
	}
	for path, want := range map[string]string{readme: readmeContent, otherReadme: otherContent} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("list changed README %s: data=%q, err=%v", path, data, err)
		}
	}
}

func TestListRejectsMismatchedID(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	writeListRecord(t, root, "20260922-valid", experiment.Record{Schema: experiment.Schema, ID: "20260922-valid", Title: "Valid", CreatedAt: metadataTestTime()})
	const folder = "20260924-renamed"
	const id = "20260924-original"
	writeListRecord(t, root, folder, experiment.Record{Schema: experiment.Schema, ID: id, Title: "Renamed experiment", CreatedAt: metadataTestTime()})
	var stdout bytes.Buffer
	err := cli.Run(context.Background(), []string{"list"}, root, time.Time{}, &stdout)
	if err == nil {
		t.Fatal("expected error for experiment ID differing from its folder")
	}
	for _, want := range []string{filepath.Join("experiments", folder, "expledger.yaml"), folder, id} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want %q", err, want)
		}
	}
	if stdout.Len() != 0 {
		t.Fatalf("failed list printed partial output: %q", stdout.String())
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
	writeMetadata(t, root, name, data)
}
