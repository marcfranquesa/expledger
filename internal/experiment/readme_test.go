package experiment

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestRecordRoundTrip(t *testing.T) {
	body := []byte("\r\n# Human notes\r\n\r\n---\r\nKeep this exactly.  \n")
	data := append([]byte("---\r\nid: 20260924-example\r\ntitle: 'Example: a study'\r\ncreated_at: 2026-09-24T12:00:00Z\r\nbased_on:\r\n  - 20260920-baseline\r\ntags: [analysis, ml]\r\nseed: 18446744073709551617\r\npayload: !!binary SGVsbG8=\r\n---\r\n"), body...)
	record, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "20260924-example" || record.Title != "Example: a study" || !reflect.DeepEqual(record.BasedOn, []string{"20260920-baseline"}) {
		t.Fatalf("unexpected metadata: %+v", record)
	}
	if !bytes.Equal(record.Body, body) {
		t.Fatalf("body changed: %q", record.Body)
	}
	encoded, err := record.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != record.ID || got.Title != record.Title || !reflect.DeepEqual(got.BasedOn, record.BasedOn) || !bytes.Equal(got.Body, record.Body) {
		t.Fatalf("round trip changed record: got=%+v, want=%+v", got, record)
	}
	for _, key := range []string{"tags", "seed", "payload"} {
		before, after := record.Extra[key], got.Extra[key]
		if !sameYAMLValue(&before, &after) {
			t.Errorf("round trip changed unknown field %q: before=%+v, after=%+v", key, before, after)
		}
	}
}

func TestParseRejectsMalformedRecords(t *testing.T) {
	for name, tt := range map[string]struct {
		data string
		want string
	}{
		"no header":       {"# Hello\n", "README must begin"},
		"unclosed header": {"---\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n", "missing its closing"},
		"empty header":    {"---\n---\n", "parse metadata"},
		"not a mapping":   {"---\n- example\n---\n", "YAML mapping"},
		"invalid YAML":    {"---\nid: [\n---\n", "parse metadata"},
		"missing id":      {"---\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n---\n", "id is required"},
		"missing title":   {"---\nid: example\ncreated_at: 2026-09-24T12:00:00Z\n---\n", "title is required"},
		"empty id":        {"---\nid: ' '\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n---\n", "id is required"},
		"empty title":     {"---\nid: example\ntitle: ' '\ncreated_at: 2026-09-24T12:00:00Z\n---\n", "title is required"},
		"numeric id":      {"---\nid: 42\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n---\n", "id must be a string"},
		"numeric title":   {"---\nid: example\ntitle: 42\ncreated_at: 2026-09-24T12:00:00Z\n---\n", "title must be a string"},
		"scalar parents":  {"---\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\nbased_on: previous\n---\n", "based_on"},
		"numeric parent":  {"---\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\nbased_on: [42]\n---\n", "based_on"},
		"empty parent":    {"---\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\nbased_on: ['']\n---\n", "based_on"},
		"duplicate key":   {"---\nid: one\nid: two\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n---\n", "already defined"},
		"null key":        {"---\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\nnull: keep-me\n---\n", "metadata keys must be strings"},
		"numeric key":     {"---\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n42: keep-me\n---\n", "metadata keys must be strings"},
		"binary key":      {"---\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n!!binary eA==: first\nx: second\n---\n", "metadata keys must be strings"},
		"trailing YAML":   {"---\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n...\nlost: value\n---\n", "exactly one YAML document"},
		"alias":           {"---\nid: &id example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\ncustom: *id\n---\n", "YAML aliases and merge keys are unsupported"},
		"merge":           {"---\ncreated_at: 2026-09-24T12:00:00Z\n<<: {id: example, title: Example}\n---\n", "YAML aliases and merge keys are unsupported"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(tt.data)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestMarshalRejectsReservedExtraFields(t *testing.T) {
	r := Record{ID: "example", Title: "Example", CreatedAt: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), Extra: map[string]yaml.Node{"id": {Kind: yaml.ScalarNode, Tag: "!!str", Value: "other"}}}
	if _, err := r.Marshal(); err == nil || !strings.Contains(err.Error(), "extra metadata cannot override id") {
		t.Fatalf("error = %v, want reserved id error", err)
	}
}

func sameYAMLValue(a, b *yaml.Node) bool {
	if a.Kind != b.Kind || a.Tag != b.Tag || a.Value != b.Value || len(a.Content) != len(b.Content) {
		return false
	}
	for i := range a.Content {
		if !sameYAMLValue(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}
