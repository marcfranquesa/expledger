package experiment

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestRecordRoundTrip(t *testing.T) {
	data := []byte("schema: expledger/v1\r\nid: 20260924-example\r\ntitle: 'Example: a study'\r\ncreated_at: 2026-09-24T12:00:00Z\r\nbased_on:\r\n  - 20260920-baseline\r\ntags: [analysis, ml]\r\nseed: 18446744073709551617\r\npayload: !!binary SGVsbG8=\r\n")
	record, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if record.Schema != Schema || record.ID != "20260924-example" || record.Title != "Example: a study" || !reflect.DeepEqual(record.BasedOn, []string{"20260920-baseline"}) {
		t.Fatalf("unexpected metadata: %+v", record)
	}
	encoded, err := record.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Schema != record.Schema || got.ID != record.ID || got.Title != record.Title || !reflect.DeepEqual(got.BasedOn, record.BasedOn) {
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
		"empty":          {"", "parse metadata"},
		"not a mapping":  {"---\n- example\n", "YAML mapping"},
		"invalid YAML":   {"schema: expledger/v1\nid: [\n", "parse metadata"},
		"missing id":     {"schema: expledger/v1\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n", "id is required"},
		"missing title":  {"schema: expledger/v1\nid: example\ncreated_at: 2026-09-24T12:00:00Z\n", "title is required"},
		"empty id":       {"schema: expledger/v1\nid: ' '\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n", "id is required"},
		"empty title":    {"schema: expledger/v1\nid: example\ntitle: ' '\ncreated_at: 2026-09-24T12:00:00Z\n", "title is required"},
		"numeric id":     {"schema: expledger/v1\nid: 42\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n", "id must be a string"},
		"numeric title":  {"schema: expledger/v1\nid: example\ntitle: 42\ncreated_at: 2026-09-24T12:00:00Z\n", "title must be a string"},
		"scalar parents": {"schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\nbased_on: previous\n", "based_on"},
		"numeric parent": {"schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\nbased_on: [42]\n", "based_on"},
		"empty parent":   {"schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\nbased_on: ['']\n", "based_on"},
		"duplicate key":  {"schema: expledger/v1\nid: one\nid: two\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n", "already defined"},
		"null key":       {"schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\nnull: keep-me\n", "metadata keys must be strings"},
		"numeric key":    {"schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n42: keep-me\n", "metadata keys must be strings"},
		"binary key":     {"schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n!!binary eA==: first\nx: second\n", "metadata keys must be strings"},
		"trailing YAML":  {"schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n...\nlost: value\n", "exactly one YAML document"},
		"alias":          {"schema: expledger/v1\nid: &id example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\ncustom: *id\n", "YAML aliases and merge keys are unsupported"},
		"merge":          {"schema: expledger/v1\ncreated_at: 2026-09-24T12:00:00Z\n<<: {id: example, title: Example}\n", "YAML aliases and merge keys are unsupported"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(tt.data)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestMarshalRejectsReservedExtraFields(t *testing.T) {
	for _, key := range []string{"schema", "id", "title", "created_at", "based_on"} {
		r := Record{Schema: Schema, ID: "example", Title: "Example", CreatedAt: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), Extra: map[string]yaml.Node{key: {Kind: yaml.ScalarNode, Tag: "!!str", Value: "other"}}}
		if _, err := r.Marshal(); err == nil || !strings.Contains(err.Error(), "extra metadata cannot override "+key) {
			t.Fatalf("error = %v, want reserved %s error", err, key)
		}
	}
}

func TestParseRequiresSupportedSchema(t *testing.T) {
	for _, schema := range []string{"", "schema: labexp/v1\n", "schema: expledger/v2\n", "schema: null\n", "schema: 1\n", "schema: [expledger/v1]\n", "schema: expledger/v1\nschema: expledger/v1\n"} {
		data := []byte(schema + "id: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n")
		if _, err := Parse(data); err == nil || !strings.Contains(err.Error(), "schema") {
			t.Fatalf("Parse(%q) error = %v, want schema error", schema, err)
		}
	}
}

func TestMarshalRequiresSupportedSchema(t *testing.T) {
	for _, schema := range []string{"", "labexp/v1", "expledger/v2"} {
		r := Record{Schema: schema, ID: "example", Title: "Example", CreatedAt: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)}
		if _, err := r.Marshal(); err == nil || !strings.Contains(err.Error(), "schema") {
			t.Fatalf("Marshal(%q) error = %v, want schema error", schema, err)
		}
	}
}

func TestParseRejectsAdditionalYAMLDocuments(t *testing.T) {
	metadata := "schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n"
	for _, suffix := range []string{"---\n", "---\nother: record\n", "---\n# README notes\n"} {
		if _, err := Parse([]byte(metadata + suffix)); err == nil || !strings.Contains(err.Error(), "exactly one YAML document") {
			t.Fatalf("Parse error = %v, want multiple document error", err)
		}
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
