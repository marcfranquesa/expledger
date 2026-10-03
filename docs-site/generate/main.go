// Command generate builds the CLI reference from the executable's command tree.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
	"github.com/spf13/cobra/doc"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	if _, err := os.Stat("hugo.toml"); err != nil {
		return fmt.Errorf("run the generator from docs-site (or use scripts/docs.sh): %w", err)
	}
	const output = "generated/reference"
	if err := os.RemoveAll(output); err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	root := cli.NewCommand("", time.Time{})
	root.DisableAutoGenTag = true
	if err := doc.GenMarkdownTreeCustom(root, output, func(path string) string {
		name := strings.ReplaceAll(strings.TrimSuffix(filepath.Base(path), ".md"), "_", " ")
		return fmt.Sprintf("---\ntitle: %q\n---\n\n", name)
	}, func(link string) string {
		return link
	}); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, "_index.md"), []byte(`---
title: CLI reference
weight: 50
---

# CLI reference

Generated from ExpLedger's Cobra commands on every documentation build.
Start with [expledger](expledger.md), or choose a command in the navigation.
`), 0o644)
}
