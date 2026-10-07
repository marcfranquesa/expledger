// Package web renders experiment catalogs as HTML.
package web

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"strings"
	"time"

	"github.com/marcfranquesa/expledger/internal/experiment"
	"github.com/marcfranquesa/expledger/internal/report"
)

type experimentView struct {
	ID, Title, CreatedAt, DateTime, RemoteURL, ReportURL string
}

type page struct {
	Project     string
	Experiments []experimentView
	Graph       graphView
	Live        bool
	Reports     []reportView
	Styles      []template.CSS
	Scripts     []template.JS
}

// PageOptions supplies display metadata and an optional experiments-directory URL.
type PageOptions struct {
	Project, RemoteURL string
	// RemoteURLs overrides the experiments-directory URL per record, including
	// an empty URL for a source without remote links.
	RemoteURLs map[string]string
	Live       bool
	Reports    map[string]*report.Report
}

// Render renders a complete HTML page with records in their supplied order.
func Render(records []experiment.Record, options PageOptions) ([]byte, error) {
	data := page{Project: options.Project, Live: options.Live}
	for _, record := range records {
		created := record.CreatedAt.UTC()
		baseURL := options.RemoteURL
		if sourceURL, ok := options.RemoteURLs[record.ID]; ok {
			baseURL = sourceURL
		}
		var remoteURL string
		if baseURL != "" {
			segments := strings.Split(record.ID, "/")
			for i, segment := range segments {
				segments[i] = url.PathEscape(segment)
			}
			remoteURL = strings.TrimRight(baseURL, "/") + "/" + strings.Join(segments, "/")
		}
		data.Experiments = append(data.Experiments, experimentView{
			ID: record.ID, Title: record.Title,
			CreatedAt: created.Format("Jan 02, 2006 · 15:04:05 UTC"),
			DateTime:  created.Format(time.RFC3339Nano),
			RemoteURL: remoteURL,
		})
		if source := options.Reports[record.ID]; source != nil {
			view := &data.Experiments[len(data.Experiments)-1]
			view.ReportURL = "#" + reportAnchor(record.ID)
			prepared, err := prepareReport(*view, source)
			if err != nil {
				return nil, fmt.Errorf("render report %s: %w", record.ID, err)
			}
			data.Reports = append(data.Reports, prepared)
		}
	}
	data.Styles, data.Scripts = pageAssets(options.Live)
	data.Graph = lineage(records, data.Experiments)
	var body bytes.Buffer
	if err := indexTemplate.Execute(&body, data); err != nil {
		return nil, fmt.Errorf("render experiments: %w", err)
	}
	return body.Bytes(), nil
}
