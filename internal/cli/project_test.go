package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
)

func TestInitPreservesExistingProject(t *testing.T) {
	for _, remote := range []string{"", "https://example.com/team/experiments/"} {
		root := t.TempDir()
		t.Setenv("PATH", t.TempDir())
		args := []string{"init"}
		if remote != "" {
			args = append(args, "--remote-url", remote)
		}
		if err := cli.Run(context.Background(), args, root, time.Time{}, cli.Streams{}); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "expledger.yaml")
		initial, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if remote == "" && string(initial) != "{}\n" {
			t.Fatalf("local config = %q", initial)
		}
		if _, err := os.Stat(filepath.Join(root, "experiments")); !os.IsNotExist(err) {
			t.Fatalf("init created experiments: %v", err)
		}
		original := append([]byte("# keep formatting\n"), initial...)
		if err := os.WriteFile(path, original, 0644); err != nil {
			t.Fatal(err)
		}
		assertNew(t, root, root)
		for _, repeat := range [][]string{args, {"init"}, {"init", "--remote-url", remote}} {
			if err := cli.Run(context.Background(), repeat, root, time.Time{}, cli.Streams{}); err != nil {
				t.Fatal(err)
			}
		}
		err = cli.Run(context.Background(), []string{"init", "--remote-url", "https://other.example/experiments"}, root, time.Time{}, cli.Streams{})
		if err == nil || !strings.Contains(err.Error(), "edit") {
			t.Fatalf("conflict = %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, original) {
			t.Fatalf("init overwrote config: %q, %v", got, err)
		}
		if _, err := os.Stat(filepath.Join(root, "experiments", "20260924-my-idea", "experiment.yaml")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNearestProjectAndExplicitNestedInit(t *testing.T) {
	root := t.TempDir()
	initProject(t, root)
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	initProject(t, nested)
	cwd := filepath.Join(nested, "deeper")
	if err := os.Mkdir(cwd, 0755); err != nil {
		t.Fatal(err)
	}
	assertNew(t, cwd, nested)
	if _, err := os.Stat(filepath.Join(root, "experiments")); !os.IsNotExist(err) {
		t.Fatalf("used outer root: %v", err)
	}
}

func TestNearestInvalidConfigNeverFallsThrough(t *testing.T) {
	for _, data := range []string{
		"", "[]\n", "null\n", "remote_url: [\n", "{}\n---\n{}\n",
		"unknown: true\n", "remote_url: 42\n", "remote_url: null\n", "remote_url: ''\nremote_url: ''\n",
		"remote_url: &url https://example.com\nother: *url\n", "<<: {}\n",
		"schema: expledger/v1\nid: old\ntitle: Old\ncreated_at: 2026-09-24T12:00:00Z\n",
	} {
		t.Run(data, func(t *testing.T) {
			root := t.TempDir()
			initProject(t, root)
			nested := filepath.Join(root, "experiments", "old")
			if err := os.MkdirAll(nested, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(nested, "expledger.yaml")
			if err := os.WriteFile(path, []byte(data), 0644); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"list"}, {"init"}} {
				err := cli.Run(context.Background(), args, nested, time.Time{}, cli.Streams{})
				if err == nil || !strings.Contains(err.Error(), path) {
					t.Fatalf("%v: %v", args, err)
				}
				if data == "" && !strings.Contains(err.Error(), "use {}") {
					t.Fatalf("missing empty-config guidance: %v", err)
				}
				if strings.Contains(data, "schema: expledger/v1") && (!strings.Contains(err.Error(), "rename") || !strings.Contains(err.Error(), "experiment.yaml")) {
					t.Fatalf("missing migration guidance: %v", err)
				}
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != data {
				t.Fatal("invalid config was changed")
			}
		})
	}
}

func TestUnreadableNearestMarker(t *testing.T) {
	for _, kind := range []string{"directory", "dangling link", "permissions"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			initProject(t, root)
			nested := filepath.Join(root, "nested")
			if err := os.Mkdir(nested, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(nested, "expledger.yaml")
			var err error
			switch kind {
			case "directory":
				err = os.Mkdir(path, 0755)
			case "dangling link":
				err = os.Symlink("missing", path)
			case "permissions":
				err = os.WriteFile(path, []byte("{}\n"), 0000)
				if _, readErr := os.ReadFile(path); readErr == nil {
					t.Skip("user can read mode 0000")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"list"}, {"init"}} {
				if err := cli.Run(context.Background(), args, nested, time.Time{}, cli.Streams{}); err == nil {
					t.Fatalf("%v accepted %s", args, kind)
				}
			}
		})
	}
}

func TestRemoteURLValidation(t *testing.T) {
	for _, remote := range []string{"javascript:alert(1)", "file:///tmp/experiments", "//example.com/experiments", "https:///experiments", "https://:80/path", "https://./path", "https://bad..host/path", "https://example.com:99999/path", "https://example.com/path?x=1", "https://example.com/path?", "https://example.com/path#fragment", "https://example.com/path#", "https://user:secret@example.com/path", "https://example.com/\\evil"} {
		t.Run(remote, func(t *testing.T) {
			root := t.TempDir()
			err := cli.Run(context.Background(), []string{"init", "--remote-url", remote}, root, time.Time{}, cli.Streams{})
			if err == nil {
				t.Fatalf("accepted %q", remote)
			}
			assertEmptyDirectory(t, root)
			if err := os.WriteFile(filepath.Join(root, "expledger.yaml"), []byte("remote_url: '"+remote+"'\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := cli.Run(context.Background(), []string{"list"}, root, time.Time{}, cli.Streams{}); err == nil {
				t.Fatal("accepted configured unsafe URL")
			}
		})
	}
	for _, remote := range []string{"https://example.com/team/experiments/", "https://gitlab.com/team/repo/-/tree/topic/experiments", "http://localhost:8080/experiments", "http://[::1]:8080/experiments", "https://example.com/space%20name/experiments"} {
		root := t.TempDir()
		if err := cli.Run(context.Background(), []string{"init", "--remote-url", remote}, root, time.Time{}, cli.Streams{}); err != nil {
			t.Fatalf("%s: %v", remote, err)
		}
		if err := cli.Run(context.Background(), []string{"list"}, root, time.Time{}, cli.Streams{}); err != nil {
			t.Fatal(err)
		}
	}
}
