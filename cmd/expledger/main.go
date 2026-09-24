package main

import (
	"fmt"
	"os"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
)

func main() {
	cwd, err := os.Getwd()
	if err == nil {
		err = cli.Run(os.Args[1:], cwd, time.Now(), os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "expledger:", err)
		os.Exit(1)
	}
}
