//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package report

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/catalog"
)

func TestReadRejectsFIFOWithoutBlocking(t *testing.T) {
	for _, name := range []string{"report.yaml", "results.csv"} {
		t.Run(name, func(t *testing.T) {
			manifest := lineManifest("results.csv")
			if name == "report.yaml" {
				manifest = ""
			}
			root, dir := reportFixture(t, manifest, nil)
			if err := syscall.Mkfifo(filepath.Join(dir, name), 0o600); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestReadFIFOHelper$")
			cmd.Env = append(os.Environ(), "EXPLEDGER_REPORT_FIFO_ROOT="+root)
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatal("Read blocked opening FIFO; inspect file type before opening")
			}
			if err != nil {
				t.Fatalf("FIFO helper failed: %v\n%s", err, output)
			}
		})
	}
}

func TestReadFIFOHelper(t *testing.T) {
	root := os.Getenv("EXPLEDGER_REPORT_FIFO_ROOT")
	if root == "" {
		t.Skip("subprocess helper")
	}
	if _, err := Read(root, "example", catalog.Layout{}); err == nil {
		t.Fatal("Read accepted FIFO")
	}
}
