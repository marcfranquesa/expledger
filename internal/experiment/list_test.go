package experiment

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestList(t *testing.T) {
	root := t.TempDir()
	writeListREADME(t, root, "a-older", "---\nid: 20260920-baseline\ntitle: Baseline\n---\n# Baseline\n")
	writeListREADME(t, root, "m-newer", "---\nid: 20260924-improved\ntitle: Improved model\nbased_on: [20260920-baseline, 20260919-reference]\n---\n# Findings\n")
	writeListREADME(t, root, "z-oldest", "---\nid: 20260919-reference\ntitle: Reference\n---\n")

	records, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %+v, want three experiments", records)
	}
	if records[0].ID != "20260924-improved" || records[0].Title != "Improved model" ||
		!reflect.DeepEqual(records[0].BasedOn, []string{"20260920-baseline", "20260919-reference"}) ||
		string(records[0].Body) != "# Findings\n" {
		t.Fatalf("newest record = %+v, want improved model with its parents and body", records[0])
	}
	if records[1].ID != "20260920-baseline" || records[1].Title != "Baseline" || len(records[1].BasedOn) != 0 {
		t.Fatalf("older record = %+v, want independent baseline", records[1])
	}
	if records[2].ID != "20260919-reference" {
		t.Fatalf("oldest record = %+v, want reference", records[2])
	}
}

func TestListEmpty(t *testing.T) {
	for _, exists := range []bool{false, true} {
		name := "absent"
		if exists {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if exists {
				if err := os.Mkdir(filepath.Join(root, "experiments"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			records, err := List(root)
			if err != nil || len(records) != 0 {
				t.Fatalf("List = %+v, %v; want empty success", records, err)
			}
			entries, err := os.ReadDir(root)
			want := 0
			if exists {
				want = 1
			}
			if err != nil || len(entries) != want {
				t.Fatalf("List changed the project: entries=%v, err=%v", entries, err)
			}
		})
	}
}

func TestListIgnoresFilesSymlinkDirectoriesAndNestedArtifacts(t *testing.T) {
	root := t.TempDir()
	writeListREADME(t, root, "baseline", "---\nid: 20260920-baseline\ntitle: Baseline\n---\n")
	writeListREADME(t, root, filepath.Join("baseline", "artifacts"), "not experiment metadata")
	if err := os.WriteFile(filepath.Join(root, "experiments", "README.md"), []byte("loose notes"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "experiments", "linked")); err != nil {
		t.Fatal(err)
	}
	records, err := List(root)
	if err != nil || len(records) != 1 || records[0].ID != "20260920-baseline" {
		t.Fatalf("List = %+v, %v; want only the baseline", records, err)
	}
}

func TestListInvalidREADME(t *testing.T) {
	for _, content := range []string{"", "not front matter", "---\nid: missing-title\n---\n"} {
		t.Run(content, func(t *testing.T) {
			root := t.TempDir()
			writeListREADME(t, root, "a-valid", "---\nid: baseline\ntitle: Baseline\n---\n")
			if content == "" {
				if err := os.Mkdir(filepath.Join(root, "experiments", "z-invalid"), 0755); err != nil {
					t.Fatal(err)
				}
			} else {
				writeListREADME(t, root, "z-invalid", content)
			}
			records, err := List(root)
			if err == nil || !strings.Contains(err.Error(), filepath.Join("experiments", "z-invalid", "README.md")) {
				t.Fatalf("error = %v, want the invalid README path", err)
			}
			if len(records) != 0 {
				t.Fatalf("returned partial results: %+v", records)
			}
		})
	}
}

func TestListRejectsInvalidExperimentsDirectory(t *testing.T) {
	for _, kind := range []string{"file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "experiments")
			var err error
			if kind == "symlink" {
				err = os.Symlink(t.TempDir(), path)
			} else {
				err = os.WriteFile(path, nil, 0644)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := List(root); err == nil || !strings.Contains(err.Error(), "experiments must be a directory") {
				t.Fatalf("error = %v, want invalid experiments directory", err)
			}
		})
	}
}

func TestListRejectsREADMEOutsideProject(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	path := filepath.Join(root, "experiments", "linked")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(outside, "README.md")
	if err := os.WriteFile(target, []byte("---\nid: outside\ntitle: Outside\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(path, "README.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := List(root); err == nil || !strings.Contains(err.Error(), filepath.Join("experiments", "linked", "README.md")) {
		t.Fatalf("error = %v, want an error at the escaped README path", err)
	}
}

func writeListREADME(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, "experiments", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
