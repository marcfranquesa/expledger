// Package cli handles ExpLedger commands and their terminal output.
package cli

import (
	"errors"
	"io"
	"time"
)

func Run(args []string, cwd string, now time.Time, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "help", "--help", "-h":
		return runHelp(args[1:], stdout)
	case "new":
		return runNew(args[1:], cwd, now, stdout)
	default:
		return errors.New(usage)
	}
}
