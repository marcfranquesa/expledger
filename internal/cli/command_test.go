package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
)

func TestCommandConstructionNeedsNoProject(t *testing.T) {
	// A nonexistent cwd would fail discovery if construction or help invoked it.
	cwd := filepath.Join(t.TempDir(), "missing")
	root := cli.NewCommand(cwd, time.Time{})
	var help bytes.Buffer
	root.SetOut(&help)
	if err := root.Help(); err != nil {
		t.Fatal(err)
	}
	for _, command := range root.Commands() {
		if command.Example == "" {
			t.Errorf("%s has no example", command.Name())
		}
		if err := command.Help(); err != nil {
			t.Fatal(err)
		}
	}
	if help.Len() == 0 {
		t.Fatal("empty help")
	}
}
