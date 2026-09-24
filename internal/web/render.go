// Package web renders experiment catalogs as HTML.
package web

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"net/url"
	"path/filepath"
	"time"

	"github.com/marcfranquesa/expledger/internal/catalog"
)

//go:embed index.html
var indexHTML string

var indexTemplate = template.Must(template.New("index").Parse(indexHTML))

type experimentView struct {
	ID, Title, CreatedAt, DateTime, GitHubURL string
}

type page struct {
	Project, Branch string
	Experiments     []experimentView
}

// Render reads the catalog and renders a complete HTML page. remoteURLPrefix
// includes the trailing slash before the experiment ID.
func Render(root, remoteURLPrefix, branch string) ([]byte, error) {
	records, err := catalog.List(root)
	if err != nil {
		return nil, err
	}
	data := page{Project: filepath.Base(root), Branch: branch}
	for _, record := range records {
		created := record.CreatedAt.UTC()
		data.Experiments = append(data.Experiments, experimentView{
			ID: record.ID, Title: record.Title,
			CreatedAt: created.Format("Jan 02, 2006 · 15:04:05 UTC"),
			DateTime:  created.Format(time.RFC3339Nano),
			GitHubURL: remoteURLPrefix + url.PathEscape(record.ID),
		})
	}
	var body bytes.Buffer
	if err := indexTemplate.Execute(&body, data); err != nil {
		return nil, fmt.Errorf("render experiments: %w", err)
	}
	return body.Bytes(), nil
}
