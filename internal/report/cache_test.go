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

func TestReaderRetainsCachedCatalogSubset(t *testing.T) {
	for _, tc := range []struct {
		name    string
		sources int
		rows    int
	}{
		{"source limit", maxCachedSources + 1, 1},
		{"value limit", maxCachedValues/((maxPlotRows+maxTableRows)*(maxSeries+1)) + 1, maxPlotRows},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			manifest := "blocks: [{type: line, title: Metrics, source: results.csv, x: step, y: [a,b,c,d,e,f,g,h]}]\n"
			var source strings.Builder
			source.WriteString("step,a,b,c,d,e,f,g,h\n")
			for row := range tc.rows {
				fmt.Fprintf(&source, "%d,1,2,3,4,5,6,7,8\n", row)
			}
			contents := source.String()
			ids := make([]string, tc.sources)
			for i := range ids {
				ids[i] = fmt.Sprintf("experiment%d", i)
				dir := filepath.Join(root, "experiments", ids[i])
				writeReportFile(t, filepath.Join(dir, "report.yaml"), manifest)
				writeReportFile(t, filepath.Join(dir, "results.csv"), contents)
			}
			reader := NewReader(true)
			read := func() map[string]*Report {
				t.Helper()
				reports, err := reader.ReadMany(root, ids, catalog.Layout{})
				if err != nil || len(reports) != len(ids) {
					t.Fatalf("ReadMany returned %d reports: %v", len(reports), err)
				}
				if len(reader.cache) > maxCachedSources || reader.cachedValues > maxCachedValues {
					t.Fatalf("cache contains %d sources and %d values", len(reader.cache), reader.cachedValues)
				}
				return reports
			}
			first := read()
			admitted := make(map[*Data]bool)
			for _, cached := range reader.cache {
				admitted[cached.data[0]] = true
			}
			if len(admitted) != len(ids)-1 {
				t.Fatalf("admitted %d sources, want %d", len(admitted), len(ids)-1)
			}
			warm := make(map[string]*Data)
			var cachedID, overflowID string
			for _, id := range ids {
				data := first[id].Blocks[0].Data
				if admitted[data] {
					warm[id] = data
					cachedID = id
				} else {
					overflowID = id
				}
			}
			checkWarm := func(reports map[string]*Report) {
				t.Helper()
				for id, previous := range warm {
					if reports[id].Blocks[0].Data != previous {
						t.Fatalf("unchanged admitted source %s was reparsed", id)
					}
				}
			}
			previousOverflow := first[overflowID].Blocks[0].Data
			checkOverflow := func(reports map[string]*Report, want float64) {
				t.Helper()
				data := reports[overflowID].Blocks[0].Data
				if data == previousOverflow || *data.Series[0].Values[0] != want {
					t.Fatal("overflow source was not read afresh")
				}
				for _, cached := range reader.cache {
					if cached.data[0] == data {
						t.Fatal("overflow source displaced an admitted entry")
					}
				}
				previousOverflow = data
			}
			poll := func(want float64) {
				t.Helper()
				reports := read()
				checkWarm(reports)
				checkOverflow(reports, want)
			}
			poll(1)
			poll(1)

			stamp := time.Unix(1700000000, 0)
			rewrite := func(id string, value string) {
				t.Helper()
				path := filepath.Join(root, "experiments", id, "results.csv")
				updated := strings.ReplaceAll(contents, ",1,2,3,4,5,6,7,8\n", ","+value+",2,3,4,5,6,7,8\n")
				writeReportFile(t, path, updated)
				stamp = stamp.Add(time.Second)
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
			}
			rewrite(overflowID, "9")
			poll(9)
			previousCached := warm[cachedID]
			rewrite(cachedID, "7")
			reports := read()
			updated := reports[cachedID].Blocks[0].Data
			if updated == previousCached || *updated.Series[0].Values[0] != 7 || *previousCached.Series[0].Values[0] != 1 {
				t.Fatal("cached rewrite did not refresh without mutating the prior projection")
			}
			warm[cachedID] = updated
			checkWarm(reports)
			checkOverflow(reports, 9)
			poll(9)
		})
	}
}
