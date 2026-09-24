package cli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
	"github.com/marcfranquesa/expledger/internal/experiment"
)

func TestNewMetadataFlags(t *testing.T) {
	const title = "Small MLP: x80"
	parents := []string{"20260920-baseline", "20260921-comparison"}
	for _, tt := range []struct {
		name string
		args []string
	}{
		{
			name: "before slug",
			args: []string{"new", "--title", title, "--based-on", parents[0], "--based-on", parents[1], "my-idea"},
		},
		{
			name: "after slug",
			args: []string{"new", "my-idea", "--title", title, "--based-on", parents[0], "--based-on", parents[1]},
		},
		{
			name: "around slug",
			args: []string{"new", "--based-on", parents[0], "my-idea", "--based-on", parents[1], "--title", title},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			createMetadataParent(t, root, "baseline", 20)
			createMetadataParent(t, root, "comparison", 21)
			var stdout bytes.Buffer
			if err := cli.Run(tt.args, root, metadataTestTime(), &stdout); err != nil {
				t.Fatal(err)
			}
			record := readNewRecord(t, root, "my-idea")
			if record.ID != "20260924-my-idea" {
				t.Fatalf("id = %q; title must not change the experiment id", record.ID)
			}
			if record.Title != title {
				t.Fatalf("title = %q, want %q", record.Title, title)
			}
			if !reflect.DeepEqual(record.BasedOn, parents) {
				t.Fatalf("based_on = %v, want %v", record.BasedOn, parents)
			}
			if !strings.HasPrefix(string(record.Body), "\n# "+title+"\n") {
				t.Fatalf("README heading does not use the title: %q", record.Body)
			}
		})
	}
}

func TestNewRejectsEmptyMetadataFlags(t *testing.T) {
	for _, tt := range []struct {
		name  string
		flags []string
	}{
		{name: "empty title", flags: []string{"--title", ""}},
		{name: "whitespace title", flags: []string{"--title", " \t "}},
		{name: "empty parent", flags: []string{"--based-on", ""}},
		{name: "whitespace parent", flags: []string{"--based-on", " \t "}},
		{name: "empty repeated parent", flags: []string{"--based-on", "20260920-baseline", "--based-on", ""}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			var stdout bytes.Buffer
			args := append([]string{"new", "my-idea"}, tt.flags...)
			if err := cli.Run(args, root, metadataTestTime(), &stdout); err == nil {
				t.Fatalf("expected an error for %q", args)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != ".git" {
				t.Fatalf("invalid metadata unexpectedly created files: %v", entries)
			}
		})
	}
}

func TestNewMetadataFlagsDoNotLeakBetweenRuns(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	createMetadataParent(t, root, "baseline", 20)
	var stdout bytes.Buffer
	if err := cli.Run([]string{"new", "first-idea", "--title", "Custom title", "--based-on", "20260920-baseline"}, root, metadataTestTime(), &stdout); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := cli.Run([]string{"new", "next-idea"}, root, metadataTestTime(), &stdout); err != nil {
		t.Fatal(err)
	}
	record := readNewRecord(t, root, "next-idea")
	if record.Title != "Next idea" {
		t.Fatalf("default title = %q, want %q", record.Title, "Next idea")
	}
	if len(record.BasedOn) != 0 {
		t.Fatalf("default based_on contains previous flags: %v", record.BasedOn)
	}
	if !strings.HasPrefix(string(record.Body), "\n# Next idea\n") {
		t.Fatalf("default README heading = %q", record.Body)
	}
}

func TestNewRejectsMissingParent(t *testing.T) {
	for _, withExistingParent := range []bool{false, true} {
		name := "only parent missing"
		if withExistingParent {
			name = "second parent missing"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			const parent = "20260920-missing"
			args := []string{"new", "my-idea"}
			if withExistingParent {
				createMetadataParent(t, root, "baseline", 20)
				args = append(args, "--based-on", "20260920-baseline")
			}
			args = append(args, "--based-on", parent)
			var stdout bytes.Buffer
			err := cli.Run(args, root, metadataTestTime(), &stdout)
			if err == nil || !strings.Contains(err.Error(), parent) {
				t.Fatalf("expected error naming missing parent %q, got %v", parent, err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("failed command printed output: %q", stdout.String())
			}
			checkDir, wantEntry := root, ".git"
			if withExistingParent {
				checkDir = filepath.Join(root, "experiments")
				wantEntry = "20260920-baseline"
			}
			entries, err := os.ReadDir(checkDir)
			if err != nil || len(entries) != 1 || entries[0].Name() != wantEntry {
				t.Fatalf("missing parent unexpectedly created files: entries=%v, err=%v", entries, err)
			}
		})
	}
}

func TestNewRejectsInvalidParentRecord(t *testing.T) {
	const parent = "20260920-baseline"
	for _, tt := range []struct {
		name   string
		readme string
	}{
		{name: "missing README"},
		{name: "malformed README", readme: "---\nid: [invalid]\ntitle: Baseline\n---\n\nOriginal research notes.\n"},
		{name: "mismatched ID", readme: "---\nid: 20260920-other\ntitle: Baseline\n---\n\nOriginal research notes.\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			dir := filepath.Join(root, "experiments", parent)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			readme := filepath.Join(dir, "README.md")
			if tt.readme != "" {
				if err := os.WriteFile(readme, []byte(tt.readme), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			notes := []byte("Existing parent notes.\n")
			notesPath := filepath.Join(dir, "notes.md")
			if err := os.WriteFile(notesPath, notes, 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout bytes.Buffer
			err := cli.Run([]string{"new", "my-idea", "--based-on", parent}, root, metadataTestTime(), &stdout)
			if err == nil || !strings.Contains(err.Error(), filepath.Join("experiments", parent, "README.md")) {
				t.Fatalf("expected error naming invalid parent README, got %v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("failed command printed output: %q", stdout.String())
			}
			entries, err := os.ReadDir(filepath.Join(root, "experiments"))
			if err != nil || len(entries) != 1 || entries[0].Name() != parent {
				t.Fatalf("invalid parent unexpectedly created files: entries=%v, err=%v", entries, err)
			}
			data, err := os.ReadFile(notesPath)
			if err != nil || !bytes.Equal(data, notes) {
				t.Fatalf("invalid parent changed existing notes: data=%q, err=%v", data, err)
			}
			data, err = os.ReadFile(readme)
			if tt.readme == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("missing parent README unexpectedly created: err=%v", err)
				}
			} else if err != nil || string(data) != tt.readme {
				t.Fatalf("invalid parent changed existing README: data=%q, err=%v", data, err)
			}
		})
	}
}

func TestNewValidatesCatalogOnlyWithParents(t *testing.T) {
	for _, tt := range []struct {
		name   string
		readme string
	}{
		{name: "malformed README", readme: "---\nid: [invalid]\ntitle: Unrelated\n---\n"},
		{name: "mismatched ID", readme: "---\nid: 20260921-other\ntitle: Unrelated\n---\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			createMetadataParent(t, root, "baseline", 20)
			createMetadataParent(t, root, "unrelated", 21)
			readme := filepath.Join(root, "experiments", "20260921-unrelated", "README.md")
			if err := os.WriteFile(readme, []byte(tt.readme), 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout bytes.Buffer
			err := cli.Run([]string{"new", "my-idea", "--based-on", "20260920-baseline"}, root, metadataTestTime(), &stdout)
			if err == nil || !strings.Contains(err.Error(), filepath.Join("20260921-unrelated", "README.md")) {
				t.Fatalf("expected unrelated record error before creation, got %v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("failed command printed output: %q", stdout.String())
			}
			if _, err := os.Stat(filepath.Join(root, "experiments", "20260924-my-idea")); !os.IsNotExist(err) {
				t.Fatalf("invalid catalog unexpectedly created child: err=%v", err)
			}
			if err := cli.Run([]string{"new", "my-idea"}, root, metadataTestTime(), &stdout); err != nil {
				t.Fatalf("independent creation rejected invalid catalog: %v", err)
			}
			if record := readNewRecord(t, root, "my-idea"); len(record.BasedOn) != 0 {
				t.Fatalf("independent record has parents: %v", record.BasedOn)
			}
			if data, err := os.ReadFile(readme); err != nil || string(data) != tt.readme {
				t.Fatalf("creation changed unrelated README: data=%q, err=%v", data, err)
			}
		})
	}
}

func TestNewRejectsExistingExperiment(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	dir, err := experiment.Create(root, "my-idea", metadataTestTime(), experiment.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(dir, "README.md")
	notes := []byte("# Existing research notes\n")
	if err := os.WriteFile(readme, notes, 0644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err = cli.Run([]string{"new", "my-idea"}, root, metadataTestTime(), &stdout)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("duplicate error = %v, want os.ErrExist", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("failed command printed output: %q", stdout.String())
	}
	data, err := os.ReadFile(readme)
	if err != nil || !bytes.Equal(data, notes) {
		t.Fatalf("duplicate changed existing notes: data=%q, err=%v", data, err)
	}
}

func TestNewHelpDescribesMetadataFlags(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	if err := cli.Run([]string{"new", "--help"}, root, metadataTestTime(), &stdout); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		flag  string
		words []string
	}{
		{flag: "--title", words: []string{"display", "default"}},
		{flag: "--based-on", words: []string{"parent", "repeat"}},
	} {
		found := false
		for _, line := range strings.Split(stdout.String(), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 0 && fields[0] == tt.flag {
				found = true
				for _, word := range tt.words {
					if !strings.Contains(strings.ToLower(line), word) {
						t.Errorf("flag description does not explain %q: %q", word, line)
					}
				}
			}
		}
		if !found {
			t.Errorf("new help does not describe %s: %q", tt.flag, stdout.String())
		}
	}
	assertEmptyDirectory(t, root)
}

func metadataTestTime() time.Time {
	return time.Date(2026, time.September, 24, 12, 0, 0, 0, time.UTC)
}

func createMetadataParent(t *testing.T, root, slug string, day int) {
	t.Helper()
	now := time.Date(2026, time.September, day, 12, 0, 0, 0, time.UTC)
	if _, err := experiment.Create(root, slug, now, experiment.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func readNewRecord(t *testing.T, root, slug string) experiment.Record {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "experiments", "20260924-"+slug, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := experiment.Parse(data)
	if err != nil {
		t.Fatalf("parse generated experiment: %v", err)
	}
	return record
}
