package web

import (
	"net/http"

	"github.com/marcfranquesa/expledger/internal/catalog"
)

// NewHandler reads the catalog on each request using fixed page options.
func NewHandler(root string, options PageOptions) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		records, err := catalog.List(root)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		body, err := Render(records, options)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodGet {
			_, _ = w.Write(body)
		}
	})
	return mux
}
