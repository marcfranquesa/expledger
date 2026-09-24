package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
)

func main() {
	cwd, err := os.Getwd()
	if err == nil {
		err = cli.Run(context.Background(), os.Args[1:], cwd, time.Now(), cli.Streams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "expledger:", err)
		os.Exit(cli.ExitCode(err))
	}
}
