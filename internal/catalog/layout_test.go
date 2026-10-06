package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLayoutDefaults(t *testing.T) {
	got := (Layout{}).WithDefaults()
	want := Layout{ExperimentsDir: "experiments", ExperimentFormat: "20060102-{name}"}
	if got != want {
		t.Fatalf("default layout = %+v, want %+v", got, want)
	}
	custom := Layout{ExperimentsDir: "research/trials", ExperimentFormat: "2006-01-02/{name}"}
	if got := custom.WithDefaults(); got != custom {
		t.Fatalf("custom layout = %+v, want %+v", got, custom)
	}
}

func TestLayoutValidation(t *testing.T) {
	for _, layout := range []Layout{
		{},
		{ExperimentsDir: "research/trials", ExperimentFormat: "2006-01-02/{name}"},
		{ExperimentFormat: "2006-01-02/05:04:15-{name}"},
		{ExperimentFormat: "2006-01-02/15:04:05"},
		{ExperimentFormat: "{name}"},
	} {
		if err := layout.Validate(); err != nil {
			t.Errorf("valid layout %+v: %v", layout, err)
		}
	}
	for _, path := range []string{".", "..", "/absolute", "../outside", "nested/../outside", "nested/./trials", "nested//trials", "nested/", `nested\trials`, "nul\x00path", " \t", "nested/ \t/trials"} {
		for _, field := range []string{"directory", "format"} {
			t.Run(field+"/"+path, func(t *testing.T) {
				layout := Layout{ExperimentsDir: path}
				if field == "format" {
					layout = Layout{ExperimentFormat: path}
				}
				if err := layout.Validate(); err == nil {
					t.Fatalf("invalid layout accepted: %+v", layout)
				}
				root := t.TempDir()
				if _, err := Create(root, "baseline", time.Now(), CreateOptions{Layout: layout}); err == nil {
					t.Fatal("Create accepted invalid layout")
				}
				if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
					t.Fatalf("invalid layout changed project: entries=%v, err=%v", entries, err)
				}
			})
		}
	}
	for _, format := range []string{"{slug}", "{unknown}/{name}", "2006-{name", "2006-name}", "{{name}}"} {
		if err := (Layout{ExperimentFormat: format}).Validate(); err == nil {
			t.Errorf("invalid placeholder format %q accepted", format)
		}
	}
}

func TestCreateRejectsSidecarDirectoryNamesBeforeWriting(t *testing.T) {
	for _, format := range []string{"2006/experiment.yaml/{name}", "2006/Experiment.yaml", "2006/{name}.yaml/child"} {
		t.Run(format, func(t *testing.T) {
			root := t.TempDir()
			layout := Layout{ExperimentFormat: format}
			if _, err := Create(root, "experiment", time.Now(), CreateOptions{Layout: layout}); err == nil {
				t.Fatal("Create accepted a grouping directory named like experiment metadata")
			}
			if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
				t.Fatalf("rejected creation changed project: entries=%v, err=%v", entries, err)
			}
		})
	}
}

func TestCreateConfiguredNestedLayout(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 24, 23, 59, 58, 123456789, time.FixedZone("local", -4*60*60))
	layout := Layout{ExperimentsDir: "research/trials", ExperimentFormat: "2006-01-02/{name}"}
	dir, err := Create(root, "baseline", now, CreateOptions{Layout: layout})
	if want := filepath.Join(root, "research", "trials", "2026-09-24", "baseline"); err != nil || dir != want {
		t.Fatalf("Create = %q, %v; want %q", dir, err, want)
	}
	record, err := Read(root, "2026-09-24/baseline", layout)
	if err != nil || record.ID != "2026-09-24/baseline" || record.Title != "Baseline" || !record.CreatedAt.Equal(now) || record.CreatedAt.Location() != time.UTC {
		t.Fatalf("Read = %+v, %v; want path ID and precise UTC timestamp", record, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "experiment.yaml"))
	if err != nil || !strings.Contains(string(data), "created_at: 2026-09-25T03:59:58.123456789Z\n") {
		t.Fatalf("metadata = %q, %v; want precise UTC timestamp", data, err)
	}
	listed, err := List(root, layout)
	if err != nil || len(listed) != 1 || !reflect.DeepEqual(listed[0], record) {
		t.Fatalf("List = %+v, %v; want created record", listed, err)
	}
	if _, err := os.Stat(filepath.Join(root, "experiments")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected default experiments directory: %v", err)
	}
	_, err = Create(root, "variant", now.Add(time.Second), CreateOptions{Layout: layout, BasedOn: []string{record.ID}})
	if err != nil {
		t.Fatal(err)
	}
	child, err := Read(root, "2026-09-24/variant", layout)
	if err != nil || !reflect.DeepEqual(child.BasedOn, []string{record.ID}) {
		t.Fatalf("child = %+v, %v; want nested parent ID", child, err)
	}
}

func TestCreateTimeFormats(t *testing.T) {
	now := time.Date(2026, 9, 24, 13, 14, 15, 0, time.UTC)
	for _, tt := range []struct{ format, slug, id string }{
		{"2006-01-02/05:04:15-{name}", "baseline", "2026-09-24/15:14:13-baseline"},
		{"2006-01-02/15:04:05", "baseline", "2026-09-24/13:14:15"},
		{"2006-01-02/{name}", "2006-15-04-05", "2026-09-24/2006-15-04-05"},
	} {
		t.Run(tt.format, func(t *testing.T) {
			root := t.TempDir()
			layout := Layout{ExperimentFormat: tt.format}
			dir, err := Create(root, tt.slug, now, CreateOptions{Layout: layout})
			if want := filepath.Join(root, "experiments", filepath.FromSlash(tt.id)); err != nil || dir != want {
				t.Fatalf("Create = %q, %v; want %q", dir, err, want)
			}
			if record, err := Read(root, tt.id, layout); err != nil || record.ID != tt.id {
				t.Fatalf("Read = %+v, %v; want ID %q", record, err, tt.id)
			}
			if !strings.Contains(tt.format, "{name}") {
				if _, err := Create(root, "different-slug", now, CreateOptions{Layout: layout}); !errors.Is(err, os.ErrExist) {
					t.Fatalf("timestamp collision = %v; want os.ErrExist", err)
				}
			}
		})
	}
}

func TestConfiguredMissingDirectoryIsEmpty(t *testing.T) {
	root := t.TempDir()
	if records, err := List(root, Layout{ExperimentsDir: "research/trials"}); err != nil || len(records) != 0 {
		t.Fatalf("List = %+v, %v; want empty catalog", records, err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("List changed project: entries=%v, err=%v", entries, err)
	}
}

func TestNestedCreationPreservesOccupiedDestinations(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	layout := Layout{ExperimentFormat: "2006-01-02/{name}"}
	dir, err := Create(root, "baseline", now, CreateOptions{Layout: layout})
	if err != nil {
		t.Fatal(err)
	}
	notes := filepath.Join(dir, "README.md")
	if err := os.WriteFile(notes, []byte("existing research"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(root, "baseline", now, CreateOptions{Layout: layout}); !errors.Is(err, os.ErrExist) {
		t.Fatalf("duplicate creation = %v; want os.ErrExist", err)
	}
	if _, err := Create(root, "variant", now, CreateOptions{Layout: layout}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(notes); err != nil || string(data) != "existing research" {
		t.Fatalf("existing notes changed: %q, %v", data, err)
	}
}

func TestNestedIDsRejectNoncanonicalPaths(t *testing.T) {
	for _, id := range []string{"/absolute", "a/../b", "a/./b", "a//b", "a/", "/a", "a/ \t/b", "a/nul\x00id", `a\b/c`} {
		if _, err := Read(t.TempDir(), id, Layout{}); err == nil || !strings.Contains(err.Error(), "invalid experiment ID") {
			t.Errorf("Read(%q) = %v; want invalid experiment ID", id, err)
		}
	}
}

func TestNestedDiscoveryStopsAtExperiment(t *testing.T) {
	root := t.TempDir()
	writeMetadata(t, root, "2026-09-24/baseline", "schema: expledger/v1\nid: 2026-09-24/baseline\ntitle: Baseline\ncreated_at: 2026-09-24T12:00:00Z\n")
	writeMetadata(t, root, "2026-09-24/baseline/artifacts", "not experiment metadata")
	writeMetadata(t, root, "2026-09-25/variant", "schema: expledger/v1\nid: 2026-09-25/variant\ntitle: Variant\ncreated_at: 2026-09-25T12:00:00Z\n")
	records, err := List(root, Layout{})
	if err != nil || len(records) != 2 || records[0].ID != "2026-09-25/variant" || records[1].ID != "2026-09-24/baseline" {
		t.Fatalf("List = %+v, %v; want nested records without artifacts", records, err)
	}
}

func TestNestedExperimentCannotBeInsideExperiment(t *testing.T) {
	root := t.TempDir()
	writeMetadata(t, root, "parent", "schema: expledger/v1\nid: parent\ntitle: Parent\ncreated_at: 2026-09-24T12:00:00Z\n")
	writeMetadata(t, root, "parent/child", "schema: expledger/v1\nid: parent/child\ntitle: Child\ncreated_at: 2026-09-24T13:00:00Z\n")
	if _, err := Read(root, "parent/child", Layout{}); err == nil {
		t.Fatal("Read accepted experiment hidden under another experiment")
	}
	layout := Layout{ExperimentFormat: "parent/{name}"}
	if _, err := Create(root, "new-child", time.Now(), CreateOptions{Layout: layout}); err == nil {
		t.Fatal("Create accepted experiment hidden under another experiment")
	}
	if _, err := os.Stat(filepath.Join(root, "experiments", "parent", "new-child")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed creation wrote child directory: %v", err)
	}
}

func TestConfiguredDirectoryRejectsSymlinkAncestors(t *testing.T) {
	for _, path := range []string{"research", "research/trials"} {
		t.Run(path, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			target := filepath.Join(root, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, target); err != nil {
				t.Fatal(err)
			}
			layout := Layout{ExperimentsDir: "research/trials", ExperimentFormat: "2006-01-02/{name}"}
			if _, err := List(root, layout); err == nil {
				t.Fatal("List accepted symlink in experiments directory")
			}
			if _, err := Read(root, "2026-09-24/baseline", layout); err == nil {
				t.Fatal("Read accepted symlink in experiments directory")
			}
			if _, err := Create(root, "baseline", time.Now(), CreateOptions{Layout: layout}); err == nil {
				t.Fatal("Create accepted symlink in experiments directory")
			}
			if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
				t.Fatalf("wrote through symlink: entries=%v, err=%v", entries, err)
			}
		})
	}
}

func TestGroupingSymlinkIsNotTraversed(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	writeMetadata(t, root, "baseline", "schema: expledger/v1\nid: baseline\ntitle: Baseline\ncreated_at: 2026-09-24T12:00:00Z\n")
	if err := os.Symlink(outside, filepath.Join(root, "experiments", "group")); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, "group/child", Layout{}); err == nil {
		t.Fatal("Read accepted symlink grouping directory")
	}
	if _, err := Create(root, "child", time.Now(), CreateOptions{Layout: Layout{ExperimentFormat: "group/{name}"}}); err == nil {
		t.Fatal("Create accepted symlink grouping directory")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("wrote through grouping symlink: entries=%v, err=%v", entries, err)
	}
	records, err := List(root, Layout{})
	if err != nil || len(records) != 1 || records[0].ID != "baseline" {
		t.Fatalf("List = %+v, %v; want baseline and ignored grouping symlink", records, err)
	}
}

func TestNestedPathsRequireExactCase(t *testing.T) {
	root := t.TempDir()
	writeMetadata(t, root, "Group/baseline", "schema: expledger/v1\nid: Group/baseline\ntitle: Baseline\ncreated_at: 2026-09-24T12:00:00Z\n")
	if _, err := Read(root, "group/baseline", Layout{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Read accepted differently-cased ancestor: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "experiments", "group")); errors.Is(err, os.ErrNotExist) {
		t.Skip("filesystem has case-sensitive directory lookup")
	}
	if _, err := Create(root, "child", time.Now(), CreateOptions{Layout: Layout{ExperimentFormat: "group/{name}"}}); err == nil {
		t.Fatal("Create accepted differently-cased existing ancestor")
	}
	if entries, err := os.ReadDir(filepath.Join(root, "experiments", "Group")); err != nil || len(entries) != 1 || entries[0].Name() != "baseline" {
		t.Fatalf("failed creation changed existing group: entries=%v, err=%v", entries, err)
	}
}

func TestConfiguredLayoutMissingParentDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	layout := Layout{ExperimentsDir: "research/trials", ExperimentFormat: "2006-01-02/{name}"}
	if _, err := Create(root, "child", time.Now(), CreateOptions{Layout: layout, BasedOn: []string{"2026-09-24/missing"}}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Create = %v; want missing parent error", err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("missing parent changed project: entries=%v, err=%v", entries, err)
	}
}

func TestNestedIDsPreserveHumanDirectoryNames(t *testing.T) {
	root := t.TempDir()
	const id = "Existing notes/café-α"
	writeMetadata(t, root, id, fmt.Sprintf("schema: expledger/v1\nid: %q\ntitle: Notes\ncreated_at: 2026-09-24T12:00:00Z\n", id))
	record, err := Read(root, id, Layout{})
	if err != nil || record.ID != id {
		t.Fatalf("Read = %+v, %v; want human directory path", record, err)
	}
	listed, err := List(root, Layout{})
	if err != nil || len(listed) != 1 || !reflect.DeepEqual(listed[0], record) {
		t.Fatalf("List = %+v, %v; want human directory path", listed, err)
	}
}
