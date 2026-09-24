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
	const content = "---\nid: example\ntitle: Example\ncreated_at: 2026-09-24T14:30:00Z\nbased_on: [missing]\ncustom: keep-me\n---\n# Notes\n"
	writeListREADME(t, root, "example", content)
	record, err := Read(root, "example")
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "example" || record.Title != "Example" || record.CreatedAt.IsZero() ||
		len(record.BasedOn) != 1 || record.BasedOn[0] != "missing" ||
		record.Extra["custom"].Value != "keep-me" || string(record.Body) != "# Notes\n" {
		t.Fatalf("unexpected record: %+v", record)
	}
	data, err := os.ReadFile(filepath.Join(root, "experiments", "example", "README.md"))
	if err != nil || string(data) != content {
		t.Fatalf("Read changed README: data=%q, err=%v", data, err)
	}
}

func TestReadRejectsMismatchedID(t *testing.T) {
	root := t.TempDir()
	writeListREADME(t, root, "20260924-test", "---\nid: 20260924-other\ntitle: Test\ncreated_at: 2026-09-24T14:30:00Z\n---\n")
	_, err := Read(root, "20260924-test")
	want := "invalid " + filepath.Join("experiments", "20260924-test", "README.md") + ": YAML id \"20260924-other\" must match folder name \"20260924-test\""
	if err == nil || err.Error() != want {
		t.Fatalf("Read error = %v, want %q", err, want)
	}
}

func TestReadReportsParseErrorWithPath(t *testing.T) {
	root := t.TempDir()
	writeListREADME(t, root, "example", "---\nid: [\n---\n")
	_, err := Read(root, "example")
	if err == nil || !strings.Contains(err.Error(), filepath.Join("experiments", "example", "README.md")) || !strings.Contains(err.Error(), "parse metadata") {
		t.Fatalf("Read error = %v, want README path and parser error", err)
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

func TestReadRejectsREADMEOutsideProject(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	dir := filepath.Join(root, "experiments", "example")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(outside, "README.md")
	if err := os.WriteFile(target, []byte("---\nid: example\ntitle: Example\ncreated_at: 2026-09-24T14:30:00Z\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, "example"); err == nil || !strings.Contains(err.Error(), "read "+filepath.Join("experiments", "example", "README.md")) {
		t.Fatalf("Read error = %v, want an error reading escaped README", err)
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
