package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

func TestCreate(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 24, 23, 59, 0, 0, time.FixedZone("local", -4*60*60))
	dir, err := Create(root, "my-idea", now, CreateOptions{})
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
	record, err := experiment.Parse(data)
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
	if _, err := Create(root, "my-idea", now, CreateOptions{}); !errors.Is(err, os.ErrExist) {
		t.Fatalf("duplicate creation error = %v, want os.ErrExist", err)
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "existing research notes" {
		t.Fatalf("duplicate attempt changed original: data=%q, err=%v", data, err)
	}
}

func TestCreateRejectsInvalidSlugs(t *testing.T) {
	for _, slug := range []string{"", " \t\n", ".", "..", "../outside", "/absolute", "a/b", `a\b`, "nul\x00slug", "Upper", "a b", "a--b", "a_1", "-a", "a-"} {
		t.Run(slug, func(t *testing.T) {
			root := t.TempDir()
			if _, err := Create(root, slug, time.Now(), CreateOptions{}); err == nil {
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
	if _, err := Create(root, "baseline", time.Now(), CreateOptions{}); err == nil {
		t.Fatal("symlinked experiments directory accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("wrote through symlink: entries=%v, err=%v", entries, err)
	}
}

func TestCreateRejectsOccupiedDestination(t *testing.T) {
	for _, kind := range []string{"empty directory", "file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
			dir := filepath.Join(root, "experiments", "20260924-baseline")
			if err := os.Mkdir(filepath.Dir(dir), 0755); err != nil {
				t.Fatal(err)
			}
			var preservedFile, target string
			switch kind {
			case "empty directory":
				if err := os.Mkdir(dir, 0755); err != nil {
					t.Fatal(err)
				}
			case "file":
				preservedFile = dir
			case "symlink":
				target = t.TempDir()
				preservedFile = filepath.Join(target, "notes.md")
				if err := os.Symlink(target, dir); err != nil {
					t.Fatal(err)
				}
			}
			if preservedFile != "" {
				if err := os.WriteFile(preservedFile, []byte("existing notes"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Create(root, "baseline", now, CreateOptions{}); !errors.Is(err, os.ErrExist) {
				t.Fatalf("occupied destination error = %v, want os.ErrExist", err)
			}
			if kind == "empty directory" {
				if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
					t.Fatalf("occupied directory changed: entries=%v, err=%v", entries, err)
				}
			} else if data, err := os.ReadFile(preservedFile); err != nil || string(data) != "existing notes" {
				t.Fatalf("occupied destination changed notes: data=%q, err=%v", data, err)
			}
			if kind == "symlink" {
				if got, err := os.Readlink(dir); err != nil || got != target {
					t.Fatalf("occupied symlink changed: target=%q, err=%v", got, err)
				}
				if entries, err := os.ReadDir(target); err != nil || len(entries) != 1 || entries[0].Name() != "notes.md" {
					t.Fatalf("wrote through occupied symlink: entries=%v, err=%v", entries, err)
				}
			}
		})
	}
}

func TestCreateRejectsExperimentsFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "experiments")
	if err := os.WriteFile(path, []byte("existing notes"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(root, "baseline", time.Now(), CreateOptions{}); err == nil {
		t.Fatal("experiments file accepted as directory")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "existing notes" {
		t.Fatalf("experiments file changed: data=%q, err=%v", data, err)
	}
}

func TestCreateRequiresProjectDirectory(t *testing.T) {
	parent := t.TempDir()
	if _, err := Create(filepath.Join(parent, "missing"), "baseline", time.Now(), CreateOptions{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing project error = %v, want os.ErrNotExist", err)
	}
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
		t.Fatalf("missing project unexpectedly created files: entries=%v, err=%v", entries, err)
	}
}

func TestCreateStoresParentMetadata(t *testing.T) {
	root := t.TempDir()
	parents := []string{"baseline", "Existing café notes"}
	for _, parent := range parents {
		writeListREADME(t, root, parent, fmt.Sprintf("---\nid: %q\ntitle: Parent\ncreated_at: 2026-09-24T12:00:00Z\n---\n", parent))
	}
	dir, err := Create(root, "improved", time.Now(), CreateOptions{BasedOn: parents})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := experiment.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(record.BasedOn, parents) {
		t.Fatalf("based_on = %v, want %v", record.BasedOn, parents)
	}
}

func TestCreateUsesProvidedLocation(t *testing.T) {
	now := time.Date(2026, 9, 24, 23, 59, 0, 0, time.FixedZone("local", -4*60*60))
	for _, tt := range []struct {
		name string
		now  time.Time
		id   string
	}{
		{name: "local", now: now, id: "20260924-baseline"},
		{name: "UTC", now: now.UTC(), id: "20260925-baseline"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			dir, err := Create(root, "baseline", tt.now, CreateOptions{})
			if err != nil || dir != filepath.Join(root, "experiments", tt.id) {
				t.Fatalf("directory = %q, err=%v; want ID %q", dir, err, tt.id)
			}
			data, err := os.ReadFile(filepath.Join(dir, "README.md"))
			if err != nil {
				t.Fatal(err)
			}
			record, err := experiment.Parse(data)
			if err != nil || record.ID != tt.id {
				t.Fatalf("record ID = %q, err=%v; want %q", record.ID, err, tt.id)
			}
		})
	}
}

func TestCreateRecordsUTCTimestampWithLocalDate(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 24, 23, 59, 0, 123456789, time.FixedZone("local", -4*60*60))
	dir, err := Create(root, "timestamp", now, CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "experiments", "20260924-timestamp"); dir != want {
		t.Fatalf("directory = %q, want %q", dir, want)
	}
	data, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := experiment.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "20260924-timestamp" || !record.CreatedAt.Equal(now) {
		t.Fatalf("creation metadata = %s, %s; want local date ID and %s", record.ID, record.CreatedAt, now)
	}
	if !bytes.Contains(data, []byte("created_at: 2026-09-25T03:59:00.123456789Z\n")) {
		t.Fatalf("README does not contain precise UTC timestamp: %s", data)
	}
}

func TestCreateRejectsMissingParent(t *testing.T) {
	for _, withExistingParent := range []bool{false, true} {
		t.Run(fmt.Sprint(withExistingParent), func(t *testing.T) {
			root := t.TempDir()
			parents := []string{"missing"}
			if withExistingParent {
				writeListREADME(t, root, "baseline", "---\nid: baseline\ntitle: Baseline\ncreated_at: 2026-09-24T12:00:00Z\n---\n")
				parents = []string{"baseline", "missing"}
			}
			if dir, err := Create(root, "child", time.Now(), CreateOptions{BasedOn: parents}); dir != "" || !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), `check parent experiment "missing"`) {
				t.Fatalf("Create = %q, %v; want missing parent error", dir, err)
			}
			checkDir, wantCount := root, 0
			if withExistingParent {
				checkDir, wantCount = filepath.Join(root, "experiments"), 1
			}
			entries, err := os.ReadDir(checkDir)
			if err != nil || len(entries) != wantCount || (wantCount == 1 && entries[0].Name() != "baseline") {
				t.Fatalf("missing parent changed project: entries=%v, err=%v", entries, err)
			}
		})
	}
}

func TestCreateRejectsInvalidParentRecord(t *testing.T) {
	for _, tt := range []struct {
		name   string
		folder string
		readme string
	}{
		{name: "missing README", folder: "baseline"},
		{name: "malformed README", folder: "baseline", readme: "---\nid: [\n---\n"},
		{name: "mismatched ID", folder: "baseline", readme: "---\nid: other\ntitle: Baseline\ncreated_at: 2026-09-24T12:00:00Z\n---\n"},
		{name: "case mismatched folder", folder: "Baseline", readme: "---\nid: baseline\ntitle: Baseline\ncreated_at: 2026-09-24T12:00:00Z\n---\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeListREADME(t, root, tt.folder, tt.readme)
			readme := filepath.Join(root, "experiments", tt.folder, "README.md")
			if tt.readme == "" {
				if err := os.Remove(readme); err != nil {
					t.Fatal(err)
				}
			}
			if dir, err := Create(root, "child", time.Now(), CreateOptions{BasedOn: []string{"baseline"}}); dir != "" || err == nil || !strings.Contains(err.Error(), `check parent experiment "baseline"`) {
				t.Fatalf("Create = %q, %v; want invalid parent error", dir, err)
			}
			entries, err := os.ReadDir(filepath.Join(root, "experiments"))
			if err != nil || len(entries) != 1 || entries[0].Name() != tt.folder {
				t.Fatalf("invalid parent changed project: entries=%v, err=%v", entries, err)
			}
			data, err := os.ReadFile(readme)
			if tt.readme == "" {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("missing parent README created: err=%v", err)
				}
			} else if err != nil || string(data) != tt.readme {
				t.Fatalf("invalid parent README changed: data=%q, err=%v", data, err)
			}
		})
	}
}

func TestCreateChecksOnlyDirectParents(t *testing.T) {
	for _, parents := range [][]string{nil, {"baseline"}} {
		t.Run(fmt.Sprint(parents), func(t *testing.T) {
			root := t.TempDir()
			writeListREADME(t, root, "baseline", "---\nid: baseline\ntitle: Baseline\ncreated_at: 2026-09-24T12:00:00Z\nbased_on: [missing-ancestor]\n---\n")
			writeListREADME(t, root, "unrelated", "not valid front matter\n")
			dir, err := Create(root, "child", time.Now(), CreateOptions{BasedOn: parents})
			if err != nil {
				t.Fatal(err)
			}
			record, err := Read(root, filepath.Base(dir))
			if err != nil || !reflect.DeepEqual(record.BasedOn, parents) {
				t.Fatalf("created record = %+v, %v; want parents %v", record, err, parents)
			}
			data, err := os.ReadFile(filepath.Join(root, "experiments", "unrelated", "README.md"))
			if err != nil || string(data) != "not valid front matter\n" {
				t.Fatalf("creation changed unrelated README: data=%q, err=%v", data, err)
			}
		})
	}
}

func TestCreateRejectsInvalidMetadataBeforeWriting(t *testing.T) {
	for _, opts := range []CreateOptions{
		{Title: " \t"},
		{BasedOn: []string{""}},
		{BasedOn: []string{"../outside"}},
		{BasedOn: []string{`nested\parent`}},
	} {
		t.Run(fmt.Sprint(opts), func(t *testing.T) {
			root := t.TempDir()
			if dir, err := Create(root, "child", time.Now(), opts); dir != "" || err == nil {
				t.Fatalf("Create = %q, %v; want invalid metadata error", dir, err)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid metadata changed project: entries=%v, err=%v", entries, err)
			}
		})
	}
}
