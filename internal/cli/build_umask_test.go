//go:build unix

package cli_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
)

func TestBuildHonorsUmask(t *testing.T) {
	if os.Getenv("EXPLEDGER_TEST_UMASK") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestBuildHonorsUmask$")
		cmd.Env = append(os.Environ(), "EXPLEDGER_TEST_UMASK=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("umask subprocess: %v\n%s", err, output)
		}
		return
	}
	syscall.Umask(0o077)
	root := t.TempDir()
	git(t, root, "init", "--quiet", "--initial-branch=main")
	git(t, root, "remote", "add", "origin", "https://github.com/owner/repo.git")
	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"build"}, root, time.Time{}, &stdout); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, "dist", "index.html"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("umask not respected: %v, %v", info, err)
	}
}
