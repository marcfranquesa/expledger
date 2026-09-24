package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/cli"
)

const validateID = "20260924-selected"

const validateReadme = "---\nid: " + validateID + "\ntitle: Selected experiment\ncreated_at: 2026-09-24T12:00:00Z\ncustom: keep-me\n---\n\nHuman notes with trailing spaces.  \n"

func TestValidateFromNestedDirectory(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	cwd := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := writeREADME(t, root, validateID, []byte(validateReadme))
	writeREADME(t, root, "unrelated", []byte("not valid front matter\n"))

	var stdout bytes.Buffer
	if err := cli.Run([]string{"validate", validateID}, cwd, time.Time{}, &stdout); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "Valid: experiments/"+validateID+"/README.md\n"; got != want {
		t.Fatalf("validate output = %q, want %q", got, want)
	}
	data, err := os.ReadFile(readme)
	if err != nil || string(data) != validateReadme {
		t.Fatalf("validation changed README: data=%q, err=%v", data, err)
	}
}

func TestValidateRejectsInvalidReadmeWithoutChangingIt(t *testing.T) {
	for _, tt := range []struct {
		name   string
		data   string
		reason string
	}{
		{name: "missing front matter", data: "# Human notes\n", reason: "---"},
		{name: "invalid YAML", data: "---\nid: [\n---\n", reason: "metadata"},
		{name: "id mismatch", data: strings.Replace(validateReadme, validateID, "20260924-other", 1), reason: "20260924-other"},
		{name: "id must match exactly", data: strings.Replace(validateReadme, validateID, "'"+validateID+" '", 1), reason: "id"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "--quiet")
			readme := writeREADME(t, root, validateID, []byte(tt.data))
			var stdout bytes.Buffer
			err := cli.Run([]string{"validate", validateID}, root, time.Time{}, &stdout)
			if err == nil {
				t.Fatal("expected validation error")
			}
			for _, want := range []string{filepath.Join("experiments", validateID, "README.md"), tt.reason} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("validation error %q does not contain %q", err, want)
				}
			}
			if stdout.Len() != 0 {
				t.Errorf("failed validation printed output: %q", stdout.String())
			}
			data, err := os.ReadFile(readme)
			if err != nil || string(data) != tt.data {
				t.Fatalf("failed validation changed README: data=%q, err=%v", data, err)
			}
		})
	}
}

func TestValidateMissingReadme(t *testing.T) {
	for _, existingFolder := range []bool{false, true} {
		name := "missing folder"
		if existingFolder {
			name = "missing README"
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
			err := cli.Run([]string{"validate", validateID}, root, time.Time{}, &stdout)
			wantPath := filepath.Join("experiments", validateID)
			if existingFolder {
				wantPath = filepath.Join(wantPath, "README.md")
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
