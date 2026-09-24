package web

import "net/http"

// NewHandler reads the catalog on each request. remoteURLPrefix includes the
// trailing slash before the experiment ID; branch is displayed in the page.
func NewHandler(root, remoteURLPrefix, branch string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		body, err := Render(root, remoteURLPrefix, branch)
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
