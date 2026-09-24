// Package web renders experiment catalogs as HTML.
package web

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"net/url"
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
	Project, Branch string
	Experiments     []experimentView
}

// PageOptions supplies display metadata and an optional normalized repository URL.
type PageOptions struct {
	Project, RepositoryURL, Branch string
}

// Render renders a complete HTML page with records in their supplied order.
func Render(records []experiment.Record, options PageOptions) ([]byte, error) {
	data := page{Project: options.Project, Branch: options.Branch}
	for _, record := range records {
		created := record.CreatedAt.UTC()
		var remoteURL string
		if options.RepositoryURL != "" && options.Branch != "" {
			remoteURL = options.RepositoryURL + "/tree/" + url.PathEscape(options.Branch) + "/experiments/" + url.PathEscape(record.ID)
		}
		data.Experiments = append(data.Experiments, experimentView{
			ID: record.ID, Title: record.Title,
			CreatedAt: created.Format("Jan 02, 2006 · 15:04:05 UTC"),
			DateTime:  created.Format(time.RFC3339Nano),
			RemoteURL: remoteURL,
		})
	}
	var body bytes.Buffer
	if err := indexTemplate.Execute(&body, data); err != nil {
		return nil, fmt.Errorf("render experiments: %w", err)
	}
	return body.Bytes(), nil
}
