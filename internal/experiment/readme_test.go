package experiment

import (
	"bytes"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestRecordRoundTrip(t *testing.T) {
	body := []byte("\r\n# Human notes\r\n\r\n---\r\nKeep this exactly.  \n")
	data := append([]byte("---\r\nid: 20260924-example\r\ntitle: 'Example: a study'\r\nbased_on:\r\n  - 20260920-baseline\r\ntags: [analysis, ml]\r\nseed: 18446744073709551617\r\npayload: !!binary SGVsbG8=\r\n---\r\n"), body...)
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
	for name, data := range map[string]string{
		"no header":       "# Hello\n",
		"unclosed header": "---\nid: example\ntitle: Example\n",
		"empty header":    "---\n---\n",
		"not a mapping":   "---\n- example\n---\n",
		"invalid YAML":    "---\nid: [\n---\n",
		"missing title":   "---\nid: example\n---\n",
		"empty id":        "---\nid: ' '\ntitle: Example\n---\n",
		"numeric id":      "---\nid: 42\ntitle: Example\n---\n",
		"numeric title":   "---\nid: example\ntitle: 42\n---\n",
		"scalar parents":  "---\nid: example\ntitle: Example\nbased_on: previous\n---\n",
		"numeric parent":  "---\nid: example\ntitle: Example\nbased_on: [42]\n---\n",
		"empty parent":    "---\nid: example\ntitle: Example\nbased_on: ['']\n---\n",
		"duplicate key":   "---\nid: one\nid: two\ntitle: Example\n---\n",
		"null key":        "---\nid: example\ntitle: Example\nnull: keep-me\n---\n",
		"numeric key":     "---\nid: example\ntitle: Example\n42: keep-me\n---\n",
		"binary key":      "---\nid: example\ntitle: Example\n!!binary eA==: first\nx: second\n---\n",
		"trailing YAML":   "---\nid: example\ntitle: Example\n...\nlost: value\n---\n",
		"alias":           "---\nid: &id example\ntitle: Example\ncustom: *id\n---\n",
		"merge":           "---\n<<: {id: example, title: Example}\n---\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(data)); err == nil {
				t.Fatal("malformed record accepted")
			}
		})
	}
}

func TestMarshalRejectsReservedExtraFields(t *testing.T) {
	r := Record{ID: "example", Title: "Example", Extra: map[string]yaml.Node{"id": {Kind: yaml.ScalarNode, Tag: "!!str", Value: "other"}}}
	if _, err := r.Marshal(); err == nil {
		t.Fatal("conflicting metadata accepted")
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
