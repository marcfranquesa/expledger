package cli

import (
	"errors"
	"fmt"
	"io"
)

const usage = "usage: expledger new <slug>"

func runHelp(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New(usage)
	}
	_, err := fmt.Fprintln(stdout, usage)
	return err
}
