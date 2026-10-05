// Package web renders experiment catalogs as HTML.
package web

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"net/url"
	"strings"
	"time"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

//go:embed index.html
var indexHTML string

var indexTemplate = template.Must(template.New("index").Parse(indexHTML))

type experimentView struct {
	ID, Title, CreatedAt, DateTime, RemoteURL string
}

type page struct {
	Project     string
	Experiments []experimentView
	Graph       graphView
	Live        bool
}

// PageOptions supplies display metadata and an optional experiments-directory URL.
type PageOptions struct {
	Project, RemoteURL string
	// RemoteURLs overrides the experiments-directory URL per record, including
	// an empty URL for a source without remote links.
	RemoteURLs map[string]string
	Live       bool
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
			remoteURL = strings.TrimRight(baseURL, "/") + "/" + url.PathEscape(record.ID)
		}
		data.Experiments = append(data.Experiments, experimentView{
			ID: record.ID, Title: record.Title,
			CreatedAt: created.Format("Jan 02, 2006 · 15:04:05 UTC"),
			DateTime:  created.Format(time.RFC3339Nano),
			RemoteURL: remoteURL,
		})
	}
	data.Graph = lineage(records, data.Experiments)
	var body bytes.Buffer
	if err := indexTemplate.Execute(&body, data); err != nil {
		return nil, fmt.Errorf("render experiments: %w", err)
	}
	return body.Bytes(), nil
}
