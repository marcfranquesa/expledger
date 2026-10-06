package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
	"github.com/marcfranquesa/expledger/internal/experiment"
)

func TestCustomLayoutCommands(t *testing.T) {
	root := t.TempDir()
	config := "experiments_dir: research/trials\nexperiment_format: '{date}/{slug}'\nremote_url: https://example.com/research/trials\n"
	if err := os.WriteFile(filepath.Join(root, "expledger.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 6, 23, 45, 12, 0, time.FixedZone("local", -4*60*60))
	run := func(args ...string) string {
		t.Helper()
		var out bytes.Buffer
		if err := cli.Run(context.Background(), args, cwd, now, cli.Streams{Out: &out}); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return out.String()
	}
	parent, child := "2026-10-06/baseline", "2026-10-06/child"
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	parentDir := filepath.Join(resolvedRoot, "research", "trials", parent)
	if got := run("new", "baseline"); got != parentDir+"\n" {
		t.Fatalf("created path = %q, want %q", got, parentDir)
	}
	childDir := filepath.Join(resolvedRoot, "research", "trials", child)
	if got := run("new", "child", "--based-on", parent); got != childDir+"\n" {
		t.Fatalf("child path = %q, want %q", got, childDir)
	}
	data, err := os.ReadFile(filepath.Join(childDir, "experiment.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := experiment.Parse(data)
	if err != nil || record.ID != child || len(record.BasedOn) != 1 || record.BasedOn[0] != parent {
		t.Fatalf("child metadata = %+v, %v", record, err)
	}
	if got := run("list"); !strings.Contains(got, parent) || !strings.Contains(got, child) {
		t.Fatalf("list = %q", got)
	}
	if got, want := run("validate", child), "Valid: "+filepath.Join("research", "trials", child, "experiment.yaml")+"\n"; got != want {
		t.Fatalf("validate = %q, want %q", got, want)
	}
	run("build", "--output", "site")
	page, err := os.ReadFile(filepath.Join(cwd, "site", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{parent, child, "https://example.com/research/trials/" + child} {
		if !bytes.Contains(page, []byte(want)) {
			t.Fatalf("built page missing %q", want)
		}
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if err := os.WriteFile(filepath.Join(childDir, "run.sh"), []byte("#!/bin/sh\nprintf '%s\\n' \"$PWD\"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if got := run("run", child); got != childDir+"\n" {
			t.Fatalf("run directory = %q, want %q", got, childDir)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "experiments")); !os.IsNotExist(err) {
		t.Fatalf("default directory unexpectedly used: %v", err)
	}
	var out bytes.Buffer
	err = cli.Run(context.Background(), []string{"new", "child"}, cwd, now, cli.Streams{Out: &out})
	if err == nil || out.Len() != 0 {
		t.Fatalf("duplicate creation: output %q, error %v", &out, err)
	}
}

func TestCustomLayoutDateAndTimePlaceholders(t *testing.T) {
	for _, tt := range []struct {
		format string
		id     string
	}{
		{"{date}/{time}", "2026-10-06/18-17-16"},
		{"{date:20060102}/{time:15:04}-{slug}", "20261006/18:17-trial"},
		{"{date:2006-01-02}/{time:15:04}-{slug}", "2026-10-06/18:17-trial"},
		{"{date}/{time:05:04:15}", "2026-10-06/16:17:18"},
		{"2006-01-02/{slug}", "2006-01-02/trial"},
	} {
		t.Run(tt.format, func(t *testing.T) {
			root := t.TempDir()
			config := "experiment_format: '" + tt.format + "'\n"
			if err := os.WriteFile(filepath.Join(root, "expledger.yaml"), []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 6, 18, 17, 16, 0, time.FixedZone("local", -7*60*60))
			var out bytes.Buffer
			if err := cli.Run(context.Background(), []string{"new", "trial"}, root, now, cli.Streams{Out: &out}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "experiments", tt.id, "experiment.yaml")); err != nil {
				t.Fatal(err)
			}
			if err := cli.Run(context.Background(), []string{"validate", tt.id}, root, now, cli.Streams{}); err != nil {
				t.Fatal(err)
			}
			out.Reset()
			if err := cli.Run(context.Background(), []string{"list"}, root, now, cli.Streams{Out: &out}); err != nil || !strings.Contains(out.String(), tt.id) {
				t.Fatalf("list = %q, %v", &out, err)
			}
		})
	}
}

func TestLayoutConfigRejectsInvalidValues(t *testing.T) {
	for _, config := range []string{
		"experiments_dir: ''", "experiment_format: ''",
		"experiments_dir: null", "experiment_format: null",
		"experiments_dir: 42", "experiment_format: 20060102",
		"experiments_dir: []", "experiment_format: {}",
		"experiments_dir: /tmp/trials", "experiments_dir: ../trials",
		"experiments_dir: .", "experiments_dir: research/../trials",
		"experiments_dir: 'research\\trials'", "experiments_dir: '  '",
		"experiment_format: /{slug}", "experiment_format: ../{slug}",
		"experiment_format: '{date}/../{slug}'", "experiment_format: '{date}\\{slug}'",
		"experiment_format: '{name}'", "experiment_format: '{unknown}'", "experiment_format: '  '",
		"experiment_format: '{date}/experiment.yaml/{slug}'",
		"experiment_format: '{date'", "experiment_format: 'date}'",
		"experiment_format: '{}'", "experiment_format: '{{date}}'",
		"experiment_format: '{slug:2006}'",
		"experiment_format: '{date:}'", "experiment_format: '{time:}'",
		"experiment_format: '{date:{time}}'",
		"experiment_format: '{date:../2006}/{slug}'",
		"experiment_format: '{time:15\\04}/{slug}'",
		"experiments_dir: trials\nexperiments_dir: trials",
		"experiment_format: '{slug}'\nexperiment_format: '{slug}'",
	} {
		t.Run(config, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "expledger.yaml")
			if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"new", "trial"}, {"list"}, {"init"}} {
				var out bytes.Buffer
				err := cli.Run(context.Background(), args, root, time.Now(), cli.Streams{Out: &out})
				if err == nil || !strings.Contains(err.Error(), path) || out.Len() != 0 {
					t.Fatalf("%v accepted invalid config %q: output %q, error %v", args, config, &out, err)
				}
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 {
				t.Fatalf("invalid config caused filesystem changes: %v, %v", entries, err)
			}
		})
	}
}
