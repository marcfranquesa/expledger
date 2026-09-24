package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadsRelativeMetadataSymlinkWithinProject(t *testing.T) {
	root := t.TempDir()
	const content = "schema: expledger/v1\nid: linked\ntitle: Linked\ncreated_at: 2026-09-24T12:00:00Z\n"
	dir := filepath.Join(root, "experiments", "linked")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// The target lives outside the experiment directory, but inside the project.
	target := filepath.Join(root, "metadata.yaml")
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(dir, "expledger.yaml")
	relativeTarget := filepath.Join("..", "..", "metadata.yaml")
	if err := os.Symlink(relativeTarget, metadata); err != nil {
		t.Fatal(err)
	}
	record, err := Read(root, "linked")
	if err != nil || record.ID != "linked" || record.Title != "Linked" {
		t.Fatalf("Read = %+v, %v; want linked metadata", record, err)
	}
	records, err := List(root)
	if err != nil || len(records) != 1 || !reflect.DeepEqual(records[0], record) {
		t.Fatalf("List = %+v, %v; want the same linked record", records, err)
	}
	if link, err := os.Readlink(metadata); err != nil || link != relativeTarget {
		t.Fatalf("metadata symlink changed: link=%q, err=%v", link, err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != content {
		t.Fatalf("metadata target changed: data=%q, err=%v", data, err)
	}
}
