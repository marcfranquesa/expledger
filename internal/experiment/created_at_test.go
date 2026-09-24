package experiment

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestCreateRecordsUTCTimestampWithLocalDate(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 24, 23, 59, 0, 123456789, time.FixedZone("local", -4*60*60))
	dir, err := Create(root, "timestamp", now, CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "experiments", "20260924-timestamp"); dir != want {
		t.Fatalf("directory = %q, want %q", dir, want)
	}
	data, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "20260924-timestamp" || !record.CreatedAt.Equal(now) {
		t.Fatalf("creation metadata = %s, %s; want local date ID and %s", record.ID, record.CreatedAt, now)
	}
	if !bytes.Contains(data, []byte("created_at: 2026-09-25T03:59:00.123456789Z\n")) {
		t.Fatalf("README does not contain precise UTC timestamp: %s", data)
	}
}

func TestCreatedAtRoundTrip(t *testing.T) {
	body := []byte("\r\n# Notes\r\n\r\nKeep this exactly.  \n")
	for _, value := range []string{
		"2026-09-24T23:59:00.123456789-04:00",
		"'2026-09-24T23:59:00.123456789-04:00'",
		"\"2026-09-25T03:59:00.123456789Z\"",
	} {
		t.Run(value, func(t *testing.T) {
			data := append([]byte("---\nid: example\ntitle: Example\ncreated_at: "+value+"\n---\n"), body...)
			record, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			want := time.Date(2026, 9, 25, 3, 59, 0, 123456789, time.UTC)
			if !record.CreatedAt.Equal(want) || !bytes.Equal(record.Body, body) {
				t.Fatalf("parsed timestamp or body changed: time=%s, body=%q", record.CreatedAt, record.Body)
			}
			if _, exists := record.Extra["created_at"]; exists {
				t.Fatal("created_at retained as extra metadata")
			}
			encoded, err := record.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			got, err := Parse(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if !got.CreatedAt.Equal(want) || !bytes.Equal(got.Body, body) {
				t.Fatalf("round trip changed timestamp or body: time=%s, body=%q", got.CreatedAt, got.Body)
			}
		})
	}
}

func TestLegacyRecordOmitsCreatedAt(t *testing.T) {
	data := []byte("---\nid: example\ntitle: Example\n---\n\n# Existing notes\n")
	record, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !record.CreatedAt.IsZero() {
		t.Fatalf("legacy record timestamp = %s, want zero", record.CreatedAt)
	}
	encoded, err := record.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, data) {
		t.Fatalf("legacy record changed: got=%q, want=%q", encoded, data)
	}
}

func TestParseRejectsInvalidCreatedAt(t *testing.T) {
	for name, value := range map[string]string{
		"malformed":        "yesterday",
		"null":             "null",
		"empty":            "",
		"date only":        "2026-09-24",
		"missing timezone": "2026-09-24T23:59:00",
		"numeric":          "42",
		"sequence":         "[2026-09-24T23:59:00Z]",
		"mapping":          "{time: 2026-09-24T23:59:00Z}",
	} {
		t.Run(name, func(t *testing.T) {
			data := []byte("---\nid: example\ntitle: Example\ncreated_at: " + value + "\n---\n")
			if _, err := Parse(data); err == nil {
				t.Fatal("invalid created_at accepted")
			}
		})
	}
}

func TestMarshalRejectsCreatedAtInExtra(t *testing.T) {
	record := Record{ID: "example", Title: "Example", Extra: map[string]yaml.Node{
		"created_at": {Kind: yaml.ScalarNode, Tag: "!!str", Value: "2026-09-24T23:59:00Z"},
	}}
	if _, err := record.Marshal(); err == nil {
		t.Fatal("created_at override in extra metadata accepted")
	}
}
