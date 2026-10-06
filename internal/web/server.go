package web

import (
	"net/http"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

// Snapshot holds a complete catalog and the options belonging to that read.
type Snapshot struct {
	Records []experiment.Record
	Options PageOptions
}

// NewLiveHandler uses the same fresh snapshot for initial pages and polling.
func NewLiveHandler(load func() (Snapshot, error)) http.Handler {
	mux := http.NewServeMux()
	render := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		snapshot, err := load()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		snapshot.Options.Live = true
		body, err := Render(snapshot.Records, snapshot.Options)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodGet {
			_, _ = w.Write(body)
		}
	}
	mux.HandleFunc("GET /{$}", render)
	mux.HandleFunc("GET /snapshot", render)
	return mux
}
