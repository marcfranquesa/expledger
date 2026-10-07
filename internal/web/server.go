package web

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

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
	var mutex sync.Mutex
	var revision string
	var body []byte
	render := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		mutex.Lock()
		defer mutex.Unlock()
		snapshot, err := load()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		snapshot.Options.Live = true
		encoded, err := json.Marshal(snapshot)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		next := fmt.Sprintf(`"%x"`, sha256.Sum256(encoded))
		if next != revision {
			updated, err := Render(snapshot.Records, snapshot.Options)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			body, revision = updated, next
		}
		w.Header().Set("ETag", revision)
		if r.Header.Get("If-None-Match") == revision {
			w.WriteHeader(http.StatusNotModified)
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
