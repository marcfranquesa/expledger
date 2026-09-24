package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookup(t *testing.T) {
	root := t.TempDir()
	writeListREADME(t, root, "baseline", "---\nid: baseline\ntitle: Baseline\n---\n# Notes\n")
	record, err := Lookup(root, "baseline")
	if err != nil || record.ID != "baseline" || record.Title != "Baseline" || string(record.Body) != "# Notes\n" {
		t.Fatalf("Lookup = %+v, %v; want baseline record", record, err)
	}
	for _, id := range []string{"missing", "Baseline"} {
		if _, err := Lookup(root, id); !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), id) {
			t.Fatalf("Lookup(%q) error = %v; want missing ID error", id, err)
		}
	}
}

func TestLookupEmpty(t *testing.T) {
	root := t.TempDir()
	if _, err := Lookup(root, "baseline"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lookup error = %v; want missing ID error", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("Lookup changed project: entries=%v, err=%v", entries, err)
	}
}

func TestLookupValidatesWholeCatalog(t *testing.T) {
	for _, content := range []string{"", "not front matter", "---\nid: different\ntitle: Invalid\n---\n"} {
		t.Run(content, func(t *testing.T) {
			root := t.TempDir()
			writeListREADME(t, root, "a-valid", "---\nid: a-valid\ntitle: Valid\n---\n")
			if content == "" {
				if err := os.Mkdir(filepath.Join(root, "experiments", "z-invalid"), 0755); err != nil {
					t.Fatal(err)
				}
			} else {
				writeListREADME(t, root, "z-invalid", content)
			}
			for _, id := range []string{"a-valid", "z-invalid"} {
				record, err := Lookup(root, id)
				if err == nil || !strings.Contains(err.Error(), filepath.Join("z-invalid", "README.md")) {
					t.Fatalf("Lookup(%q) error = %v; want invalid README path", id, err)
				}
				if record.ID != "" {
					t.Fatalf("Lookup returned a record from an invalid catalog: %+v", record)
				}
			}
		})
	}
}
