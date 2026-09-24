package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestList(t *testing.T) {
	root := t.TempDir()
	writeListREADME(t, root, "20260924-baseline", "---\nid: 20260924-baseline\ntitle: Baseline\ncreated_at: 2026-09-24T10:00:00Z\n---\n# Baseline\n")
	writeListREADME(t, root, "20260924-improved", "---\nid: 20260924-improved\ntitle: Improved model\ncreated_at: 2026-09-24T12:00:00Z\nbased_on: [20260924-baseline, 20260924-reference]\n---\n# Findings\n")
	writeListREADME(t, root, "20260924-reference", "---\nid: 20260924-reference\ntitle: Reference\ncreated_at: 2026-09-24T09:00:00Z\n---\n")

	records, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %+v, want three experiments", records)
	}
	if records[0].ID != "20260924-improved" || records[0].Title != "Improved model" ||
		!reflect.DeepEqual(records[0].BasedOn, []string{"20260924-baseline", "20260924-reference"}) ||
		string(records[0].Body) != "# Findings\n" {
		t.Fatalf("newest record = %+v, want improved model with its parents and body", records[0])
	}
	if records[1].ID != "20260924-baseline" || records[1].Title != "Baseline" || len(records[1].BasedOn) != 0 {
		t.Fatalf("older record = %+v, want independent baseline", records[1])
	}
	if records[2].ID != "20260924-reference" {
		t.Fatalf("oldest record = %+v, want reference", records[2])
	}

}

func TestListOrdersTimestampInstants(t *testing.T) {
	root := t.TempDir()
	writeListREADME(t, root, "a-older", "---\nid: a-older\ntitle: Older\ncreated_at: 2026-09-24T12:00:00+02:00\n---\n")
	writeListREADME(t, root, "b-newer", "---\nid: b-newer\ntitle: Newer\ncreated_at: 2026-09-24T11:00:00Z\n---\n")
	writeListREADME(t, root, "c-same-instant", "---\nid: c-same-instant\ntitle: Same instant\ncreated_at: 2026-09-24T07:00:00-04:00\n---\n")
	writeListREADME(t, root, "d-fractionally-newest", "---\nid: d-fractionally-newest\ntitle: Fractionally newest\ncreated_at: 2026-09-24T11:00:00.000000001Z\n---\n")

	records, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	want := []string{"d-fractionally-newest", "b-newer", "c-same-instant", "a-older"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("IDs = %v, want %v", ids, want)
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
	writeListREADME(t, root, "20260920-baseline", "---\nid: 20260920-baseline\ntitle: Baseline\n---\n")
	writeListREADME(t, root, filepath.Join("20260920-baseline", "artifacts"), "not experiment metadata")
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
			writeListREADME(t, root, "a-valid", "---\nid: a-valid\ntitle: Baseline\n---\n")
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

func TestListRejectsMismatchedIDs(t *testing.T) {
	for _, id := range []string{"different", "Z-folder", "z-folder ", "a-valid"} {
		t.Run(id, func(t *testing.T) {
			root := t.TempDir()
			writeListREADME(t, root, "a-valid", "---\nid: a-valid\ntitle: Valid\n---\n")
			// Reusing a-valid also checks duplicate metadata in different folders.
			content := fmt.Sprintf("---\nid: %q\ntitle: Notes\n---\n# Keep these notes\n", id)
			writeListREADME(t, root, "z-folder", content)
			readme := filepath.Join("experiments", "z-folder", "README.md")
			records, err := List(root)
			if err == nil {
				t.Fatal("accepted mismatched ID")
			}
			for _, want := range []string{readme, fmt.Sprintf("%q", id), `"z-folder"`} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %v, want %q", err, want)
				}
			}
			if len(records) != 0 {
				t.Fatalf("returned partial results: %+v", records)
			}
			data, err := os.ReadFile(filepath.Join(root, readme))
			if err != nil || string(data) != content {
				t.Fatalf("List changed README: data=%q, err=%v", data, err)
			}
		})
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
