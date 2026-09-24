//go:build !darwin && !linux

package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

func lockRun(*os.Root, string) (func() error, error) {
	return nil, errors.New("expledger run requires macOS or Linux")
}

func executeRun(context.Context, *exec.Cmd, func() error) (bool, error) {
	return true, errors.New("expledger run requires macOS or Linux")
}
