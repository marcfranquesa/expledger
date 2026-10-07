package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/marcfranquesa/expledger/internal/report"
	"github.com/marcfranquesa/expledger/internal/web"
)

// loadSnapshot resolves all sources afresh; no partially read catalog is returned.
func loadSnapshot(root string, override []string) (web.Snapshot, error) {
	return loadReportsSnapshot(root, override, report.NewReader(false))
}

func liveSnapshotLoader(root string, override []string) func() (web.Snapshot, error) {
	reader := report.NewReader(true)
	return func() (web.Snapshot, error) { return loadReportsSnapshot(root, override, reader) }
}

func loadReportsSnapshot(root string, override []string, reader *report.Reader) (web.Snapshot, error) {
	config, err := readProjectConfig(filepath.Join(root, "expledger.yaml"))
	if err != nil {
		return web.Snapshot{}, fmt.Errorf("read project %s: %w", root, err)
	}
	sources := config.Sources
	if override != nil {
		sources = override
	}
	if sources == nil {
		sources = []string{"."}
	}
	if len(sources) == 0 {
		return web.Snapshot{}, fmt.Errorf("sources must contain at least one project-root path")
	}
	snapshot := web.Snapshot{Options: web.PageOptions{Project: filepath.Base(root), RemoteURLs: make(map[string]string), Reports: make(map[string]*report.Report)}}
	for _, source := range sources {
		if strings.TrimSpace(source) == "" {
			return web.Snapshot{}, fmt.Errorf("source must be a nonempty project-root path")
		}
		path := source
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		sourceConfig := config
		if filepath.Clean(path) != filepath.Clean(root) {
			sourceConfig, err = readProjectConfig(filepath.Join(path, "expledger.yaml"))
			if err != nil {
				return web.Snapshot{}, fmt.Errorf("read source %s: %w", path, err)
			}
		}
		records, err := catalog.List(path, sourceConfig.Layout)
		if err != nil {
			return web.Snapshot{}, fmt.Errorf("read source %s: %w", path, err)
		}
		var ids []string
		for _, record := range records {
			if _, exists := snapshot.Options.RemoteURLs[record.ID]; exists {
				continue
			}
			ids = append(ids, record.ID)
			snapshot.Records = append(snapshot.Records, record)
			snapshot.Options.RemoteURLs[record.ID] = sourceConfig.RemoteURL
		}
		reports, err := reader.ReadMany(path, ids, sourceConfig.Layout)
		if err != nil {
			return web.Snapshot{}, fmt.Errorf("read source %s: %w", path, err)
		}
		for id, content := range reports {
			snapshot.Options.Reports[id] = content
		}
	}
	sort.SliceStable(snapshot.Records, func(i, j int) bool { return snapshot.Records[i].CreatedAt.After(snapshot.Records[j].CreatedAt) })
	return snapshot, nil
}
