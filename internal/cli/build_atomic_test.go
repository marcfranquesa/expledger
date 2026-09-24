package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
)

func TestBuildReplacesLinkedOutput(t *testing.T) {
	for _, link := range []struct {
		name   string
		create func(string, string) error
	}{{"symlink", os.Symlink}, {"hard link", os.Link}} {
		t.Run(link.name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet", "--initial-branch=main")
			git(t, root, "remote", "add", "origin", "https://github.com/owner/repo.git")
			target := filepath.Join(t.TempDir(), "notes.md")
			notes := []byte("Keep these research notes.\n")
			if err := os.WriteFile(target, notes, 0o640); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, "dist")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "index.html")
			if err := link.create(target, path); err != nil {
				t.Fatal(err)
			}
			var stdout bytes.Buffer
			if err := cli.Run(context.Background(), []string{"build"}, root, time.Time{}, cli.Streams{Out: &stdout}); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(target); err != nil || !bytes.Equal(data, notes) {
				t.Fatalf("build changed linked notes: %q, %v", data, err)
			}
			if data, err := os.ReadFile(path); err != nil || !bytes.Contains(data, []byte("No experiments yet")) {
				t.Fatalf("snapshot missing: %q, %v", data, err)
			}
			if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
				t.Fatalf("output was not replaced with a regular file: %v", err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 || entries[0].Name() != "index.html" {
				t.Fatalf("temporary files remain: %v, %v", entries, err)
			}
		})
	}
}

func TestBuildReplacementFailureCleansUp(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet", "--initial-branch=main")
	git(t, root, "remote", "add", "origin", "https://github.com/owner/repo.git")
	dir := filepath.Join(root, "dist")
	index := filepath.Join(dir, "index.html")
	if err := os.MkdirAll(index, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(index, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"build"}, root, time.Time{}, cli.Streams{Out: &stdout}); err == nil || stdout.Len() != 0 {
		t.Fatalf("failed replacement reported success: %v, %q", err, stdout.String())
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep me" {
		t.Fatalf("failed replacement changed existing content: %q, %v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "index.html" {
		t.Fatalf("failed replacement left temporary files: %v, %v", entries, err)
	}
}

func TestBuildPreservesSnapshotPermissions(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet", "--initial-branch=main")
	git(t, root, "remote", "add", "origin", "https://github.com/owner/repo.git")
	dir := filepath.Join(root, "dist")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "index.html")
	if err := os.WriteFile(path, []byte("old snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"build"}, root, time.Time{}, cli.Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("snapshot permissions changed: %v, %v", info, err)
	}
}
