package catalog

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/experiment"
	"go.yaml.in/yaml/v3"
)

const runMetadata = "schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\nbased_on: [missing]\ncustom: {seed: 18446744073709551617, missing: null}\n"

func TestRecordRunReadsFreshMetadataAndReplacesAtomically(t *testing.T) {
	root := t.TempDir()
	writeMetadata(t, root, "example", runMetadata)
	if _, err := Read(root, "example"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "experiments", "example", "expledger.yaml")
	latest := strings.Replace(runMetadata, "title: Example", "title: Updated while preparing the run", 1)
	if err := os.WriteFile(path, []byte(latest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	originalFile, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer originalFile.Close()
	receipt := testRunReceipt()
	if err := RecordRun(root, "example", receipt); err != nil {
		t.Fatal(err)
	}
	record, err := Read(root, "example")
	if err != nil {
		t.Fatal(err)
	}
	if record.Title != "Updated while preparing the run" || len(record.BasedOn) != 1 || record.BasedOn[0] != "missing" {
		t.Fatalf("run changed unrelated metadata: %+v", record)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	custom := metadataField(t, updated, "custom")
	originalCustom := metadataField(t, []byte(latest), "custom")
	if len(custom.Content) != 4 || custom.Content[1].Value != originalCustom.Content[1].Value || custom.Content[1].Tag != originalCustom.Content[1].Tag || custom.Content[3].Tag != "!!null" {
		t.Fatalf("run changed custom YAML values: %+v", custom)
	}
	if record.LastRun == nil || *record.LastRun != receipt {
		t.Fatalf("receipt=%+v, want=%+v", record.LastRun, receipt)
	}
	oldData, err := io.ReadAll(originalFile)
	if err != nil || string(oldData) != latest {
		t.Fatalf("metadata was modified in place: data=%q, err=%v", oldData, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("metadata mode changed: info=%v, err=%v", info, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != "expledger.yaml" {
		t.Fatalf("temporary files remain: entries=%v, err=%v", entries, err)
	}

	receipt.ProjectCommit = strings.Repeat("b", 64)
	receipt.StartedAt = receipt.StartedAt.Add(time.Hour)
	if err := RecordRun(root, "example", receipt); err != nil {
		t.Fatal(err)
	}
	record, err = Read(root, "example")
	if err != nil || record.LastRun == nil || *record.LastRun != receipt {
		t.Fatalf("new receipt=%+v, err=%v, want=%+v", record.LastRun, err, receipt)
	}
}

func metadataField(t *testing.T, data []byte, name string) *yaml.Node {
	t.Helper()
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	fields := document.Content[0].Content
	for i := 0; i < len(fields); i += 2 {
		if fields[i].Value == name {
			return fields[i+1]
		}
	}
	t.Fatalf("missing metadata field %s", name)
	return nil
}

func TestRecordRunRejectsInvalidMetadataWithoutChangingIt(t *testing.T) {
	for name, metadata := range map[string]string{
		"invalid YAML":    "schema: [\n",
		"different ID":    strings.Replace(runMetadata, "id: example", "id: other", 1),
		"invalid receipt": runMetadata + "last_run: custom-value\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeMetadata(t, root, "example", metadata)
			if err := RecordRun(root, "example", testRunReceipt()); err == nil {
				t.Fatal("accepted invalid metadata")
			}
			data, err := os.ReadFile(filepath.Join(root, "experiments", "example", "expledger.yaml"))
			if err != nil || string(data) != metadata {
				t.Fatalf("invalid metadata changed: data=%q, err=%v", data, err)
			}
		})
	}
}

func TestRecordRunRejectsInvalidReceiptWithoutChangingMetadata(t *testing.T) {
	root := t.TempDir()
	writeMetadata(t, root, "example", runMetadata)
	if err := RecordRun(root, "example", experiment.RunReceipt{}); err == nil {
		t.Fatal("accepted invalid run receipt")
	}
	data, err := os.ReadFile(filepath.Join(root, "experiments", "example", "expledger.yaml"))
	if err != nil || string(data) != runMetadata {
		t.Fatalf("invalid receipt changed metadata: data=%q, err=%v", data, err)
	}
}

func TestRecordRunRejectsMetadataSymlink(t *testing.T) {
	root := t.TempDir()
	writeMetadata(t, root, "example", runMetadata)
	path := filepath.Join(root, "experiments", "example", "expledger.yaml")
	target := filepath.Join(root, "saved.yaml")
	if err := os.Rename(path, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "saved.yaml"), path); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, "example"); err != nil {
		t.Fatalf("ordinary reading should still support contained metadata symlinks: %v", err)
	}
	if err := RecordRun(root, "example", testRunReceipt()); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("RecordRun error=%v, want regular metadata error", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != runMetadata {
		t.Fatalf("symlink target changed: data=%q, err=%v", data, err)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("metadata symlink replaced: info=%v, err=%v", info, err)
	}
}

func TestRecordRunRejectsWrongFolderAndInvalidIDs(t *testing.T) {
	root := t.TempDir()
	writeMetadata(t, root, "Example", runMetadata)
	for _, id := range []string{"example", "../Example", "", ".", "nested/example"} {
		if err := RecordRun(root, id, testRunReceipt()); err == nil {
			t.Fatalf("RecordRun accepted ID %q", id)
		}
	}
}

func testRunReceipt() experiment.RunReceipt {
	return experiment.RunReceipt{ProjectCommit: strings.Repeat("a", 40), StartedAt: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)}
}
