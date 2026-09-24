package cli_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
	"github.com/marcfranquesa/expledger/internal/web"
)

func TestBuildSnapshot(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "project"))); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "--quiet", "--initial-branch=research/v2")
	git(t, root, "remote", "add", "origin", "git@github.com:owner/repo.git")
	cwd := filepath.Join(root, "nested")
	if err := os.Mkdir(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	handler := web.NewHandler(root, web.PageOptions{Project: filepath.Base(root), RepositoryURL: "https://github.com/owner/repo", Branch: "research/v2"})
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
		if err := cli.Run(args, cwd, time.Time{}, &stdout); err != nil {
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
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		if response.Code != http.StatusOK || !bytes.Equal(body, response.Body.Bytes()) {
			t.Fatal("build and serve produced different pages")
		}
	}

	path := filepath.Join(cwd, "dist", "index.html")
	keep := filepath.Join(cwd, "dist", "keep.txt")
	if err := os.WriteFile(keep, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(root, "experiments", "20260924-baseline", "README.md")
	data, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readme, bytes.Replace(data, []byte("Baseline model"), []byte("Updated baseline"), 1), 0o644); err != nil {
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
	if err := cli.Run([]string{"build"}, cwd, time.Time{}, &stdout); err != nil {
		t.Fatal(err)
	}
	snapshot, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(snapshot, response.Body.Bytes()) {
		t.Fatalf("rebuild did not update the snapshot: %v", err)
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "keep me" {
		t.Fatalf("build modified another output file: %q, %v", data, err)
	}

	if err := os.WriteFile(readme, []byte("invalid README"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := cli.Run([]string{"build"}, cwd, time.Time{}, &stdout); err == nil || !strings.Contains(err.Error(), "README.md") {
		t.Fatalf("invalid catalog error = %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, snapshot) || stdout.Len() != 0 {
		t.Fatalf("failed build changed the snapshot or printed success: %v", err)
	}
}

func TestBuildEmptyAndOutputErrors(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet", "--initial-branch=main")
	git(t, root, "remote", "add", "origin", "https://github.com/owner/repo.git")
	var stdout bytes.Buffer
	if err := cli.Run([]string{"build"}, root, time.Time{}, &stdout); err != nil {
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
		err := cli.Run([]string{"build", "--output", dir}, root, time.Time{}, &stdout)
		if err == nil || !strings.Contains(err.Error(), want) || stdout.Len() != 0 {
			t.Fatalf("output error = %v, stdout = %q", err, stdout.String())
		}
	}
}

func TestBuildHelpAndArguments(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, args := range [][]string{{"build", "--help"}, {"help", "build"}} {
		var stdout bytes.Buffer
		if err := cli.Run(args, t.TempDir(), time.Time{}, &stdout); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"--output", "dist", "index.html", "snapshot"} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("help missing %q: %s", want, stdout.String())
			}
		}
	}
	for _, args := range [][]string{{"build", "extra"}, {"build", "--output="}, {"build", "--output"}, {"build", "--unknown"}} {
		var stdout bytes.Buffer
		err := cli.Run(args, t.TempDir(), time.Time{}, &stdout)
		if err == nil || strings.Contains(err.Error(), "Git") || stdout.Len() != 0 {
			t.Fatalf("expected argument error before Git lookup for %v, got %v", args, err)
		}
	}
}
