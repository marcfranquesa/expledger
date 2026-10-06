package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func sourceProject(t *testing.T, root, config string) {
	t.Helper()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	sourceWrite(t, filepath.Join(root, "expledger.yaml"), config)
}
func sourceWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func sourceRecord(t *testing.T, root, id, title, parent string) {
	t.Helper()
	sourceRecordIn(t, root, "experiments", id, title, parent)
}

func sourceRecordIn(t *testing.T, root, experimentsDir, id, title, parent string) {
	t.Helper()
	dir := filepath.Join(root, experimentsDir, id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "schema: expledger/v1\nid: " + id + "\ntitle: " + title + "\ncreated_at: 2026-10-01T12:00:00Z\n"
	if parent != "" {
		content += "based_on: [" + parent + "]\n"
	}
	sourceWrite(t, filepath.Join(dir, "experiment.yaml"), content)
}

func TestServeSourcesUseEachProjectLayout(t *testing.T) {
	base := t.TempDir()
	root, other := filepath.Join(base, "root"), filepath.Join(base, "other")
	sourceProject(t, root, "sources: [., ../other]\nexperiments_dir: local/trials\nexperiment_format: '{date}/{slug}'\nremote_url: https://first.example/local/trials")
	sourceProject(t, other, "experiments_dir: remote/trials\nexperiment_format: '{date}/{time:15:04:05}'\nremote_url: https://second.example/remote/trials")
	now := time.Date(2026, 10, 6, 18, 17, 16, 0, time.UTC)
	for _, source := range []string{root, other} {
		if err := Run(context.Background(), []string{"new", "trial"}, source, now, Streams{}); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := loadServeSnapshot(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Records) != 2 || snapshot.Records[0].ID != "2026-10-06/trial" || snapshot.Records[1].ID != "2026-10-06/18:17:16" {
		t.Fatalf("records = %+v", snapshot.Records)
	}
	if snapshot.Options.RemoteURLs["2026-10-06/trial"] != "https://first.example/local/trials" || snapshot.Options.RemoteURLs["2026-10-06/18:17:16"] != "https://second.example/remote/trials" {
		t.Fatalf("remote URLs = %+v", snapshot.Options.RemoteURLs)
	}
	sourceProject(t, other, "experiments_dir: missing/trials\n")
	snapshot, err = loadServeSnapshot(root, nil)
	if err != nil || len(snapshot.Records) != 1 || snapshot.Records[0].ID != "2026-10-06/trial" {
		t.Fatalf("refreshed layout: %+v, %v", snapshot, err)
	}
	sourceProject(t, other, "experiments_dir: ../trials\n")
	snapshot, err = loadServeSnapshot(root, nil)
	if err == nil || len(snapshot.Records) != 0 {
		t.Fatalf("invalid source layout: %+v, %v", snapshot, err)
	}
}

func TestSourceConfigValidation(t *testing.T) {
	for _, content := range []string{"sources: []", "sources: null", "sources: .", "sources: [1]", "sources: ['']", "sources: ['  ']", "sources: [{}]", "sources: [.]\nsources: [.]", "sources: &paths [.]\nremote_url: *paths"} {
		t.Run(content, func(t *testing.T) {
			root := t.TempDir()
			sourceProject(t, root, content)
			if _, err := readProjectConfig(filepath.Join(root, "expledger.yaml")); err == nil {
				t.Fatalf("accepted %q", content)
			}
		})
	}
	root := t.TempDir()
	sourceProject(t, root, "sources: [., '~/work', '$HOME/work']")
	config, err := readProjectConfig(filepath.Join(root, "expledger.yaml"))
	if err != nil || !reflect.DeepEqual(config.Sources, []string{".", "~/work", "$HOME/work"}) {
		t.Fatalf("%+v, %v", config, err)
	}
}

func TestServeSourcesFirstWinnerAndRefresh(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	other := filepath.Join(base, "other")
	sourceProject(t, root, "sources: [., ../other]\nremote_url: https://first.example/experiments")
	sourceProject(t, other, "sources: [../missing]\nremote_url: https://second.example/experiments")
	sourceRecord(t, root, "same", "First", "first-parent")
	sourceRecord(t, other, "same", "Second", "second-parent")
	sourceRecord(t, other, "unique", "Other", "")
	snapshot, err := loadServeSnapshot(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Records) != 2 || snapshot.Records[0].Title != "First" || !reflect.DeepEqual(snapshot.Records[0].BasedOn, []string{"first-parent"}) || snapshot.Options.RemoteURLs["same"] != "https://first.example/experiments" {
		t.Fatalf("snapshot: %+v", snapshot)
	}
	sourceProject(t, root, "sources: [../other, .]")
	snapshot, err = loadServeSnapshot(root, nil)
	if err != nil || snapshot.Records[0].Title != "Second" || snapshot.Options.RemoteURLs["same"] != "https://second.example/experiments" {
		t.Fatalf("reordered: %+v, %v", snapshot, err)
	}
	if err := os.RemoveAll(filepath.Join(other, "experiments", "same")); err != nil {
		t.Fatal(err)
	}
	snapshot, err = loadServeSnapshot(root, nil)
	if err != nil || len(snapshot.Records) != 2 || snapshot.Records[1].Title != "First" || snapshot.Options.RemoteURLs["same"] != "" {
		t.Fatalf("fallback: %+v, %v", snapshot, err)
	}
	sourceRecord(t, other, "same", "Edited", "")
	snapshot, err = loadServeSnapshot(root, nil)
	if err != nil || snapshot.Records[0].Title != "Edited" {
		t.Fatalf("edited: %+v, %v", snapshot, err)
	}
	sourceWrite(t, filepath.Join(root, "experiments", "same", "experiment.yaml"), "invalid")
	snapshot, err = loadServeSnapshot(root, nil)
	if err == nil || len(snapshot.Records) != 0 {
		t.Fatalf("invalid losing duplicate must fail whole snapshot: %+v, %v", snapshot, err)
	}
}

func TestServeSourcesDefaultOverrideAndLiteralPaths(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	other := filepath.Join(base, "other")
	sourceProject(t, root, "{}")
	sourceProject(t, other, "{}")
	sourceRecord(t, root, "local", "Local", "")
	sourceRecord(t, other, "remote", "Remote", "")
	snapshot, err := loadServeSnapshot(root, nil)
	if err != nil || len(snapshot.Records) != 1 || snapshot.Records[0].ID != "local" {
		t.Fatalf("default: %+v, %v", snapshot, err)
	}
	nested := filepath.Join(root, "nested", "deep")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	discovered, _, err := discoverProject(nested)
	if err != nil {
		t.Fatal(err)
	}
	for _, override := range [][]string{{"../other"}, {other}, {"../other", "."}} {
		snapshot, err = loadServeSnapshot(discovered, override)
		if err != nil || snapshot.Records[0].ID != "remote" || len(snapshot.Records) != len(override) {
			t.Fatalf("override %v: %+v, %v", override, snapshot, err)
		}
	}
	sourceProject(t, root, "sources: [../missing]\nremote_url: https://updated.example/experiments")
	snapshot, err = loadServeSnapshot(root, []string{"."})
	if err != nil || snapshot.Options.RemoteURLs["local"] != "https://updated.example/experiments" {
		t.Fatalf("override reread: %+v, %v", snapshot, err)
	}
	for _, literal := range []string{"~/literal", "$HOME/literal"} {
		sourceProject(t, filepath.Join(root, literal), "{}")
		sourceRecord(t, filepath.Join(root, literal), "literal", "Literal", "")
		snapshot, err = loadServeSnapshot(root, []string{literal})
		if err != nil || len(snapshot.Records) != 1 || snapshot.Records[0].ID != "literal" {
			t.Fatalf("literal %s: %+v, %v", literal, snapshot, err)
		}
	}
	if _, err := loadServeSnapshot(root, []string{}); err == nil {
		t.Fatal("accepted explicit empty override")
	}
	for _, invalid := range []string{"", " ", "../missing", "nested"} {
		snapshot, err = loadServeSnapshot(root, []string{invalid})
		if err == nil || len(snapshot.Records) != 0 {
			t.Fatalf("accepted invalid %q: %+v", invalid, snapshot)
		}
	}
	sourceWrite(t, filepath.Join(other, "expledger.yaml"), "unknown: true")
	if _, err = loadServeSnapshot(root, []string{other}); err == nil || !strings.Contains(err.Error(), other) {
		t.Fatalf("invalid source config: %v", err)
	}
}

func TestSourcesDoNotChangeLocalCommands(t *testing.T) {
	root := t.TempDir()
	sourceProject(t, root, "sources: [../does-not-exist]")
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	if err := Run(context.Background(), []string{"new", "local"}, root, now, Streams{}); err != nil {
		t.Fatal(err)
	}
	id := "20261003-local"
	if _, err := os.Stat(filepath.Join(root, "experiments", id, "experiment.yaml")); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Run(context.Background(), []string{"list"}, root, now, Streams{Out: &out}); err != nil || !strings.Contains(out.String(), id) {
		t.Fatalf("local list: %s, %v", &out, err)
	}
	if err := Run(context.Background(), []string{"build", "--output", filepath.Join(root, "offline.html")}, root, now, Streams{}); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return
	}
	sourceWrite(t, filepath.Join(root, "experiments", id, "run.sh"), "#!/bin/sh\necho local-only\n")
	if err := os.Chmod(filepath.Join(root, "experiments", id, "run.sh"), 0755); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(context.Background(), []string{"run", id}, root, now, Streams{Out: &out}); err != nil || out.String() != "local-only\n" {
		t.Fatalf("local run: %s, %v", &out, err)
	}
}

func TestServeRejectsInvalidSourceBeforeListening(t *testing.T) {
	root := t.TempDir()
	sourceProject(t, root, "{}")
	for _, args := range [][]string{{"serve", "--source=", "--port=0"}, {"serve", "--source= ", "--port=0"}, {"serve", "--source=missing", "--port=0"}, {"serve", "--port=0"}} {
		sourceProject(t, root, "sources: [missing]")
		var out bytes.Buffer
		err := Run(context.Background(), args, root, time.Time{}, Streams{Out: &out})
		if err == nil || out.Len() != 0 {
			t.Fatalf("args %v: output %q, error %v", args, out.String(), err)
		}
	}
}

func TestServeSourceFilesystemFailures(t *testing.T) {
	for _, kind := range []string{"unreadable config", "symlinked experiments", "escaping metadata"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			other := t.TempDir()
			sourceProject(t, root, "{}")
			sourceProject(t, other, "{}")
			switch kind {
			case "unreadable config":
				path := filepath.Join(other, "expledger.yaml")
				if err := os.Chmod(path, 0000); err != nil {
					t.Fatal(err)
				}
				if _, err := os.ReadFile(path); err == nil {
					t.Skip("user can read mode 0000")
				}
			case "symlinked experiments":
				if err := os.Symlink(root, filepath.Join(other, "experiments")); err != nil {
					t.Fatal(err)
				}
			case "escaping metadata":
				sourceRecord(t, root, "outside", "Outside", "")
				dir := filepath.Join(other, "experiments", "outside")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, "experiments", "outside", "experiment.yaml"), filepath.Join(dir, "experiment.yaml")); err != nil {
					t.Fatal(err)
				}
			}
			snapshot, err := loadServeSnapshot(root, []string{".", other})
			if err == nil || len(snapshot.Records) != 0 {
				t.Fatalf("accepted %s: %+v, %v", kind, snapshot, err)
			}
		})
	}
}

func TestServeSourcesSortByWinningTimestamp(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	sourceProject(t, root, "{}")
	sourceProject(t, other, "{}")
	sourceRecord(t, root, "same", "First copy", "")
	sourceRecord(t, other, "same", "Newer losing copy", "")
	sourceRecord(t, other, "unique", "Newer unique", "")
	for id, date := range map[string]string{"same": "2026-10-03", "unique": "2026-10-02"} {
		path := filepath.Join(other, "experiments", id, "experiment.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sourceWrite(t, path, strings.ReplaceAll(string(data), "2026-10-01", date))
	}
	snapshot, err := loadServeSnapshot(root, []string{".", other})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Records) != 2 || snapshot.Records[0].ID != "unique" || snapshot.Records[1].Title != "First copy" {
		t.Fatalf("want newer unique before older winning copy: %+v", snapshot.Records)
	}
}
