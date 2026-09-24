package experiment

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreate(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 24, 23, 59, 0, 0, time.FixedZone("local", -4*60*60))
	dir, err := Create(root, "my-idea", now)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "experiments", "20260924-my-idea"); dir != want {
		t.Fatalf("directory = %q, want %q", dir, want)
	}
	path := filepath.Join(dir, "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	record, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "20260924-my-idea" || record.Title != "My idea" {
		t.Fatalf("unexpected metadata: %+v", record)
	}
	if string(record.Body) != "\n# My idea\n\n## Hypothesis\n\n## Method\n\n## Finding\n" {
		t.Fatalf("unexpected body: %q", record.Body)
	}
	if _, exists := record.Extra["status"]; exists {
		t.Fatal("unexpected status field")
	}

	if err := os.WriteFile(path, []byte("existing research notes"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(root, "my-idea", now); !errors.Is(err, os.ErrExist) {
		t.Fatalf("duplicate creation error = %v, want os.ErrExist", err)
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "existing research notes" {
		t.Fatalf("duplicate attempt changed original: data=%q, err=%v", data, err)
	}
}

func TestCreateRejectsInvalidSlugs(t *testing.T) {
	for _, slug := range []string{"", "../outside", "/absolute", "a/b", `a\b`, "Upper", "a b", "a--b", "a_1", "-a", "a-"} {
		t.Run(slug, func(t *testing.T) {
			root := t.TempDir()
			if _, err := Create(root, slug, time.Now()); err == nil {
				t.Fatal("invalid slug accepted")
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid input changed project: entries=%v, err=%v", entries, err)
			}
		})
	}
}

func TestCreateRejectsSymlink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "experiments")); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(root, "baseline", time.Now()); err == nil {
		t.Fatal("symlinked experiments directory accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("wrote through symlink: entries=%v, err=%v", entries, err)
	}
}
