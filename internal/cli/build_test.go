package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/marcfranquesa/expledger/internal/web"
)

func TestBuildSnapshot(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "project"))); err != nil {
		t.Fatal(err)
	}
	sourceProject(t, root, "remote_url: https://github.com/example/expledger-preview/tree/preview/experiments")

	cwd := filepath.Join(root, "nested")
	if err := os.Mkdir(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	options := web.PageOptions{Project: filepath.Base(root), RemoteURL: "https://github.com/example/expledger-preview/tree/preview/experiments"}
	handler := web.NewLiveHandler(func() (web.Snapshot, error) { return loadSnapshot(root, nil) })
	renderStatic := func() []byte {
		t.Helper()
		records, err := catalog.List(root, catalog.Layout{})
		if err != nil {
			t.Fatal(err)
		}
		body, err := web.Render(records, options)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	for _, output := range []string{"", "site/output", filepath.Join(t.TempDir(), "absolute")} {
		args := []string{"build"}
		dir := output
		if output == "" {
			dir = "dist"
		} else {
			args = append(args, "--output", output)
		}
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(cwd, dir)
		}
		var stdout bytes.Buffer
		if err := Run(context.Background(), args, cwd, time.Time{}, Streams{Out: &stdout}); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "index.html")
		if got := stdout.String(); got != path+"\n" {
			t.Fatalf("build output = %q, want %q", got, path+"\n")
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(body, renderStatic()) || bytes.Contains(body, []byte("fetch(")) {
			t.Fatal("build did not produce the offline catalog page")
		}
	}

	path := filepath.Join(cwd, "dist", "index.html")
	keep := filepath.Join(cwd, "dist", "keep.txt")
	if err := os.WriteFile(keep, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(root, "experiments", "20260924-baseline", "experiment.yaml")
	data, err := os.ReadFile(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadata, bytes.Replace(data, []byte("Baseline model"), []byte("Updated baseline"), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Updated baseline") {
		t.Fatal("serve did not refresh the catalog")
	}
	snapshot, err := os.ReadFile(path)
	if err != nil || bytes.Contains(snapshot, []byte("Updated baseline")) {
		t.Fatalf("snapshot changed without a build: %v", err)
	}
	var stdout bytes.Buffer
	if err := Run(context.Background(), []string{"build"}, cwd, time.Time{}, Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(snapshot, renderStatic()) {
		t.Fatalf("rebuild did not update the snapshot: %v", err)
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "keep me" {
		t.Fatalf("build modified another output file: %q, %v", data, err)
	}

	if err := os.WriteFile(metadata, []byte("invalid metadata"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := Run(context.Background(), []string{"build"}, cwd, time.Time{}, Streams{Out: &stdout}); err == nil || !strings.Contains(err.Error(), "experiment.yaml") {
		t.Fatalf("invalid catalog error = %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, snapshot) || stdout.Len() != 0 {
		t.Fatalf("failed build changed the snapshot or printed success: %v", err)
	}
}

func TestBuildEmptyAndOutputErrors(t *testing.T) {
	root := t.TempDir()
	sourceProject(t, root, "{}")

	var stdout bytes.Buffer
	if err := Run(context.Background(), []string{"build"}, root, time.Time{}, Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(root, "dist", "index.html")); err != nil || !bytes.Contains(body, []byte("No experiments yet")) {
		t.Fatalf("empty page missing: %v", err)
	}
	for _, indexIsDirectory := range []bool{false, true} {
		dir := filepath.Join(t.TempDir(), "output")
		want := "create output directory"
		if indexIsDirectory {
			if err := os.MkdirAll(filepath.Join(dir, "index.html"), 0o755); err != nil {
				t.Fatal(err)
			}
			want = "write experiment page"
		} else if err := os.WriteFile(dir, []byte("existing file"), 0o644); err != nil {
			t.Fatal(err)
		}
		stdout.Reset()
		err := Run(context.Background(), []string{"build", "--output", dir}, root, time.Time{}, Streams{Out: &stdout})
		if err == nil || !strings.Contains(err.Error(), want) || stdout.Len() != 0 {
			t.Fatalf("output error = %v, stdout = %q", err, stdout.String())
		}
	}
}

func TestBuildWithoutRemoteLinks(t *testing.T) {
	for _, config := range []string{"{}\n", "remote_url: ''\n"} {
		root := t.TempDir()
		if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "project"))); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "expledger.yaml"), []byte(config), 0644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", t.TempDir())
		if err := Run(context.Background(), []string{"build"}, root, time.Time{}, Streams{}); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(root, "dist", "index.html"))
		if err != nil || !bytes.Contains(body, []byte("Baseline model")) || bytes.Contains(body, []byte(`class="remote-link"`)) {
			t.Fatalf("local snapshot: %v", err)
		}
	}
}

func TestBuildUsesLocalReportsOnly(t *testing.T) {
	base := t.TempDir()
	root, other := filepath.Join(base, "root"), filepath.Join(base, "other")
	sourceProject(t, root, "sources: [../other]\nremote_url: https://local.example/experiments")
	sourceProject(t, other, "remote_url: https://other.example/experiments")
	sourceRecord(t, root, "local", "Local experiment", "")
	sourceRecord(t, other, "other", "Other experiment", "")
	writeTestReport(t, root, "local", "Local findings", "step,loss\n1,5\n")
	sourceWrite(t, filepath.Join(other, "experiments", "other", "report.yaml"), "invalid external report")
	if err := Run(context.Background(), []string{"build"}, root, time.Time{}, Streams{}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "dist", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Local experiment", "Local findings", "https://local.example/experiments/local"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("local build missing %q", want)
		}
	}
	if bytes.Contains(body, []byte("Other experiment")) || bytes.Contains(body, []byte("https://other.example")) {
		t.Fatal("build included a configured external source")
	}
}
