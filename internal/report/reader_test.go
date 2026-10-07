package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/catalog"
)

func TestLiveReaderInvalidatesCacheAndRecovers(t *testing.T) {
	root, dir := reportFixture(t, "blocks: [{type: line, title: Loss, source: results.csv, x: step, y: [loss]}]\n", map[string]string{"results.csv": "step,loss\n0,1\n1,2\n"})
	reader := NewReader(true)
	read := func() *Data {
		t.Helper()
		reports, err := reader.ReadMany(root, []string{"example"}, catalog.Layout{})
		if err != nil {
			t.Fatal(err)
		}
		return reports["example"].Blocks[0].Data
	}
	first := read()
	if next := read(); next != first {
		t.Fatal("unchanged source did not reuse bounded projection")
	}
	original, err := os.ReadFile(filepath.Join(dir, "results.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != "step,loss\n0,1\n1,2\n" {
		t.Fatal("reader modified CSV")
	}

	stamp := time.Unix(1700000000, 0)
	write := func(contents string) {
		t.Helper()
		writeReportFile(t, filepath.Join(dir, "results.csv"), contents)
		stamp = stamp.Add(time.Second)
		if err := os.Chtimes(filepath.Join(dir, "results.csv"), stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	write("step,loss\n0,1\n1,2\n2,3")
	partial := read()
	if partial.TotalRows != 2 || partial.SourceID != first.SourceID {
		t.Fatal("partial append changed visible history")
	}
	write("step,loss\n0,1\n1,2\n2,3\n")
	appended := read()
	if appended.TotalRows != 3 || appended.SourceID != first.SourceID || appended == first {
		t.Fatal("append did not refresh projection")
	}

	writeReportFile(t, filepath.Join(dir, "replacement.csv"), "step,loss\n0,1\n1,2\n2,3\n3,4\n")
	if err := os.Rename(filepath.Join(dir, "replacement.csv"), filepath.Join(dir, "results.csv")); err != nil {
		t.Fatal(err)
	}
	replaced := read()
	if replaced.TotalRows != 4 || replaced.SourceID != first.SourceID {
		t.Fatal("atomic continuation lost history identity")
	}
	write("step,loss\n0,1\n")
	if truncated := read(); truncated.TotalRows != 1 {
		t.Fatal("truncate did not refresh")
	}

	write("step,loss\n0,invalid\n")
	if reports, err := reader.ReadMany(root, []string{"example"}, catalog.Layout{}); err == nil || reports != nil {
		t.Fatal("invalid rewrite returned a partial report")
	}
	write("step,loss\n0,9\n1,8\n")
	recovered := read()
	if recovered.TotalRows != 2 || recovered.SourceID == first.SourceID || *recovered.Series[0].Values[0] != 9 {
		t.Fatal("valid rewrite did not recover or reset fingerprint")
	}
	if first.TotalRows != 2 || *first.Series[0].Values[0] != 1 {
		t.Fatal("cached projection was mutated")
	}

	write("step,loss\n0,7\n1,6\n")
	if rewritten := read(); *rewritten.Series[0].Values[0] != 7 {
		t.Fatal("same-size rewrite reused cache")
	}
}

func TestReaderRechecksConfinementOnCacheHit(t *testing.T) {
	root, dir := reportFixture(t, "blocks: [{type: line, title: Loss, source: results.csv, x: step, y: [loss]}]\n", map[string]string{"results.csv": "step,loss\n0,1\n"})
	reader := NewReader(true)
	if _, err := reader.ReadMany(root, []string{"example"}, catalog.Layout{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "results.csv")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	writeReportFile(t, filepath.Join(root, "outside.csv"), "step,loss\n0,1\n")
	if err := os.Symlink("../../outside.csv", path); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadMany(root, []string{"example"}, catalog.Layout{}); err == nil {
		t.Fatal("cached data bypassed source confinement")
	}
}

func TestReaderCacheAndReportDisplayAreBounded(t *testing.T) {
	minimumPlotRows := maxReportValues/(maxBlocks*(1+maxSeries)) - maxTableRows
	if minimumPlotRows < 2+2*maxSeries {
		t.Fatalf("worst report plot budget %d cannot retain one bucket's endpoints/extrema", minimumPlotRows)
	}
	var source strings.Builder
	source.WriteString("step,a,b,c,d,e,f,g,h\n")
	for i := range 2000 {
		fmt.Fprintf(&source, "%d,1,2,3,4,5,6,7,8\n", i)
	}
	root, dir := reportFixture(t, "", nil)
	reader := NewReader(true)
	for i := range 70 {
		name := fmt.Sprintf("results%d.csv", i)
		writeReportFile(t, filepath.Join(dir, name), source.String())
		writeReportFile(t, filepath.Join(dir, "report.yaml"), fmt.Sprintf("blocks: [{type: line, title: Metrics, source: %s, x: step, y: [a,b,c,d,e,f,g,h]}]\n", name))
		if _, err := reader.ReadMany(root, []string{"example"}, catalog.Layout{}); err != nil {
			t.Fatal(err)
		}
		if len(reader.cache) > maxCachedSources || reader.cachedValues > maxCachedValues {
			t.Fatal("projection cache exceeded limits")
		}
	}
	manifest := "blocks:\n" + strings.Repeat("  - {type: line, title: Metrics, source: results0.csv, x: step, y: [a,b,c,d,e,f,g,h]}\n", maxBlocks)
	writeReportFile(t, filepath.Join(dir, "report.yaml"), manifest)
	reports, err := reader.ReadMany(root, []string{"example"}, catalog.Layout{})
	if err != nil {
		t.Fatal(err)
	}
	values := 0
	for _, block := range reports["example"].Blocks {
		values += (len(block.Data.X) + len(block.Data.Table.X)) * (1 + len(block.Data.Series))
		if block.Data.TotalRows != 2000 {
			t.Fatal("display budget limited source row count")
		}
	}
	if values > maxReportValues {
		t.Fatalf("report contains %d values, limit %d", values, maxReportValues)
	}
}

func TestReaderReplacesCachedSourceAfterManifestSelectionEdit(t *testing.T) {
	root, dir := reportFixture(t, "blocks: [{type: line, title: Metrics, source: results.csv, x: step, y: [a]}]\n", map[string]string{"results.csv": "step,iteration,a,b,c\n0,100,1,10,1000\n1,200,2,20,2000\n"})
	reader := NewReader(true)
	read := func() *Data {
		t.Helper()
		reports, err := reader.ReadMany(root, []string{"example"}, catalog.Layout{})
		if err != nil {
			t.Fatal(err)
		}
		if len(reader.cache) != 1 {
			t.Fatalf("manifest selection left %d source cache entries", len(reader.cache))
		}
		return reports["example"].Blocks[0].Data
	}
	first := read()
	writeReportFile(t, filepath.Join(dir, "report.yaml"), "blocks: [{type: line, title: Metrics, source: ./results.csv, x: iteration, y: [b,c]}]\n")
	updated := read()
	if updated == first || fmt.Sprint(updated.X) != "[100 200]" || len(updated.Series) != 2 || updated.Series[0].Name != "b" || *updated.Series[0].Values[0] != 10 || updated.Series[1].Name != "c" || *updated.Series[1].Values[1] != 2000 {
		t.Fatal("manifest selection reused old x/y projection")
	}
	if again := read(); again != updated {
		t.Fatal("updated manifest selection was not cached")
	}
	if first.Series[0].Name != "a" || *first.Series[0].Values[0] != 1 || first.X[0] != 0 {
		t.Fatal("manifest selection mutated prior projection")
	}
}
