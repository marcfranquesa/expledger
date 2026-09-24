package experiment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExists(t *testing.T) {
	root := t.TempDir()
	if exists, err := Exists(root, "baseline"); err != nil || exists {
		t.Fatalf("missing experiments directory: Exists = %v, %v", exists, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("Exists changed project: entries=%v, err=%v", entries, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "experiments", "baseline"), 0755); err != nil {
		t.Fatal(err)
	}
	if exists, err := Exists(root, "baseline"); err != nil || !exists {
		t.Fatalf("directory without README: Exists = %v, %v", exists, err)
	}
	if exists, err := Exists(root, "missing"); err != nil || exists {
		t.Fatalf("missing experiment: Exists = %v, %v", exists, err)
	}
}

func TestExistsRejectsUnsafeIDs(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"", " \t\n", ".", "..", "../outside", "/absolute", "a/b", `a\b`, "nul\x00id"} {
		t.Run(id, func(t *testing.T) {
			if exists, err := Exists(root, id); err == nil || exists {
				t.Fatalf("Exists(%q) = %v, %v; want invalid ID error", id, exists, err)
			}
		})
	}
}

func TestExistsRejectsNonDirectories(t *testing.T) {
	for _, location := range []string{"experiments", filepath.Join("experiments", "baseline")} {
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
				if exists, err := Exists(root, "baseline"); err == nil || exists || !strings.Contains(err.Error(), "must be a directory") {
					t.Fatalf("Exists = %v, %v; want directory type error", exists, err)
				}
			})
		}
	}
}

func TestExistsRequiresProjectDirectory(t *testing.T) {
	if exists, err := Exists(filepath.Join(t.TempDir(), "missing"), "baseline"); err == nil || exists {
		t.Fatalf("Exists = %v, %v; want missing project error", exists, err)
	}
}
