package web_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/marcfranquesa/expledger/internal/experiment"
	"github.com/marcfranquesa/expledger/internal/web"
)

func catalogHandler(root string, options web.PageOptions) http.Handler {
	return web.NewLiveHandler(func() (web.Snapshot, error) {
		records, err := catalog.List(root, catalog.Layout{})
		return web.Snapshot{Records: records, Options: options}, err
	})
}

func TestExperiments(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "project")
	handler := catalogHandler(root, web.PageOptions{Project: filepath.Base(root), RemoteURL: "https://github.com/example/project/tree/research%2Fnext/experiments/"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	body := response.Body.String()
	for _, want := range []string{
		"Unicode &amp; HTML: comparing café embeddings with α &lt; β across a deliberately long experiment title",
		`href="https://github.com/example/project/tree/research%2Fnext/experiments/20260924-long-title"`,
		`datetime="2026-09-24T11:45:00.123456789Z"`,
		"Sep 24, 2026 · 11:45:00 UTC",
		`datetime="2026-09-24T10:30:00Z"`,
		"Sep 24, 2026 · 10:30:00 UTC",
		filepath.Base(root),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing %q", want)
		}
	}
	for _, unwanted := range []string{"fixture body", "Extra fixture metadata", "based_on:"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("response contains %q", unwanted)
		}
	}
	position := -1
	for _, id := range []string{"20260924-long-title", "20260924-variant", "20260924-baseline"} {
		next := strings.Index(body, id)
		if next <= position {
			t.Fatalf("experiment %s is missing or not ordered newest first", id)
		}
		position = next
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	for _, filename := range []string{"README.md", "experiment.yaml"} {
		response := httptest.NewRecorder()
		path := "/experiments/20260924-baseline/" + filename
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("experiment file %s was served: status %d", path, response.Code)
		}
	}
}

func TestRefreshReadsCurrentFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "project"))); err != nil {
		t.Fatal(err)
	}
	handler := catalogHandler(root, web.PageOptions{Project: filepath.Base(root), RemoteURL: "https://github.com/example/project/tree/main/experiments/"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Baseline model") {
		t.Fatalf("initial response: %d %s", response.Code, response.Body)
	}
	metadata := filepath.Join(root, "experiments", "20260924-baseline", "experiment.yaml")
	data, err := os.ReadFile(metadata)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), "title: Baseline model", "title: Updated baseline", 1)
	if err := os.WriteFile(metadata, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Updated baseline") || strings.Contains(response.Body.String(), "Baseline model") {
		t.Fatalf("refresh did not reflect changed files: %d %s", response.Code, response.Body)
	}
}

func TestRefreshRecoversAfterMetadataRepair(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "experiments", "example")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(dir, "experiment.yaml")
	if err := os.WriteFile(metadata, []byte("invalid metadata"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := catalogHandler(root, web.PageOptions{
		Project: "Example", RemoteURL: "https://github.com/example/project/tree/main/experiments/",
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "experiment.yaml") {
		t.Fatalf("invalid metadata response: %d %s", response.Code, response.Body)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("invalid metadata response may be cached")
	}
	content := "schema: expledger/v1\nid: example\ntitle: Repaired example\ncreated_at: 2026-09-24T12:00:00Z\n"
	if err := os.WriteFile(metadata, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Repaired example") {
		t.Fatalf("repaired metadata response: %d %s", response.Code, response.Body)
	}
}

func TestEmptyCatalogAndRoutes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("Private repository file"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := catalogHandler(root, web.PageOptions{Project: filepath.Base(root), RemoteURL: "https://github.com/example/project/tree/main/experiments/"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "No experiments yet") || !strings.Contains(response.Body.String(), "expledger new my-idea") {
		t.Fatalf("empty catalog response: %d %s", response.Code, response.Body)
	}
	for _, path := range []string{"/README.md", "/experiments/one/README.md", "/experiments/one/experiment.yaml", "/missing"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "Private repository file") {
				t.Fatalf("unexpected route response: %d %s", response.Code, response.Body)
			}
		})
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodHead, "/", nil))
	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Fatalf("HEAD response: %d %s", response.Code, response.Body)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST response: %d %s", response.Code, response.Body)
	}
}

// A failed read must not return a partially rendered page, and the next request
// must invoke the loader again rather than retaining a failed snapshot.
func TestLiveSnapshotReadErrorAndRecovery(t *testing.T) {
	calls := 0
	handler := web.NewLiveHandler(func() (web.Snapshot, error) {
		calls++
		if calls == 2 {
			return web.Snapshot{Records: []experiment.Record{{ID: "partial", Title: "Partial result"}}}, errors.New("unreadable source")
		}
		title := "Before"
		remote := "https://example.com/before"
		if calls > 2 {
			title = "After"
			remote = "https://example.com/after"
		}
		return web.Snapshot{
			Records: []experiment.Record{{ID: "one", Title: title}},
			Options: web.PageOptions{Project: "Live", RemoteURLs: map[string]string{"one": remote}},
		}, nil
	})
	for i, route := range []string{"/", "/snapshot", "/snapshot"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("snapshot response may be cached")
		}
		if i == 1 {
			if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "Partial result") {
				t.Fatalf("failed snapshot returned partial content: %d %s", response.Code, response.Body)
			}
			continue
		}
		want := "https://example.com/before/one"
		if i == 2 {
			want = "https://example.com/after/one"
		}
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), want) || !strings.Contains(response.Body.String(), "fetch('/snapshot'") {
			t.Fatalf("snapshot response: %d %s", response.Code, response.Body)
		}
	}
	if calls != 3 {
		t.Fatalf("loader called %d times", calls)
	}
}
