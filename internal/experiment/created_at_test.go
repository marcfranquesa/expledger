package experiment

import (
	"strings"
	"testing"
	"time"
)

func TestCreatedAtRoundTrip(t *testing.T) {
	for _, value := range []string{
		"2026-09-24T23:59:00.123456789-04:00",
		"'2026-09-24T23:59:00.123456789-04:00'",
		"\"2026-09-25T03:59:00.123456789Z\"",
	} {
		t.Run(value, func(t *testing.T) {
			data := []byte("schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: " + value + "\n")
			record, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			want := time.Date(2026, 9, 25, 3, 59, 0, 123456789, time.UTC)
			if !record.CreatedAt.Equal(want) {
				t.Fatalf("parsed timestamp changed: time=%s", record.CreatedAt)
			}
			encoded, err := record.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			got, err := Parse(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if !got.CreatedAt.Equal(want) {
				t.Fatalf("round trip changed timestamp: time=%s", got.CreatedAt)
			}
		})
	}
}

func TestParseAndMarshalRequireCreatedAt(t *testing.T) {
	for _, field := range []string{"", "created_at: 0001-01-01T00:00:00Z\n"} {
		data := []byte("schema: expledger/v1\nid: example\ntitle: Example\n" + field)
		if _, err := Parse(data); err == nil || !strings.Contains(err.Error(), "created_at is required") {
			t.Fatalf("Parse error = %v, want missing or zero created_at error", err)
		}
	}
	record := Record{Schema: Schema, ID: "example", Title: "Example"}
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
			data := []byte("schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: " + value + "\n")
			if _, err := Parse(data); err == nil || !strings.Contains(err.Error(), "created_at must be an RFC3339 timestamp with a timezone") {
				t.Fatalf("error = %v, want created_at format error", err)
			}
		})
	}
}
