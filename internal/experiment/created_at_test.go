package experiment

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

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

func TestParseAndMarshalRequireCreatedAt(t *testing.T) {
	for _, field := range []string{"", "created_at: 0001-01-01T00:00:00Z\n"} {
		data := []byte("---\nid: example\ntitle: Example\n" + field + "---\n\n# Existing notes\n")
		if _, err := Parse(data); err == nil || !strings.Contains(err.Error(), "created_at is required") {
			t.Fatalf("Parse error = %v, want missing or zero created_at error", err)
		}
	}
	record := Record{ID: "example", Title: "Example"}
	if _, err := record.Marshal(); err == nil || !strings.Contains(err.Error(), "created_at is required") {
		t.Fatalf("Marshal error = %v, want missing created_at error", err)
	}
}

func TestParseRejectsInvalidCreatedAt(t *testing.T) {
	for name, value := range map[string]string{
		"malformed":                  "yesterday",
		"null":                       "null",
		"empty":                      "",
		"date only":                  "2026-09-24",
		"missing timezone":           "2026-09-24T23:59:00",
		"single-digit hour":          "'2026-09-24T4:30:00Z'",
		"comma fraction":             "'2026-09-24T14:30:00,5Z'",
		"offset hour out of range":   "'2026-09-24T14:30:00+24:00'",
		"offset minute out of range": "'2026-09-24T14:30:00+00:60'",
		"numeric":                    "42",
		"sequence":                   "[2026-09-24T23:59:00Z]",
		"mapping":                    "{time: 2026-09-24T23:59:00Z}",
	} {
		t.Run(name, func(t *testing.T) {
			data := []byte("---\nid: example\ntitle: Example\ncreated_at: " + value + "\n---\n")
			if _, err := Parse(data); err == nil || !strings.Contains(err.Error(), "created_at must be an RFC3339 timestamp with a timezone") {
				t.Fatalf("error = %v, want created_at format error", err)
			}
		})
	}
}

func TestMarshalRejectsCreatedAtInExtra(t *testing.T) {
	record := Record{ID: "example", Title: "Example", CreatedAt: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), Extra: map[string]yaml.Node{
		"created_at": {Kind: yaml.ScalarNode, Tag: "!!str", Value: "2026-09-24T23:59:00Z"},
	}}
	if _, err := record.Marshal(); err == nil || !strings.Contains(err.Error(), "extra metadata cannot override created_at") {
		t.Fatalf("error = %v, want reserved created_at error", err)
	}
}
