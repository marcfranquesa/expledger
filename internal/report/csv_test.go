package report

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strings"
	"testing"
)

func parseCSV(source []byte, xColumn string, yColumns []string) (*Data, error) {
	data, err := parseCSVStream(bytes.NewReader(source), []chartSelection{{xColumn, yColumns, maxPlotRows}}, false)
	if err != nil {
		return nil, err
	}
	return data[0], nil
}

func TestLargeCSVSamplingPreservesExtremaEndpointsAndGaps(t *testing.T) {
	const rows = 25001
	var source strings.Builder
	source.WriteString("step,a,b\n")
	for i := range rows {
		a, b := fmt.Sprint(i%7), fmt.Sprint(i%11)
		if i == 11999 {
			a = "1000000"
		}
		if i == 18001 {
			b = "-1000000"
		}
		if i >= 12001 && i <= 12009 {
			a = ""
		}
		if i >= 17003 && i <= 17008 {
			b = ""
		}
		fmt.Fprintf(&source, "%d,%s,%s\n", i, a, b)
	}
	data, err := parseCSV([]byte(source.String()), "step", []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if data.TotalRows != rows || !data.Sampled || len(data.X) > maxPlotRows {
		t.Fatalf("projection rows=%d sampled=%v plotted=%d", data.TotalRows, data.Sampled, len(data.X))
	}
	if data.X[0] != 0 || data.X[len(data.X)-1] != rows-1 {
		t.Fatal("sampling lost an endpoint")
	}
	indices := make(map[float64]int)
	for i, x := range data.X {
		indices[x] = i
		if i > 0 && x <= data.X[i-1] {
			t.Fatal("sampled x is not increasing")
		}
	}
	for _, x := range []float64{11999, 18001} {
		if _, ok := indices[x]; !ok {
			t.Fatalf("sampling lost required row x=%v", x)
		}
	}
	if *data.Series[0].Values[indices[11999]] != 1000000 || *data.Series[1].Values[indices[18001]] != -1000000 {
		t.Fatal("sampling changed extrema")
	}
	for series, gap := range [][2]int{{12001, 12009}, {17003, 17008}} {
		previous := -1
		for i, x := range data.X {
			if data.Series[series].Values[i] == nil {
				continue
			}
			if previous >= 0 && data.X[previous] < float64(gap[0]) && x > float64(gap[1]) {
				if len(data.Series[series].Breaks) != len(data.X) || !data.Series[series].Breaks[i] {
					t.Fatal("sampling bridged an omitted missing-value gap")
				}
			}
			previous = i
		}
	}
	if len(data.Table.X) != maxTableRows || data.Table.StartRow != rows-maxTableRows+1 {
		t.Fatalf("recent table = %#v", data.Table)
	}
	for i, x := range data.Table.X {
		want := rows - maxTableRows + i
		if x != float64(want) || *data.Table.Series[0].Values[i] != float64(want%7) {
			t.Fatalf("recent row %d was sampled or changed", i)
		}
	}
}

func TestSmallCSVRemainsExactAndRawWindowHasGaps(t *testing.T) {
	data, err := parseCSV([]byte("step,a\n0,1\n1,\n2,3"), "step", []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if data.Sampled || data.TotalRows != 3 || fmt.Sprint(data.X) != "[0 1 2]" || data.Table.StartRow != 1 {
		t.Fatalf("small data = %#v", data)
	}
	if data.Series[0].Values[1] != nil || data.Table.Series[0].Values[1] != nil {
		t.Fatal("missing cell lost")
	}
	if len(data.Table.Series[0].Breaks) != 0 {
		t.Fatal("raw table has chart break metadata")
	}
}

func TestLiveCSVWaitsForCompleteRecords(t *testing.T) {
	for _, tail := range []string{"1,2", "1,invalid", "1,\"2", "1,\"2\n"} {
		t.Run(fmt.Sprintf("%q", tail), func(t *testing.T) {
			data, err := parseCSVStream(strings.NewReader("step,a\n0,1\n"+tail), []chartSelection{{"step", []string{"a"}, maxPlotRows}}, true)
			if err != nil {
				t.Fatal(err)
			}
			if data[0].TotalRows != 1 {
				t.Fatalf("live read exposed unfinished record: %#v", data[0])
			}
		})
	}
	for _, live := range []bool{false, true} {
		data, err := parseCSVStream(strings.NewReader("step,a,note\n0,1,\"first\nsecond\"\n1,2,\"with \"\"quotes\"\"\"\n"), []chartSelection{{"step", []string{"a"}, maxPlotRows}}, live)
		if err != nil || data[0].TotalRows != 2 {
			t.Fatalf("quoted records: data=%#v error=%v", data, err)
		}
		_, err = parseCSVStream(strings.NewReader("step,a\n0,1\n1,2\"bad\n"), []chartSelection{{"step", []string{"a"}, maxPlotRows}}, live)
		if err == nil {
			t.Fatal("accepted invalid completed record")
		}
	}
	_, err := parseCSVStream(strings.NewReader("step,a\n0,1\n1,\"2"), []chartSelection{{"step", []string{"a"}, maxPlotRows}}, false)
	if err == nil {
		t.Fatal("static CSV accepted malformed final record")
	}
}

func TestCSVCRLFQuotedMultilineAndLiveTail(t *testing.T) {
	complete := "step,a,note\r\n0,1,\"first\r\nsecond\"\r\n1,2,\"with \"\"quotes\"\"\"\r\n"
	selection := []chartSelection{{"step", []string{"a"}, maxPlotRows}}
	for _, live := range []bool{false, true} {
		data, err := parseCSVStream(strings.NewReader(complete), selection, live)
		if err != nil || data[0].TotalRows != 2 || *data[0].Series[0].Values[1] != 2 {
			t.Fatalf("CRLF multiline data=%#v error=%v", data, err)
		}
	}
	for _, tail := range []string{"2,3,\"third\r\nfourth", "2,3,\"third\r\nfourth\""} {
		data, err := parseCSVStream(strings.NewReader(complete+tail), selection, true)
		if err != nil || data[0].TotalRows != 2 {
			t.Fatalf("live CRLF tail data=%#v error=%v", data, err)
		}
	}
	data, err := parseCSVStream(strings.NewReader(complete+"2,3,\"third\r\nfourth\"\r\n"), selection, true)
	if err != nil || data[0].TotalRows != 3 {
		t.Fatalf("completed CRLF tail data=%#v error=%v", data, err)
	}
}

func TestLiveCSVIncompleteHeaderAndEmptyResults(t *testing.T) {
	for _, source := range []string{"", "step,a", "step,\"a\n", "step,a\n", "step,a\n0,\n"} {
		data, err := parseCSVStream(strings.NewReader(source), []chartSelection{{"step", []string{"a"}, maxPlotRows}}, true)
		if err != nil || len(data) != 1 || data[0] != nil {
			t.Fatalf("incomplete/empty results %q = (%#v, %v)", source, data, err)
		}
	}
}

func TestCSVRecordLimit(t *testing.T) {
	for _, source := range []string{
		"step,a,note\n0,1," + strings.Repeat("x", maxCSVRecordBytes) + "\n",
		"step,a,note\n0,1,\"" + strings.Repeat("x\n", maxCSVRecordBytes/2) + "\"\n",
	} {
		_, err := parseCSV([]byte(source), "step", []string{"a"})
		if err == nil || !strings.Contains(err.Error(), "record exceeds") {
			t.Fatalf("record limit error = %v", err)
		}
	}

}

func TestCSVSharedProjectionsHaveOneSnapshotAndReuseSelection(t *testing.T) {
	selections := []chartSelection{{"step", []string{"a"}, 512}, {"step", []string{"b"}, 512}, {"step", []string{"a"}, 512}}
	data, err := parseCSVStream(strings.NewReader("step,a,b\n0,1,2\n1,3,4\n"), selections, false)
	if err != nil {
		t.Fatal(err)
	}
	if data[0] != data[2] || data[0].TotalRows != data[1].TotalRows || *data[1].Series[0].Values[1] != 4 {
		t.Fatal("shared projections did not reuse the same parsed stream")
	}
}

func TestSparseValidationSamplingPreservesEveryDiscontinuity(t *testing.T) {
	const rows = 1000001
	var source strings.Builder
	source.WriteString("step,train,validation\n")
	for i := range rows {
		validation := ""
		if i%1000 == 0 {
			validation = fmt.Sprint(i % 17000)
		}
		fmt.Fprintf(&source, "%d,%d,%s\n", i, i%17000, validation)
	}
	data, err := parseCSV([]byte(source.String()), "step", []string{"train", "validation"})
	if err != nil {
		t.Fatal(err)
	}
	if data.TotalRows != rows || len(data.X) > maxPlotRows {
		t.Fatal("sparse validation was rejected or output grew")
	}
	lastValid := -1
	validation := data.Series[1]
	for i, value := range validation.Values {
		if value == nil {
			continue
		}
		x := int(data.X[i])
		if x%1000 != 0 || *value != float64(x%17000) {
			t.Fatal("sampler fabricated a validation value")
		}
		if lastValid >= 0 && (len(validation.Breaks) != len(data.X) || !validation.Breaks[i]) {
			t.Fatal("sampler connected separate validation runs")
		}
		lastValid = i
	}
	if lastValid < 0 {
		t.Fatal("validation extrema disappeared")
	}
}

type repeatingCSVField struct{}

func (repeatingCSVField) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

type countingCSVReader struct {
	io.Reader
	bytes int
}

func (reader *countingCSVReader) Read(p []byte) (int, error) {
	n, err := reader.Reader.Read(p)
	reader.bytes += n
	return n, err
}

func TestOversizedCSVRecordStopsReadingAtBound(t *testing.T) {
	source := &countingCSVReader{Reader: io.MultiReader(strings.NewReader("step,a,note\n0,1,"), io.LimitReader(repeatingCSVField{}, 10<<20), strings.NewReader("\n"))}
	_, err := parseCSVStream(source, []chartSelection{{"step", []string{"a"}, maxPlotRows}}, false)
	if err == nil || !strings.Contains(err.Error(), "record exceeds") {
		t.Fatalf("record bound error = %v", err)
	}
	if source.bytes > 2*maxCSVRecordBytes {
		t.Fatalf("record limit read %d bytes of oversized input", source.bytes)
	}
}

func TestSamplerInvariantsAgainstOriginalRows(t *testing.T) {
	random := rand.New(rand.NewSource(7730))
	for trial := range 30 {
		rows := 600 + random.Intn(1400)
		seriesCount := 1 + random.Intn(maxSeries)
		limit := 73 + random.Intn(maxPlotRows-72)
		names := make([]string, seriesCount)
		var source strings.Builder
		source.WriteString("step")
		for i := range names {
			names[i] = fmt.Sprintf("y%d", i)
			fmt.Fprintf(&source, ",%s", names[i])
		}
		source.WriteByte('\n')
		original := make([][]*float64, rows)
		runs := make([][]int, rows)
		currentRuns := make([]int, seriesCount)
		minimum, maximum := make([]float64, seriesCount), make([]float64, seriesCount)
		for i := range minimum {
			minimum[i], maximum[i] = math.Inf(1), math.Inf(-1)
		}
		for row := range rows {
			original[row], runs[row] = make([]*float64, seriesCount), make([]int, seriesCount)
			fmt.Fprintf(&source, "%d", row)
			for series := range seriesCount {
				source.WriteByte(',')
				if random.Intn(5) != 0 {
					value := float64(random.Intn(1000000) - 500000)
					original[row][series] = &value
					fmt.Fprintf(&source, "%.0f", value)
					minimum[series] = min(minimum[series], value)
					maximum[series] = max(maximum[series], value)
					if row == 0 || original[row-1][series] == nil {
						currentRuns[series]++
					}
				}
				runs[row][series] = currentRuns[series]
			}
			source.WriteByte('\n')
		}
		parsed, err := parseCSVStream(strings.NewReader(source.String()), []chartSelection{{"step", names, limit}}, false)
		if err != nil {
			t.Fatal(err)
		}
		data := parsed[0]
		if data.TotalRows != rows || len(data.X) > limit || data.X[0] != 0 || data.X[len(data.X)-1] != float64(rows-1) {
			t.Fatalf("trial %d: output bound/endpoints failed", trial)
		}
		for series := range seriesCount {
			foundMinimum, foundMaximum, previousRun := false, false, 0
			for i, x := range data.X {
				row := int(x)
				value, want := data.Series[series].Values[i], original[row][series]
				if (value == nil) != (want == nil) || (value != nil && *value != *want) {
					t.Fatal("sampling fabricated a row value")
				}
				if value == nil {
					continue
				}
				foundMinimum = foundMinimum || *value == minimum[series]
				foundMaximum = foundMaximum || *value == maximum[series]
				if previousRun != 0 && previousRun != runs[row][series] && (len(data.Series[series].Breaks) != len(data.X) || !data.Series[series].Breaks[i]) {
					t.Fatal("sampling connected different original contiguous runs")
				}
				previousRun = runs[row][series]
			}
			if !foundMinimum || !foundMaximum {
				t.Fatal("sampling discarded global extrema")
			}
		}
	}
}
