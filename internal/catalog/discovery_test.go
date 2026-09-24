package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoveryOnlyReadsSidecars(t *testing.T) {
	root := t.TempDir()
	writeMetadata(t, root, "ours", "schema: expledger/v1\nid: ours\ntitle: Ours\ncreated_at: 2026-09-24T12:00:00Z\n")
	for _, name := range []string{"legacy", "labexp", `foreign\name`, " \t"} {
		dir := filepath.Join(root, "experiments", name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		// Plausible legacy metadata is not a sidecar record.
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("---\nid: legacy\ntitle: Legacy\ncreated_at: 2026-09-24T12:00:00Z\n---\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "labexp.yaml"), []byte("not even valid: ["), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// ExpLedger metadata can be read without any README or notes convention.
	records, err := List(root)
	if err != nil || len(records) != 1 || records[0].ID != "ours" {
		t.Fatalf("List = %+v, %v; want only ours", records, err)
	}
	if _, err := Read(root, "ours"); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, "legacy"); err == nil || !strings.Contains(err.Error(), "expledger.yaml") {
		t.Fatalf("legacy read = %v; want missing sidecar", err)
	}
}

func TestDiscoveryRejectsBrokenSidecarLink(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "experiments", "broken")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("absent.yaml", filepath.Join(dir, "expledger.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := List(root); err == nil || !strings.Contains(err.Error(), "expledger.yaml") {
		t.Fatalf("List = %v; want error for present broken sidecar", err)
	}
}
