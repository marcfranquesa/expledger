package cli_test

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
		if err := cli.Run(context.Background(), args, cwd, time.Time{}, cli.Streams{Out: &stdout}); err != nil {
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
	metadata := filepath.Join(root, "experiments", "20260924-baseline", "expledger.yaml")
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
	if err := cli.Run(context.Background(), []string{"build"}, cwd, time.Time{}, cli.Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(snapshot, response.Body.Bytes()) {
		t.Fatalf("rebuild did not update the snapshot: %v", err)
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "keep me" {
		t.Fatalf("build modified another output file: %q, %v", data, err)
	}

	if err := os.WriteFile(metadata, []byte("invalid metadata"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := cli.Run(context.Background(), []string{"build"}, cwd, time.Time{}, cli.Streams{Out: &stdout}); err == nil || !strings.Contains(err.Error(), "expledger.yaml") {
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
	if err := cli.Run(context.Background(), []string{"build"}, root, time.Time{}, cli.Streams{Out: &stdout}); err != nil {
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
		err := cli.Run(context.Background(), []string{"build", "--output", dir}, root, time.Time{}, cli.Streams{Out: &stdout})
		if err == nil || !strings.Contains(err.Error(), want) || stdout.Len() != 0 {
			t.Fatalf("output error = %v, stdout = %q", err, stdout.String())
		}
	}
}

func TestBuildWithoutRemoteLinks(t *testing.T) {
	for _, tt := range []struct {
		name, remote string
		detached     bool
	}{
		{name: "no origin"},
		{name: "non-GitHub origin", remote: "https://gitlab.com/owner/repo.git"},
		{name: "detached HEAD", remote: "https://github.com/owner/repo.git", detached: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "project"))); err != nil {
				t.Fatal(err)
			}
			git(t, root, "init", "--quiet", "--initial-branch=main")
			if tt.remote != "" {
				git(t, root, "remote", "add", "origin", tt.remote)
			}
			if tt.detached {
				git(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "--allow-empty", "-m", "initial")
				git(t, root, "checkout", "--quiet", "--detach", "HEAD")
			}
			var stdout bytes.Buffer
			if err := cli.Run(context.Background(), []string{"build"}, root, time.Time{}, cli.Streams{Out: &stdout}); err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(filepath.Join(root, "dist", "index.html"))
			if err != nil || !bytes.Contains(body, []byte("Baseline model")) || bytes.Contains(body, []byte(`class="github"`)) {
				t.Fatalf("local snapshot missing or includes remote links: %v\n%s", err, body)
			}
		})
	}
}
