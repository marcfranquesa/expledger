package experiment

import (
	"bytes"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestMarshalRejectsUnrepresentableCreatedAt(t *testing.T) {
	for name, createdAt := range map[string]time.Time{
		"negative year":       time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC),
		"five-digit year":     time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		"positive day offset": time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("", 24*60*60)),
		"negative day offset": time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("", -24*60*60)),
		"positive second":     time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("", 1)),
		"negative second":     time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("", -1)),
		"historical offset":   time.Date(1890, 1, 1, 0, 0, 0, 0, time.FixedZone("", 9*60+21)),
	} {
		t.Run(name, func(t *testing.T) {
			r := Record{ID: "example", Title: "Example", CreatedAt: createdAt}
			if _, err := r.Marshal(); err == nil || !strings.Contains(err.Error(), "created_at") {
				t.Fatalf("Marshal error = %v, want created_at representability error", err)
			}
		})
	}
}

func TestMarshalPreservesRepresentableCreatedAt(t *testing.T) {
	for _, value := range []string{
		"0000-01-01T00:00:00+23:59",
		"9999-12-31T23:59:59.999999999-23:59",
		"2026-09-24T23:59:00.123456789-04:00",
		"2026-09-24T23:59:00.123456789+05:45",
		"2026-09-24T23:59:00Z",
	} {
		t.Run(value, func(t *testing.T) {
			createdAt, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				t.Fatal(err)
			}
			r := Record{ID: "example", Title: "Example", CreatedAt: createdAt}
			data, err := r.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			got, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			if !got.CreatedAt.Equal(createdAt) || got.CreatedAt.Format(time.RFC3339Nano) != value {
				t.Fatalf("round trip changed timestamp: got %s, want %s", got.CreatedAt, value)
			}
		})
	}
}

func TestMarshalPreservesEmptyNullMetadata(t *testing.T) {
	for _, custom := range []string{
		"{0}",
		"{nested: [{key: }, {!!null '': !!null ''}], empty_string: ''}",
		"",
	} {
		t.Run(custom, func(t *testing.T) {
			data := []byte("---\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\ncustom: " + custom + "\n---\n")
			before, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			original, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := before.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before.Extra, original.Extra) {
				t.Fatal("Marshal mutated caller-owned metadata nodes")
			}
			after, err := Parse(encoded)
			if err != nil {
				t.Fatal(err)
			}
			want, got := before.Extra["custom"], after.Extra["custom"]
			if !sameYAMLValue(&want, &got) {
				t.Fatalf("round trip changed null metadata: %s", encoded)
			}
		})
	}
}

func FuzzRecordRoundTrip(f *testing.F) {
	for _, data := range []string{
		"---\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n---\n",
		"---\r\nid: example\r\ntitle: 'Example: a study'\r\ncreated_at: '0000-01-01T00:00:00+23:59'\r\nbased_on: [baseline]\r\ncustom: {tags: [analysis, ml], seed: 18446744073709551617, payload: !!binary SGVsbG8=}\r\n---\r\n\r\n# Notes\r\n---\r\nKeep exactly.  \n",
		"---\nid: example\ntitle: Example\ncreated_at: 9999-12-31T23:59:59.999999999-23:59\nbased_on: []\ncustom: !future {enabled: true, missing: null}\n---\nbody without final newline",
		"---\nid: &id example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\ncustom: *id\n---\n",
		"not a README",
	} {
		f.Add([]byte(data))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		before, err := Parse(data)
		if err != nil {
			return
		}
		encoded, err := before.Marshal()
		if err != nil {
			t.Fatalf("Marshal parsed record: %v", err)
		}
		after, err := Parse(encoded)
		if err != nil {
			t.Fatalf("Parse marshaled record: %v", err)
		}
		if before.ID != after.ID || before.Title != after.Title || !before.CreatedAt.Equal(after.CreatedAt) || !slices.Equal(before.BasedOn, after.BasedOn) || !bytes.Equal(before.Body, after.Body) {
			t.Fatalf("round trip changed record: before=%+v, after=%+v", before, after)
		}
		if len(before.Extra) != len(after.Extra) {
			t.Fatalf("round trip changed extra metadata count: before=%d, after=%d", len(before.Extra), len(after.Extra))
		}
		for key, value := range before.Extra {
			got, exists := after.Extra[key]
			if !exists || !sameYAMLValue(&value, &got) {
				t.Errorf("round trip changed extra field %q: before=%+v, after=%+v", key, value, got)
			}
		}
	})
}
