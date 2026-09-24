package experiment

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

const receiptMetadata = "schema: expledger/v1\nid: example\ntitle: Example\ncreated_at: 2026-09-24T12:00:00Z\n"

func TestRunReceiptRoundTrip(t *testing.T) {
	for _, hash := range []string{strings.Repeat("abc01234", 5), strings.Repeat("abc01234", 8), strings.Repeat("1", 40)} {
		record, err := Parse([]byte(receiptMetadata))
		if err != nil {
			t.Fatal(err)
		}
		if record.LastRun != nil {
			t.Fatal("legacy metadata unexpectedly has a run receipt")
		}
		record.LastRun = &RunReceipt{ProjectCommit: hash, StartedAt: time.Date(2026, 9, 24, 23, 59, 0, 123456789, time.FixedZone("", -4*60*60))}
		data, err := record.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		got, err := Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		if got.LastRun == nil || got.LastRun.ProjectCommit != hash || !got.LastRun.StartedAt.Equal(record.LastRun.StartedAt) {
			t.Fatalf("run receipt changed: got=%+v, want=%+v", got.LastRun, record.LastRun)
		}
	}
}

func TestParseRejectsInvalidRunReceipt(t *testing.T) {
	hash := strings.Repeat("a", 40)
	valid := "{project_commit: " + hash + ", started_at: 2026-09-24T12:00:00Z}"
	for name, value := range map[string]string{
		"null":                "null",
		"empty scalar":        "",
		"scalar":              "previous",
		"sequence":            "[]",
		"empty mapping":       "{}",
		"missing commit":      "{started_at: 2026-09-24T12:00:00Z}",
		"missing start":       "{project_commit: " + hash + "}",
		"short commit":        strings.Replace(valid, hash, "abc0123", 1),
		"long commit":         strings.Replace(valid, hash, hash+"a", 1),
		"uppercase commit":    strings.Replace(valid, hash, strings.ToUpper(hash), 1),
		"nonhex commit":       strings.Replace(valid, hash, strings.Repeat("z", 40), 1),
		"numeric commit":      strings.Replace(valid, hash, "42", 1),
		"null commit":         strings.Replace(valid, hash, "null", 1),
		"mapping commit":      strings.Replace(valid, hash, "{}", 1),
		"zero start":          strings.Replace(valid, "2026-09-24T12:00:00Z", "0001-01-01T00:00:00Z", 1),
		"null start":          strings.Replace(valid, "2026-09-24T12:00:00Z", "null", 1),
		"date only":           strings.Replace(valid, "2026-09-24T12:00:00Z", "2026-09-24", 1),
		"missing timezone":    strings.Replace(valid, "2026-09-24T12:00:00Z", "2026-09-24T12:00:00", 1),
		"single digit hour":   strings.Replace(valid, "2026-09-24T12:00:00Z", "'2026-09-24T2:00:00Z'", 1),
		"comma fractions":     strings.Replace(valid, "2026-09-24T12:00:00Z", "'2026-09-24T12:00:00,5Z'", 1),
		"out of range offset": strings.Replace(valid, "2026-09-24T12:00:00Z", "'2026-09-24T12:00:00+24:00'", 1),
		"unknown field":       strings.Replace(valid, "}", ", exit_code: 0}", 1),
		"numeric key":         strings.Replace(valid, "}", ", 42: ignored}", 1),
		"duplicate field":     strings.Replace(valid, "}", ", project_commit: "+hash+"}", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(receiptMetadata + "last_run: " + value + "\n")); err == nil {
				t.Fatalf("accepted invalid last_run: %s", value)
			}
		})
	}
}

func TestMarshalRejectsInvalidRunReceipt(t *testing.T) {
	validTime := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	for name, receipt := range map[string]RunReceipt{
		"missing commit":     {StartedAt: validTime},
		"short commit":       {ProjectCommit: "abc0123", StartedAt: validTime},
		"missing start":      {ProjectCommit: strings.Repeat("a", 40)},
		"out of range year":  {ProjectCommit: strings.Repeat("a", 40), StartedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)},
		"out of range zone":  {ProjectCommit: strings.Repeat("a", 40), StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("", 24*60*60))},
		"subminute timezone": {ProjectCommit: strings.Repeat("a", 40), StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("", 1))},
	} {
		t.Run(name, func(t *testing.T) {
			record := Record{Schema: Schema, ID: "example", Title: "Example", CreatedAt: validTime, LastRun: &receipt}
			if _, err := record.Marshal(); err == nil || !strings.Contains(err.Error(), "last_run.") {
				t.Fatalf("Marshal error = %v, want run receipt error", err)
			}
		})
	}
}

func TestWithRunReceiptPreservesOtherYAMLValues(t *testing.T) {
	for _, custom := range customMetadataValues {
		t.Run(custom, func(t *testing.T) {
			original := []byte("# Experiment metadata\n" + receiptMetadata + "# Custom settings\ncustom: " + custom + "\n# Preserve this trailing note\n")
			receipt := RunReceipt{ProjectCommit: strings.Repeat("a", 40), StartedAt: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)}
			updated, err := WithRunReceipt(original, receipt)
			if err != nil {
				t.Fatal(err)
			}
			record, err := Parse(updated)
			if err != nil || record.LastRun == nil || *record.LastRun != receipt {
				t.Fatalf("run receipt=%+v, err=%v; want=%+v", record.LastRun, err, receipt)
			}
			before, after := decodeMetadataNode(t, original), decodeMetadataNode(t, updated)
			if len(after.Content) != len(before.Content)+2 {
				t.Fatalf("update added more than last_run: %s", updated)
			}
			for i, field := range before.Content {
				if !sameYAMLValue(field, after.Content[i]) {
					t.Fatalf("update changed existing YAML field at index %d: %s", i, updated)
				}
			}
			for _, comment := range []string{"# Experiment metadata", "# Custom settings", "# Preserve this trailing note"} {
				if !bytes.Contains(updated, []byte(comment)) {
					t.Fatalf("update lost comment %q: %s", comment, updated)
				}
			}
			receipt.ProjectCommit = strings.Repeat("b", 64)
			receipt.StartedAt = receipt.StartedAt.Add(time.Hour)
			replaced, err := WithRunReceipt(updated, receipt)
			if err != nil {
				t.Fatal(err)
			}
			record, err = Parse(replaced)
			if err != nil || record.LastRun == nil || *record.LastRun != receipt {
				t.Fatalf("replacement receipt=%+v, err=%v; want=%+v", record.LastRun, err, receipt)
			}
			if len(decodeMetadataNode(t, replaced).Content) != len(after.Content) {
				t.Fatalf("replacement duplicated last_run: %s", replaced)
			}
		})
	}
}

func TestWithRunReceiptRejectsInvalidInputs(t *testing.T) {
	receipt := RunReceipt{ProjectCommit: strings.Repeat("a", 40), StartedAt: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)}
	for _, original := range []string{
		"schema: [\n",
		receiptMetadata + "last_run: null\n",
		receiptMetadata + "custom: &item [one]\nother: *item\n",
		receiptMetadata + "---\nother: document\n",
	} {
		if updated, err := WithRunReceipt([]byte(original), receipt); updated != nil || err == nil {
			t.Fatalf("invalid metadata produced data=%q, err=%v", updated, err)
		}
	}
	if updated, err := WithRunReceipt([]byte(receiptMetadata), RunReceipt{}); updated != nil || err == nil {
		t.Fatalf("invalid receipt produced data=%q, err=%v", updated, err)
	}
}

func decodeMetadataNode(t *testing.T, data []byte) *yaml.Node {
	t.Helper()
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document.Content[0]
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
