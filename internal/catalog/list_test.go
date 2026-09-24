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
	root := filepath.Join("..", "..", "testdata", "project")

	records, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %+v, want three experiments", records)
	}
	if records[0].ID != "20260924-long-title" || records[0].Title != "Unicode & HTML: comparing café embeddings with α < β across a deliberately long experiment title" {
		t.Fatalf("newest record = %+v, want long title", records[0])
	}
	if records[1].ID != "20260924-variant" || records[1].Title != "Lower learning rate" ||
		!reflect.DeepEqual(records[1].BasedOn, []string{"20260924-baseline"}) {
		t.Fatalf("middle record = %+v, want variant based on baseline", records[1])
	}
	if records[2].ID != "20260924-baseline" || records[2].Title != "Baseline model" || len(records[2].BasedOn) != 0 {
		t.Fatalf("oldest record = %+v, want independent baseline", records[2])
	}

}

func TestListOrdersTimestampInstants(t *testing.T) {
	root := t.TempDir()
	writeMetadata(t, root, "a-older", "schema: expledger/v1\nid: a-older\ntitle: Older\ncreated_at: 2026-09-24T12:00:00+02:00\n")
	writeMetadata(t, root, "b-newer", "schema: expledger/v1\nid: b-newer\ntitle: Newer\ncreated_at: 2026-09-24T11:00:00Z\n")
	writeMetadata(t, root, "c-same-instant", "schema: expledger/v1\nid: c-same-instant\ntitle: Same instant\ncreated_at: 2026-09-24T07:00:00-04:00\n")
	writeMetadata(t, root, "d-fractionally-newest", "schema: expledger/v1\nid: d-fractionally-newest\ntitle: Fractionally newest\ncreated_at: 2026-09-24T11:00:00.000000001Z\n")

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
	writeMetadata(t, root, "20260920-baseline", "schema: expledger/v1\nid: 20260920-baseline\ntitle: Baseline\ncreated_at: 2026-09-20T12:00:00Z\n")
	writeMetadata(t, root, filepath.Join("20260920-baseline", "artifacts"), "not experiment metadata")
	if err := os.WriteFile(filepath.Join(root, "experiments", "expledger.yaml"), []byte("loose notes"), 0644); err != nil {
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

func TestListInvalidMetadata(t *testing.T) {
	for _, content := range []string{"", "not YAML metadata", "schema: expledger/v1\nid: missing-title\ncreated_at: 2026-09-24T12:00:00Z\n"} {
		t.Run(content, func(t *testing.T) {
			root := t.TempDir()
			writeMetadata(t, root, "a-valid", "schema: expledger/v1\nid: a-valid\ntitle: Baseline\ncreated_at: 2026-09-24T12:00:00Z\n")
			writeMetadata(t, root, "z-invalid", content)
			records, err := List(root)
			if err == nil || !strings.Contains(err.Error(), filepath.Join("experiments", "z-invalid", "expledger.yaml")) {
				t.Fatalf("error = %v, want the invalid metadata path", err)
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

func TestListRejectsMetadataOutsideProject(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	path := filepath.Join(root, "experiments", "linked")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(outside, "expledger.yaml")
	if err := os.WriteFile(target, []byte("schema: expledger/v1\nid: outside\ntitle: Outside\ncreated_at: 2026-09-24T12:00:00Z\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(path, "expledger.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := List(root); err == nil || !strings.Contains(err.Error(), filepath.Join("experiments", "linked", "expledger.yaml")) {
		t.Fatalf("error = %v, want an error at the escaped metadata path", err)
	}
}

func TestListRejectsMismatchedIDs(t *testing.T) {
	for _, id := range []string{"different", "Z-folder", "z-folder ", "a-valid"} {
		t.Run(id, func(t *testing.T) {
			root := t.TempDir()
			writeMetadata(t, root, "a-valid", "schema: expledger/v1\nid: a-valid\ntitle: Valid\ncreated_at: 2026-09-24T12:00:00Z\n")
			// Reusing a-valid also checks duplicate metadata in different folders.
			content := fmt.Sprintf("schema: expledger/v1\nid: %q\ntitle: Notes\ncreated_at: 2026-09-24T12:00:00Z\n", id)
			writeMetadata(t, root, "z-folder", content)
			metadata := filepath.Join("experiments", "z-folder", "expledger.yaml")
			records, err := List(root)
			if err == nil {
				t.Fatal("accepted mismatched ID")
			}
			for _, want := range []string{metadata, fmt.Sprintf("%q", id), `"z-folder"`} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %v, want %q", err, want)
				}
			}
			if len(records) != 0 {
				t.Fatalf("returned partial results: %+v", records)
			}
			data, err := os.ReadFile(filepath.Join(root, metadata))
			if err != nil || string(data) != content {
				t.Fatalf("List changed metadata: data=%q, err=%v", data, err)
			}
		})
	}
}

func writeMetadata(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, "experiments", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "expledger.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestListRecordsCanBeRead(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"20260924-baseline", "Existing notes", "café-α", " leading and trailing "} {
		content := fmt.Sprintf("schema: expledger/v1\nid: %q\ntitle: Notes\ncreated_at: 2026-09-24T12:00:00Z\n", id)
		writeMetadata(t, root, id, content)
	}
	records, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 4 {
		t.Fatalf("List returned %d records, want 4", len(records))
	}
	for _, listed := range records {
		read, err := Read(root, listed.ID)
		if err != nil || !reflect.DeepEqual(read, listed) {
			t.Errorf("Read(%q) = %+v, %v; want listed record %+v", listed.ID, read, err, listed)
		}
	}
}

func TestListRejectsInvalidDirectoryIDs(t *testing.T) {
	for _, id := range []string{`z\invalid`, " \t"} {
		t.Run(id, func(t *testing.T) {
			root := t.TempDir()
			writeMetadata(t, root, "a-valid", "schema: expledger/v1\nid: a-valid\ntitle: Valid\ncreated_at: 2026-09-24T12:00:00Z\n")
			content := fmt.Sprintf("schema: expledger/v1\nid: %q\ntitle: Notes\ncreated_at: 2026-09-24T12:00:00Z\n", id)
			writeMetadata(t, root, id, content)
			records, err := List(root)
			_, readErr := Read(root, id)
			if err == nil || readErr == nil || err.Error() != readErr.Error() || !strings.Contains(err.Error(), "invalid experiment ID") {
				t.Fatalf("List error = %v, Read error = %v; want same invalid ID error", err, readErr)
			}
			if len(records) != 0 {
				t.Fatalf("returned partial results: %+v", records)
			}
		})
	}
}
