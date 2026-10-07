package report

import (
	"bufio"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

// csvRecords bounds logical records before encoding/csv can allocate their fields.
// Splitting at quoted record boundaries also lets live reads omit unfinished writes.
type csvRecords struct {
	scanner *bufio.Scanner
	pending []byte
}

func (records *csvRecords) Read(p []byte) (int, error) {
	if len(records.pending) == 0 {
		if !records.scanner.Scan() {
			if err := records.scanner.Err(); err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		records.pending = records.scanner.Bytes()
	}
	n := copy(p, records.pending)
	records.pending = records.pending[n:]
	return n, nil
}

func csvRecordSplit(live bool) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		quoted, fieldStart := false, true
		for i := 0; i < len(data); i++ {
			if i >= maxCSVRecordBytes {
				return 0, nil, fmt.Errorf("CSV record exceeds %d bytes", maxCSVRecordBytes)
			}
			if quoted {
				if data[i] == '"' {
					if i+1 < len(data) && data[i+1] == '"' {
						if i+1 >= maxCSVRecordBytes {
							return 0, nil, fmt.Errorf("CSV record exceeds %d bytes", maxCSVRecordBytes)
						}
						i++
					} else {
						quoted = false
					}
				}
				continue
			}
			switch data[i] {
			case '\n':
				return i + 1, data[:i+1], nil
			case ',':
				fieldStart = true
			case '"':
				quoted = fieldStart
				fieldStart = false
			default:
				fieldStart = false
			}
		}
		if atEOF {
			if !live && len(data) > 0 {
				return len(data), data, nil
			}
			return len(data), nil, nil
		}
		return 0, nil, nil
	}
}

func parseCSVStream(source io.Reader, selections []chartSelection, live bool) ([]*Data, error) {
	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 4096), maxCSVRecordBytes+1)
	scanner.Split(csvRecordSplit(live))
	reader := csv.NewReader(&csvRecords{scanner: scanner})
	reader.ReuseRecord = true
	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return make([]*Data, len(selections)), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	columns := make(map[string]int, len(header))
	for i, name := range header {
		if strings.TrimSpace(name) == "" {
			return nil, errors.New("CSV header contains an empty column name")
		}
		if _, found := columns[name]; found {
			return nil, fmt.Errorf("CSV header contains duplicate column %q", name)
		}
		columns[name] = i
	}
	var projections []*csvProjection
	indices := make([]int, len(selections))
	seen := make(map[string]int)
	for i, selection := range selections {
		for _, name := range append([]string{selection.X}, selection.Y...) {
			if _, ok := columns[name]; !ok {
				return nil, fmt.Errorf("CSV is missing column %q", name)
			}
		}
		key, _ := json.Marshal(selection)
		index, ok := seen[string(key)]
		if !ok {
			index = len(projections)
			seen[string(key)] = index
			projections = append(projections, &csvProjection{selection: selection, sampler: newSampler(selection.Limit)})
		}
		indices[i] = index
	}
	for row := 1; ; row++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV row %d: %w", row, err)
		}
		// A column shared by several charts is parsed only once per input row.
		parsed := make(map[int]*float64)
		for _, projection := range projections {
			selection := projection.selection
			xIndex := columns[selection.X]
			xPoint, ok := parsed[xIndex]
			if !ok || xPoint == nil {
				x, err := numericCell(record[xIndex], row, selection.X)
				if err != nil {
					return nil, err
				}
				xPoint = &x
				parsed[xIndex] = xPoint
			}
			if row > 1 && *xPoint <= projection.previousX {
				return nil, fmt.Errorf("CSV row %d: x column %q must be strictly increasing in file order", row, selection.X)
			}
			projection.previousX = *xPoint
			point := sample{row: row, x: *xPoint, values: make([]*float64, len(selection.Y))}
			for i, name := range selection.Y {
				index := columns[name]
				value, ok := parsed[index]
				if !ok {
					if strings.TrimSpace(record[index]) != "" {
						number, err := numericCell(record[index], row, name)
						if err != nil {
							return nil, err
						}
						value = &number
					}
					parsed[index] = value
				}
				point.values[i] = value
				if value != nil && (row == 1 || projection.missing[i]) {
					projection.runs[i]++
				}
				projection.missing[i] = value == nil
				point.runs[i] = projection.runs[i]
				projection.hasValues = projection.hasValues || value != nil
			}
			if row == 1 {
				projection.sourceID = sampleFingerprint(point)
			}
			projection.sampler.add(point)
			projection.recent[(row-1)%maxTableRows] = point
			projection.rows = row
		}
	}
	parsedData := make([]*Data, len(projections))
	for i, projection := range projections {
		if projection.hasValues {
			parsedData[i] = projection.data()
		}
	}
	data := make([]*Data, len(selections))
	for i, index := range indices {
		data[i] = parsedData[index]
	}
	return data, nil
}

func sampleFingerprint(point sample) string {
	// Float bits avoid locale, formatting, and pointer identity in restart markers.
	hash := sha256.New()
	fmt.Fprintf(hash, "%016x", math.Float64bits(point.x))
	for _, value := range point.values {
		if value == nil {
			hash.Write([]byte("nil"))
		} else {
			fmt.Fprintf(hash, "%016x", math.Float64bits(*value))
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type sample struct {
	row    int
	x      float64
	values []*float64
	runs   [maxSeries]int
}

type csvProjection struct {
	selection chartSelection
	sampler   *sampler
	previousX float64
	hasValues bool
	rows      int
	recent    [maxTableRows]sample
	sourceID  string
	runs      [maxSeries]int
	missing   [maxSeries]bool
}

func (projection *csvProjection) data() *Data {
	points := projection.sampler.points()
	data := &Data{TotalRows: projection.rows, Sampled: len(points) < projection.rows, SourceID: projection.sourceID}
	data.X, data.Series = sampleColumns(points, projection.selection.Y, true)
	start := max(1, projection.rows-maxTableRows+1)
	recent := make([]sample, 0, projection.rows-start+1)
	for row := start; row <= projection.rows; row++ {
		recent = append(recent, projection.recent[(row-1)%maxTableRows])
	}
	x, series := sampleColumns(recent, projection.selection.Y, false)
	data.Table = &DataWindow{X: x, Series: series, StartRow: start}
	return data
}

func sampleColumns(points []sample, names []string, breaks bool) ([]float64, []Series) {
	x := make([]float64, len(points))
	series := make([]Series, len(names))
	for i, name := range names {
		series[i] = Series{Name: name, Values: make([]*float64, len(points))}
	}
	var previousRun [maxSeries]int
	for row, point := range points {
		x[row] = point.x
		for i, value := range point.values {
			series[i].Values[row] = value
			if breaks && value != nil {
				if previousRun[i] != 0 && point.runs[i] != previousRun[i] {
					if series[i].Breaks == nil {
						series[i].Breaks = make([]bool, len(points))
					}
					series[i].Breaks[row] = true
				}
				previousRun[i] = point.runs[i]
			}
		}
	}
	return x, series
}

// Adjacent dyadic buckets retain endpoints and per-series extrema. Run identities
// prevent connecting retained values across missing rows omitted by sampling.
type sampler struct {
	limit      int
	span       int
	buckets    []sampleBucket
	candidates int
}

type sampleBucket struct {
	first   sample
	last    sample
	minimum []sample
	maximum []sample
}

func newSampler(limit int) *sampler {
	return &sampler{limit: limit, span: 1}
}

func (sampler *sampler) add(point sample) {
	index := (point.row - 1) / sampler.span
	if index == len(sampler.buckets) {
		sampler.buckets = append(sampler.buckets, newBucket(point))
		sampler.candidates++
	} else {
		sampler.candidates -= sampler.buckets[index].count()
		sampler.buckets[index].add(point)
		sampler.candidates += sampler.buckets[index].count()
	}
	for sampler.candidates > sampler.limit {
		merged := make([]sampleBucket, 0, (len(sampler.buckets)+1)/2)
		for i := 0; i < len(sampler.buckets); i += 2 {
			bucket := sampler.buckets[i]
			if i+1 < len(sampler.buckets) {
				for _, candidate := range sampler.buckets[i+1].points() {
					bucket.add(candidate)
				}
			}
			merged = append(merged, bucket)
		}
		sampler.buckets = merged
		sampler.candidates = 0
		for i := range merged {
			sampler.candidates += merged[i].count()
		}
		sampler.span *= 2
	}
}

func newBucket(point sample) sampleBucket {
	bucket := sampleBucket{first: point, last: point, minimum: make([]sample, len(point.values)), maximum: make([]sample, len(point.values))}
	bucket.add(point)
	return bucket
}

func (bucket *sampleBucket) add(point sample) {
	if point.row < bucket.first.row {
		bucket.first = point
	}
	if point.row > bucket.last.row {
		bucket.last = point
	}
	for i, value := range point.values {
		if value == nil {
			continue
		}
		if bucket.minimum[i].row == 0 || *value < *bucket.minimum[i].values[i] {
			bucket.minimum[i] = point
		}
		if bucket.maximum[i].row == 0 || *value > *bucket.maximum[i].values[i] {
			bucket.maximum[i] = point
		}
	}
}

func (bucket *sampleBucket) points() []sample {
	points := []sample{bucket.first, bucket.last}
	for i := range bucket.minimum {
		if bucket.minimum[i].row != 0 {
			points = append(points, bucket.minimum[i], bucket.maximum[i])
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].row < points[j].row })
	unique := points[:0]
	for _, point := range points {
		if len(unique) == 0 || point.row != unique[len(unique)-1].row {
			unique = append(unique, point)
		}
	}
	return unique
}

func (bucket *sampleBucket) count() int {
	var rows [2 + 2*maxSeries]int
	rows[0], rows[1] = bucket.first.row, bucket.last.row
	for i := range bucket.minimum {
		rows[2+2*i], rows[3+2*i] = bucket.minimum[i].row, bucket.maximum[i].row
	}
	count := 0
	for i, row := range rows[:2+2*len(bucket.minimum)] {
		if row == 0 {
			continue
		}
		unique := true
		for _, previous := range rows[:i] {
			if previous == row {
				unique = false
				break
			}
		}
		if unique {
			count++
		}
	}
	return count
}

func (sampler *sampler) points() []sample {
	points := make(map[int]sample, sampler.limit)
	for i := range sampler.buckets {
		for _, point := range sampler.buckets[i].points() {
			points[point.row] = point
		}
	}
	ordered := make([]sample, 0, len(points))
	for _, point := range points {
		ordered = append(ordered, point)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].row < ordered[j].row })
	return ordered
}

func numericCell(value string, row int, column string) (float64, error) {
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("CSV row %d: column %q must contain a finite number; got %q", row, column, value)
	}
	return number, nil
}
