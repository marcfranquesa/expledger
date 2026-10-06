// Package report loads an experiment's optional report layout and its local sources.
package report

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/marcfranquesa/expledger/internal/catalog"
)

const (
	maxManifestBytes = 256 << 10
	maxMarkdownBytes = 1 << 20
	maxCSVBytes      = 8 << 20
	maxSourceBytes   = 16 << 20
	maxBlocks        = 64
	maxSeries        = 8
	maxCSVRows       = 10000
	maxReportValues  = 100000
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
	X      []float64 `json:"x"`
	Series []Series  `json:"series"`
}

type Series struct {
	Name   string     `json:"name"`
	Values []*float64 `json:"values"`
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
		content, err := readExperiment(project, layout.ExperimentsDir, id)
		if err != nil {
			return nil, err
		}
		if content != nil {
			reports[id] = content
		}
	}
	return reports, nil
}

func readExperiment(project *os.Root, directory, id string) (*Report, error) {
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
	loader := sourceLoader{root: experiment, files: make(map[string]sourceFile)}
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
	root        *os.Root
	files       map[string]sourceFile
	bytes       int
	chartValues int
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
	loader.bytes += len(data)
	if loader.bytes > maxSourceBytes {
		return sourceFile{}, fmt.Errorf("report sources exceed %d bytes", maxSourceBytes)
	}
	file := sourceFile{data: data, missing: missing}
	loader.files[name] = file
	return file, nil
}

func (loader *sourceLoader) load(blocks []Block, location string) error {
	for i := range blocks {
		block := &blocks[i]
		where := fmt.Sprintf("%s[%d]", location, i)
		if block.Type == "row" {
			if err := loader.load(block.Blocks, where+".blocks"); err != nil {
				return err
			}
			continue
		}
		limit := maxMarkdownBytes
		if block.Type == "line" {
			limit = maxCSVBytes
		}
		file, err := loader.source(block.Source, limit)
		if err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		if block.Type == "markdown" {
			if file.missing {
				return fmt.Errorf("%s: %s: %w", where, block.Source, os.ErrNotExist)
			}
			block.Markdown = string(file.data)
			continue
		}
		if !file.missing {
			block.Data, err = parseCSV(file.data, block.X, block.Y)
			if err != nil {
				return fmt.Errorf("%s: %s: %w", where, block.Source, err)
			}
		}
		if block.Data == nil {
			block.Message = "Results unavailable"
			continue
		}
		loader.chartValues += len(block.Data.X) * (1 + len(block.Data.Series))
		if loader.chartValues > maxReportValues {
			return fmt.Errorf("%s: chart data exceeds %d selected values per report", where, maxReportValues)
		}
	}
	return nil
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
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		// A dangling parent symlink is an error, not a missing result.
		parts := strings.Split(name, "/")
		for i := 1; i < len(parts); i++ {
			parent := strings.Join(parts[:i], "/")
			parentInfo, parentErr := root.Lstat(parent)
			if errors.Is(parentErr, os.ErrNotExist) {
				return nil, true, nil
			}
			if parentErr != nil {
				return nil, false, parentErr
			}
			if parentInfo.Mode()&os.ModeSymlink != 0 {
				if _, parentErr = root.Stat(parent); parentErr != nil {
					return nil, false, parentErr
				}
			}
		}
		return nil, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		info, err = root.Stat(name)
		if err != nil {
			return nil, false, err
		}
	}
	if !info.Mode().IsRegular() {
		return nil, false, errors.New("source must be a regular file")
	}
	if info.Size() > int64(limit) {
		return nil, false, fmt.Errorf("file exceeds %d bytes", limit)
	}
	// Nonblocking open avoids hanging if a result is replaced by a FIFO after Stat.
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	if info, err = file.Stat(); err != nil {
		return nil, false, err
	} else if !info.Mode().IsRegular() {
		return nil, false, errors.New("source must be a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > limit {
		return nil, false, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return data, false, nil
}
