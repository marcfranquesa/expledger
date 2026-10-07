// Package report loads an experiment's optional report layout and its local sources.
package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/marcfranquesa/expledger/internal/catalog"
)

const (
	maxManifestBytes  = 256 << 10
	maxMarkdownBytes  = 1 << 20
	maxCSVBytes       = 256 << 20
	maxSourceBytes    = 512 << 20
	maxBlocks         = 64
	maxSeries         = 8
	maxPlotRows       = 512
	maxTableRows      = 100
	maxCSVRecordBytes = 64 << 10
	maxReportValues   = 100000
	maxCachedSources  = 64
	maxCachedValues   = 200000
)

type Report struct {
	Blocks []Block `json:"blocks"`
}

// Block keeps chart selection separate from presentation, which belongs to the renderer.
type Block struct {
	Type     string   `json:"type"`
	Source   string   `json:"source,omitempty"`
	Title    string   `json:"title,omitempty"`
	X        string   `json:"x,omitempty"`
	Y        []string `json:"y,omitempty"`
	Blocks   []Block  `json:"blocks,omitempty"`
	Markdown string   `json:"markdown,omitempty"`
	Data     *Data    `json:"data,omitempty"`
	Message  string   `json:"message,omitempty"`
}

type Data struct {
	X         []float64   `json:"x"`
	Series    []Series    `json:"series"`
	TotalRows int         `json:"totalRows"`
	Sampled   bool        `json:"sampled"`
	Table     *DataWindow `json:"table"`
	SourceID  string      `json:"sourceID"`
}

type DataWindow struct {
	X        []float64 `json:"x"`
	Series   []Series  `json:"series"`
	StartRow int       `json:"startRow"`
}

type Series struct {
	Name   string     `json:"name"`
	Values []*float64 `json:"values"`
	Breaks []bool     `json:"breaks,omitempty"`
}

// Read returns nil when report.yaml is absent. Sources are confined to the experiment
// directory; missing chart results remain visible as unavailable blocks.
func Read(root, id string, layout catalog.Layout) (*Report, error) {
	reports, err := ReadMany(root, []string{id}, layout)
	return reports[id], err
}

// ReadMany loads only the requested reports, omitting absent manifests.
// Each report's sources are confined to its own experiment directory.
func ReadMany(root string, ids []string, layout catalog.Layout) (map[string]*Report, error) {
	return NewReader(false).ReadMany(root, ids, layout)
}

// Reader reuses bounded chart projections when sources are unchanged. Live readers
// wait for the final CSV record's newline before exposing it.
type Reader struct {
	mu           sync.Mutex
	live         bool
	cache        map[string]cachedCSV
	cachedValues int
}

type cachedCSV struct {
	info       os.FileInfo
	projection string
	data       []*Data
	values     int
}

func NewReader(live bool) *Reader {
	return &Reader{live: live, cache: make(map[string]cachedCSV)}
}

func (reader *Reader) ReadMany(root string, ids []string, layout catalog.Layout) (map[string]*Report, error) {
	reader.mu.Lock()
	defer reader.mu.Unlock()

	if err := layout.Validate(); err != nil {
		return nil, err
	}
	layout = layout.WithDefaults()
	reports := make(map[string]*Report)
	if len(ids) == 0 {
		return reports, nil
	}
	project, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open project directory: %w", err)
	}
	defer project.Close()
	if err := catalog.CheckExperimentDirectories(project, layout.ExperimentsDir, ids); err != nil {
		return nil, err
	}
	for _, id := range ids {
		content, err := reader.readExperiment(project, root, layout.ExperimentsDir, id)
		if err != nil {
			return nil, err
		}
		if content != nil {
			reports[id] = content
		}
	}
	return reports, nil
}

func (reader *Reader) readExperiment(project *os.Root, root, directory, id string) (*Report, error) {
	experimentPath := filepath.Join(filepath.FromSlash(directory), filepath.FromSlash(id))
	experiment, err := project.OpenRoot(experimentPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", experimentPath, err)
	}
	defer experiment.Close()
	manifest := filepath.Join(experimentPath, "report.yaml")
	data, missing, err := readFile(experiment, "report.yaml", maxManifestBytes)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", manifest, err)
	}
	if missing {
		return nil, nil
	}
	report, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifest, err)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	loader := sourceLoader{root: experiment, files: make(map[string]sourceFile), reader: reader, key: filepath.Join(absolute, experimentPath)}
	if err := loader.load(report.Blocks, "blocks"); err != nil {
		return nil, fmt.Errorf("read %s: %w", manifest, err)
	}
	return report, nil
}

type sourceFile struct {
	data    []byte
	missing bool
}

type sourceLoader struct {
	root    *os.Root
	files   map[string]sourceFile
	reader  *Reader
	key     string
	bytes   int64
	counted map[string]bool
}

func (loader *sourceLoader) account(name string, size int64) error {
	if loader.counted == nil {
		loader.counted = make(map[string]bool)
	}
	if !loader.counted[name] {
		loader.counted[name] = true
		loader.bytes += size
	}
	if loader.bytes > maxSourceBytes {
		return fmt.Errorf("report sources exceed %d bytes", maxSourceBytes)
	}
	return nil
}

func (loader *sourceLoader) source(name string, limit int) (sourceFile, error) {
	if file, ok := loader.files[name]; ok {
		if len(file.data) > limit {
			return sourceFile{}, fmt.Errorf("%s exceeds %d bytes", name, limit)
		}
		return file, nil
	}
	data, missing, err := readFile(loader.root, name, limit)
	if err != nil {
		return sourceFile{}, fmt.Errorf("%s: %w", name, err)
	}
	if err := loader.account(name, int64(len(data))); err != nil {
		return sourceFile{}, err
	}
	file := sourceFile{data: data, missing: missing}
	loader.files[name] = file
	return file, nil
}

type chartSelection struct {
	X     string
	Y     []string
	Limit int
}

type chartBlock struct {
	block *Block
	where string
}

func (loader *sourceLoader) load(blocks []Block, location string) error {
	var ordered []chartBlock
	var visit func([]Block, string)
	visit = func(blocks []Block, location string) {
		for i := range blocks {
			block := &blocks[i]
			where := fmt.Sprintf("%s[%d]", location, i)
			if block.Type == "row" {
				visit(block.Blocks, where+".blocks")
			} else {
				ordered = append(ordered, chartBlock{block, where})
			}
		}
	}
	visit(blocks, location)
	weight := 0
	groups := make(map[string][]chartBlock)
	for _, item := range ordered {
		if item.block.Type == "line" {
			weight += 1 + len(item.block.Y)
			groups[item.block.Source] = append(groups[item.block.Source], item)
		}
	}
	plotLimit := maxPlotRows
	if weight > 0 {
		plotLimit = min(plotLimit, maxReportValues/weight-maxTableRows)
	}
	for _, item := range ordered {
		block, where := item.block, item.where
		if block.Type == "markdown" {
			file, err := loader.source(block.Source, maxMarkdownBytes)
			if err != nil {
				return fmt.Errorf("%s: %w", where, err)
			}
			if file.missing {
				return fmt.Errorf("%s: %s: %w", where, block.Source, os.ErrNotExist)
			}
			block.Markdown = string(file.data)
			continue
		}
		group, pending := groups[block.Source]
		if !pending {
			continue
		}
		delete(groups, block.Source)
		selections := make([]chartSelection, len(group))
		for i, chart := range group {
			selections[i] = chartSelection{chart.block.X, chart.block.Y, plotLimit}
		}
		data, err := loader.csv(block.Source, selections)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", where, block.Source, err)
		}
		for i, chart := range group {
			chart.block.Data = data[i]
			if data[i] == nil {
				chart.block.Message = "Results unavailable"
			}
		}
	}
	return nil
}

func (loader *sourceLoader) csv(name string, selections []chartSelection) ([]*Data, error) {
	file, info, missing, err := openFile(loader.root, name, maxCSVBytes)
	if err != nil {
		return nil, err
	}
	if missing {
		return make([]*Data, len(selections)), nil
	}
	defer file.Close()
	if err := loader.account(name, info.Size()); err != nil {
		return nil, err
	}
	projection, _ := json.Marshal(selections)
	key := loader.key + "\x00" + name
	reader := loader.reader
	if cached, ok := reader.cache[key]; ok && cached.projection == string(projection) && os.SameFile(info, cached.info) && info.Size() == cached.info.Size() && info.ModTime() == cached.info.ModTime() {
		return cached.data, nil
	}
	// A fixed prefix of the open descriptor keeps every projection on one snapshot.
	data, err := parseCSVStream(io.NewSectionReader(file, 0, info.Size()), selections, reader.live)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if after.Size() < info.Size() || (after.Size() == info.Size() && after.ModTime() != info.ModTime()) {
		return nil, errors.New("CSV changed while reading; waiting for a stable snapshot")
	}
	values := 0
	for _, chart := range data {
		if chart != nil {
			values += (len(chart.X) + len(chart.Table.X)) * (1 + len(chart.Series))
		}
	}
	reader.evict(key)
	if len(reader.cache) < maxCachedSources && reader.cachedValues+values <= maxCachedValues {
		reader.cache[key] = cachedCSV{info: info, projection: string(projection), data: data, values: values}
		reader.cachedValues += values
	}
	return data, nil
}

func (reader *Reader) evict(key string) {
	if old, ok := reader.cache[key]; ok {
		reader.cachedValues -= old.values
		delete(reader.cache, key)
	}
}

func cleanSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" || filepath.IsAbs(source) || strings.ContainsAny(source, "\\\x00") {
		return "", errors.New("source must be a relative file path within the experiment")
	}
	for _, component := range strings.Split(source, "/") {
		if component == ".." {
			return "", errors.New("source must not contain parent traversal (..)")
		}
	}
	clean := path.Clean(source)
	if clean == "." {
		return "", errors.New("source must name a file")
	}
	return clean, nil
}

func readFile(root *os.Root, name string, limit int) ([]byte, bool, error) {
	file, _, missing, err := openFile(root, name, limit)
	if err != nil || missing {
		return nil, missing, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > limit {
		return nil, false, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return data, false, nil
}

func openFile(root *os.Root, name string, limit int) (*os.File, os.FileInfo, bool, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		// A dangling parent symlink is an error, not a missing result.
		parts := strings.Split(name, "/")
		for i := 1; i < len(parts); i++ {
			parent := strings.Join(parts[:i], "/")
			parentInfo, parentErr := root.Lstat(parent)
			if errors.Is(parentErr, os.ErrNotExist) {
				return nil, nil, true, nil
			}
			if parentErr != nil {
				return nil, nil, false, parentErr
			}
			if parentInfo.Mode()&os.ModeSymlink != 0 {
				if _, parentErr = root.Stat(parent); parentErr != nil {
					return nil, nil, false, parentErr
				}
			}
		}
		return nil, nil, true, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		info, err = root.Stat(name)
		if err != nil {
			return nil, nil, false, err
		}
	}
	if !info.Mode().IsRegular() {
		return nil, nil, false, errors.New("source must be a regular file")
	}
	if info.Size() > int64(limit) {
		return nil, nil, false, fmt.Errorf("file exceeds %d bytes", limit)
	}
	// Nonblocking open avoids hanging if a result is replaced by a FIFO after Stat.
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, false, err
	}
	if info, err = file.Stat(); err != nil {
		file.Close()
		return nil, nil, false, err
	} else if !info.Mode().IsRegular() {
		file.Close()
		return nil, nil, false, errors.New("source must be a regular file")
	}
	if info.Size() > int64(limit) {
		file.Close()
		return nil, nil, false, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return file, info, false, nil
}
