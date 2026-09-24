// Package cli handles ExpLedger commands and their terminal output.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

const usage = "usage: expledger new <slug>"

func Run(args []string, cwd string, now time.Time, stdout io.Writer) error {
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		_, err := fmt.Fprintln(stdout, usage)
		return err
	}
	if len(args) != 2 || args[0] != "new" {
		return errors.New(usage)
	}

	root, err := gitRoot(cwd)
	if err != nil {
		return err
	}
	dir, err := experiment.Create(root, args[1], now)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, dir)
	return err
}

func gitRoot(cwd string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("find Git working tree (run inside a repository): %w", err)
	}
	return strings.TrimSuffix(strings.TrimSuffix(string(output), "\n"), "\r"), nil
}
