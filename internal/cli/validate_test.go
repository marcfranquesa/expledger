package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
)

const validateID = "20260924-selected"

const validateMetadata = "schema: expledger/v1\nid: " + validateID + "\ntitle: Selected experiment\ncreated_at: 2026-09-24T12:00:00Z\ncustom: keep-me\n"

func TestValidateFromNestedDirectory(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	cwd := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := writeMetadata(t, root, validateID, []byte(validateMetadata))
	writeMetadata(t, root, "unrelated", []byte("invalid metadata\n"))

	var stdout bytes.Buffer
	if err := cli.Run(context.Background(), []string{"validate", validateID}, cwd, time.Time{}, cli.Streams{Out: &stdout}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "Valid: experiments/"+validateID+"/expledger.yaml\n"; got != want {
		t.Fatalf("validate output = %q, want %q", got, want)
	}
	data, err := os.ReadFile(metadata)
	if err != nil || string(data) != validateMetadata {
		t.Fatalf("validation changed metadata: data=%q, err=%v", data, err)
	}
}

func TestValidateRejectsInvalidMetadataWithoutChangingIt(t *testing.T) {
	for _, tt := range []struct {
		name   string
		data   string
		reason string
	}{
		{name: "missing schema", data: strings.Replace(validateMetadata, "schema: expledger/v1\n", "", 1), reason: "schema"},
		{name: "invalid YAML", data: "schema: expledger/v1\nid: [\n", reason: "metadata"},
		{name: "id mismatch", data: strings.Replace(validateMetadata, validateID, "20260924-other", 1), reason: "20260924-other"},
		{name: "id must match exactly", data: strings.Replace(validateMetadata, validateID, "'"+validateID+" '", 1), reason: "id"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			metadata := writeMetadata(t, root, validateID, []byte(tt.data))
			var stdout bytes.Buffer
			err := cli.Run(context.Background(), []string{"validate", validateID}, root, time.Time{}, cli.Streams{Out: &stdout})
			if err == nil {
				t.Fatal("expected validation error")
			}
			for _, want := range []string{filepath.Join("experiments", validateID, "expledger.yaml"), tt.reason} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("validation error %q does not contain %q", err, want)
				}
			}
			if stdout.Len() != 0 {
				t.Errorf("failed validation printed output: %q", stdout.String())
			}
			data, err := os.ReadFile(metadata)
			if err != nil || string(data) != tt.data {
				t.Fatalf("failed validation changed metadata: data=%q, err=%v", data, err)
			}
		})
	}
}

func TestValidateMissingMetadata(t *testing.T) {
	for _, existingFolder := range []bool{false, true} {
		name := "missing folder"
		if existingFolder {
			name = "missing metadata"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			dir := filepath.Join(root, "experiments", validateID)
			if existingFolder {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			var stdout bytes.Buffer
			err := cli.Run(context.Background(), []string{"validate", validateID}, root, time.Time{}, cli.Streams{Out: &stdout})
			wantPath := filepath.Join("experiments", validateID)
			if existingFolder {
				wantPath = filepath.Join(wantPath, "expledger.yaml")
			}
			if err == nil || !strings.Contains(err.Error(), wantPath) {
				t.Fatalf("expected error naming missing path %s, got %v", wantPath, err)
			}
			if stdout.Len() != 0 {
				t.Errorf("failed validation printed output: %q", stdout.String())
			}
			if existingFolder {
				assertEmptyDirectory(t, dir)
			} else if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatalf("validation created the missing folder; stat error: %v", err)
			}
		})
	}
}
