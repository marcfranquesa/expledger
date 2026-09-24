//go:build !darwin && !linux

package cli

import (
	"context"
	"errors"
	"os/exec"
)

func executeRun(context.Context, *exec.Cmd, func() error) error {
	return errors.New("expledger run requires macOS or Linux")
}
