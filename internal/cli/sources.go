package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/marcfranquesa/expledger/internal/web"
)

// loadServeSnapshot resolves all sources afresh; no partially read catalog is returned.
func loadServeSnapshot(root string, override []string) (web.Snapshot, error) {
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
	snapshot := web.Snapshot{Options: web.PageOptions{Project: filepath.Base(root), RemoteURLs: make(map[string]string)}}
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
		for _, record := range records {
			if _, exists := snapshot.Options.RemoteURLs[record.ID]; exists {
				continue
			}
			snapshot.Records = append(snapshot.Records, record)
			snapshot.Options.RemoteURLs[record.ID] = sourceConfig.RemoteURL
		}
	}
	sort.SliceStable(snapshot.Records, func(i, j int) bool { return snapshot.Records[i].CreatedAt.After(snapshot.Records[j].CreatedAt) })
	return snapshot, nil
}
