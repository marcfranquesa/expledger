package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRead(t *testing.T) {
	root := t.TempDir()
	const content = "schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: 2026-09-24T14:30:00Z\nbased_on: [missing]\ncustom: keep-me\n"
	writeMetadata(t, root, "example", content)
	record, err := Read(root, "example")
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "example" || record.Title != "Example" || record.CreatedAt.IsZero() ||
		len(record.BasedOn) != 1 || record.BasedOn[0] != "missing" {
		t.Fatalf("unexpected record: %+v", record)
	}
	data, err := os.ReadFile(filepath.Join(root, "experiments", "example", "expledger.yaml"))
	if err != nil || string(data) != content {
		t.Fatalf("Read changed metadata: data=%q, err=%v", data, err)
	}
}

func TestReadRejectsMismatchedID(t *testing.T) {
	root := t.TempDir()
	writeMetadata(t, root, "20260924-test", "schema: expledger/v1\nid: 20260924-other\ntitle: Test\ncreated_at: 2026-09-24T14:30:00Z\n")
	_, err := Read(root, "20260924-test")
	want := "invalid " + filepath.Join("experiments", "20260924-test", "expledger.yaml") + ": YAML id \"20260924-other\" must match folder name \"20260924-test\""
	if err == nil || err.Error() != want {
		t.Fatalf("Read error = %v, want %q", err, want)
	}
}

func TestReadReportsParseErrorWithPath(t *testing.T) {
	root := t.TempDir()
	writeMetadata(t, root, "example", "schema: expledger/v1\nid: [\n")
	_, err := Read(root, "example")
	if err == nil || !strings.Contains(err.Error(), filepath.Join("experiments", "example", "expledger.yaml")) || !strings.Contains(err.Error(), "parse metadata") {
		t.Fatalf("Read error = %v, want metadata path and parser error", err)
	}
}

func TestReadRejectsInvalidID(t *testing.T) {
	for _, id := range []string{"", " ", ".", "..", "../outside", "nested/experiment", `nested\experiment`, "nul\x00id"} {
		t.Run(id, func(t *testing.T) {
			if _, err := Read(t.TempDir(), id); err == nil || !strings.Contains(err.Error(), "invalid experiment ID") {
				t.Fatalf("Read error = %v, want invalid experiment ID", err)
			}
		})
	}
}

func TestReadRejectsMetadataOutsideProject(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	dir := filepath.Join(root, "experiments", "example")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(outside, "expledger.yaml")
	if err := os.WriteFile(target, []byte("schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: 2026-09-24T14:30:00Z\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "expledger.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, "example"); err == nil || !strings.Contains(err.Error(), "read "+filepath.Join("experiments", "example", "expledger.yaml")) {
		t.Fatalf("Read error = %v, want an error reading escaped metadata", err)
	}
}

func TestReadMissingExperiment(t *testing.T) {
	if _, err := Read(t.TempDir(), "missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Read error = %v, want os.ErrNotExist", err)
	}
}

func TestReadRejectsNonDirectories(t *testing.T) {
	for _, location := range []string{"experiments", filepath.Join("experiments", "example")} {
		for _, kind := range []string{"file", "symlink"} {
			t.Run(location+"/"+kind, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, location)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				var err error
				if kind == "symlink" {
					err = os.Symlink(t.TempDir(), path)
				} else {
					err = os.WriteFile(path, nil, 0644)
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, err := Read(root, "example"); err == nil || !strings.Contains(err.Error(), location+" must be a directory") {
					t.Fatalf("Read error = %v, want directory type error", err)
				}
			})
		}
	}
}
