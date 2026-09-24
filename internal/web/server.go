// Package web serves the local experiment browser.
package web

import (
	"bytes"
	_ "embed"
	"html/template"
	"net/http"
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

// NewHandler reads the catalog on each request. GitHub links use the repository
// and branch selected when the server starts.
func NewHandler(root, repositoryURL, branch string) http.Handler {
	remoteURL := repositoryURL + "/tree/" + url.PathEscape(branch) + "/experiments/"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		records, err := catalog.List(root)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data := page{Project: filepath.Base(root), Branch: branch}
		for _, record := range records {
			created := record.CreatedAt.UTC()
			data.Experiments = append(data.Experiments, experimentView{
				ID: record.ID, Title: record.Title,
				CreatedAt: created.Format("Jan 02, 2006 · 15:04:05 UTC"),
				DateTime:  created.Format(time.RFC3339Nano),
				GitHubURL: remoteURL + url.PathEscape(record.ID),
			})
		}
		var body bytes.Buffer
		if err := indexTemplate.Execute(&body, data); err != nil {
			http.Error(w, "Unable to render experiments", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodGet {
			_, _ = w.Write(body.Bytes())
		}
	})
}
