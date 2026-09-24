package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/catalog"
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
			if err := cli.Run(context.Background(), tt.args, root, metadataTestTime(), &stdout); err != nil {
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
			readme := readNewREADME(t, root, "my-idea")
			if !strings.HasPrefix(string(readme), "# "+title+"\n") {
				t.Fatalf("README heading does not use the title: %q", readme)
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
			if err := cli.Run(context.Background(), args, root, metadataTestTime(), &stdout); err == nil {
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
	if err := cli.Run(context.Background(), []string{"new", "first-idea", "--title", "Custom title", "--based-on", "20260920-baseline"}, root, metadataTestTime(), &stdout); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := cli.Run(context.Background(), []string{"new", "next-idea"}, root, metadataTestTime(), &stdout); err != nil {
		t.Fatal(err)
	}
	record := readNewRecord(t, root, "next-idea")
	if record.Title != "Next idea" {
		t.Fatalf("default title = %q, want %q", record.Title, "Next idea")
	}
	if len(record.BasedOn) != 0 {
		t.Fatalf("default based_on contains previous flags: %v", record.BasedOn)
	}
	readme := readNewREADME(t, root, "next-idea")
	if !strings.HasPrefix(string(readme), "# Next idea\n") {
		t.Fatalf("default README heading = %q", readme)
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
			err := cli.Run(context.Background(), args, root, metadataTestTime(), &stdout)
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
		name     string
		folder   string
		metadata string
	}{
		{name: "missing metadata"},
		{name: "malformed metadata", metadata: "schema: expledger/v1\nid: [invalid]\ntitle: Baseline\ncreated_at: 2026-09-20T12:00:00Z\n"},
		{name: "mismatched ID", metadata: "schema: expledger/v1\nid: 20260920-other\ntitle: Baseline\ncreated_at: 2026-09-20T12:00:00Z\n"},
		{name: "case mismatched folder", folder: "20260920-Baseline", metadata: "schema: expledger/v1\nid: 20260920-baseline\ntitle: Baseline\ncreated_at: 2026-09-20T12:00:00Z\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			folder := tt.folder
			if folder == "" {
				folder = parent
			}
			dir := filepath.Join(root, "experiments", folder)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			metadata := filepath.Join(dir, "expledger.yaml")
			if tt.metadata != "" {
				if err := os.WriteFile(metadata, []byte(tt.metadata), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			notes := []byte("Existing parent notes.\n")
			notesPath := filepath.Join(dir, "notes.md")
			if err := os.WriteFile(notesPath, notes, 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout bytes.Buffer
			err := cli.Run(context.Background(), []string{"new", "my-idea", "--based-on", parent}, root, metadataTestTime(), &stdout)
			if err == nil || !strings.Contains(err.Error(), filepath.Join("experiments", parent, "expledger.yaml")) {
				t.Fatalf("expected error naming invalid parent metadata, got %v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("failed command printed output: %q", stdout.String())
			}
			entries, err := os.ReadDir(filepath.Join(root, "experiments"))
			if err != nil || len(entries) != 1 || entries[0].Name() != folder {
				t.Fatalf("invalid parent unexpectedly created files: entries=%v, err=%v", entries, err)
			}
			data, err := os.ReadFile(notesPath)
			if err != nil || !bytes.Equal(data, notes) {
				t.Fatalf("invalid parent changed existing notes: data=%q, err=%v", data, err)
			}
			data, err = os.ReadFile(metadata)
			if tt.metadata == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("missing parent metadata unexpectedly created: err=%v", err)
				}
			} else if err != nil || string(data) != tt.metadata {
				t.Fatalf("invalid parent changed existing metadata: data=%q, err=%v", data, err)
			}
		})
	}
}

func TestNewIgnoresInvalidUnrelatedExperiments(t *testing.T) {
	const parent = "20260920-baseline"
	for _, tt := range []struct {
		name     string
		metadata string
	}{
		{name: "missing metadata"},
		{name: "malformed metadata", metadata: "schema: expledger/v1\nid: [invalid]\ntitle: Unrelated\ncreated_at: 2026-09-21T12:00:00Z\n"},
		{name: "mismatched ID", metadata: "schema: expledger/v1\nid: 20260921-other\ntitle: Unrelated\ncreated_at: 2026-09-21T12:00:00Z\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			createMetadataParent(t, root, "baseline", 20)
			createMetadataParent(t, root, "unrelated", 21)
			metadata := filepath.Join(root, "experiments", "20260921-unrelated", "expledger.yaml")
			var err error
			if tt.metadata == "" {
				err = os.Remove(metadata)
			} else {
				err = os.WriteFile(metadata, []byte(tt.metadata), 0o644)
			}
			if err != nil {
				t.Fatal(err)
			}
			var stdout bytes.Buffer
			if err := cli.Run(context.Background(), []string{"new", "my-idea", "--based-on", parent}, root, metadataTestTime(), &stdout); err != nil {
				t.Fatalf("valid parent rejected because of unrelated experiment: %v", err)
			}
			if got := readNewRecord(t, root, "my-idea").BasedOn; !reflect.DeepEqual(got, []string{parent}) {
				t.Fatalf("based_on = %v, want [%s]", got, parent)
			}
			if err := cli.Run(context.Background(), []string{"new", "independent"}, root, metadataTestTime(), &stdout); err != nil {
				t.Fatalf("independent creation rejected invalid catalog: %v", err)
			}
			if record := readNewRecord(t, root, "independent"); len(record.BasedOn) != 0 {
				t.Fatalf("independent record has parents: %v", record.BasedOn)
			}
			data, err := os.ReadFile(metadata)
			if tt.metadata == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("creation changed missing unrelated metadata: err=%v", err)
				}
			} else if err != nil || string(data) != tt.metadata {
				t.Fatalf("creation changed unrelated metadata: data=%q, err=%v", data, err)
			}
		})
	}
}

func TestNewRejectsExistingExperiment(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	dir, err := catalog.Create(root, "my-idea", metadataTestTime(), catalog.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(dir, "README.md")
	notes := []byte("# Existing research notes\n")
	if err := os.WriteFile(readme, notes, 0644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err = cli.Run(context.Background(), []string{"new", "my-idea"}, root, metadataTestTime(), &stdout)
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

func TestNewDoesNotValidateAncestors(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	const parent = "20260920-baseline"
	writeMetadata(t, root, parent, []byte("schema: expledger/v1\nid: "+parent+"\ntitle: Baseline\ncreated_at: 2026-09-20T12:00:00Z\nbased_on: [20260919-missing]\n"))
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"new", "my-idea", "--based-on", parent}, root, metadataTestTime(), &stdout); err != nil {
		t.Fatalf("valid direct parent was rejected: %v", err)
	}
	if got := readNewRecord(t, root, "my-idea").BasedOn; !reflect.DeepEqual(got, []string{parent}) {
		t.Fatalf("based_on = %v, want [%s]", got, parent)
	}
}

func TestNewHelpDescribesMetadataFlags(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"new", "--help"}, root, metadataTestTime(), &stdout); err != nil {
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
	if _, err := catalog.Create(root, slug, now, catalog.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func readNewRecord(t *testing.T, root, slug string) experiment.Record {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "experiments", "20260924-"+slug, "expledger.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := experiment.Parse(data)
	if err != nil {
		t.Fatalf("parse generated experiment: %v", err)
	}
	return record
}

func readNewREADME(t *testing.T, root, slug string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "experiments", "20260924-"+slug, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
