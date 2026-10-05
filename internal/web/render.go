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
}

// PageOptions supplies display metadata and an optional experiments-directory URL.
type PageOptions struct {
	Project, RemoteURL string
}

// Render renders a complete HTML page with records in their supplied order.
func Render(records []experiment.Record, options PageOptions) ([]byte, error) {
	data := page{Project: options.Project}
	for _, record := range records {
		created := record.CreatedAt.UTC()
		var remoteURL string
		if options.RemoteURL != "" {
			remoteURL = strings.TrimRight(options.RemoteURL, "/") + "/" + url.PathEscape(record.ID)
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
