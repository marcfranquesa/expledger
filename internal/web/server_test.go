package web_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/experiment"
	"github.com/marcfranquesa/expledger/internal/web"
	"go.yaml.in/yaml/v3"
)

func TestExperiments(t *testing.T) {
	root := t.TempDir()
	writeExperiment(t, root, "z-earlier", experiment.Record{
		ID: "z-earlier", Title: "Earlier experiment",
		CreatedAt: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC),
	})
	writeExperiment(t, root, "actual folder & notes", experiment.Record{
		ID: "actual folder & notes", Title: `<script>alert("x")</script> & trial`,
		CreatedAt: time.Date(2026, 9, 24, 9, 12, 3, 0, time.FixedZone("EDT", -4*60*60)),
		BasedOn:   []string{"hidden-parent"},
		Extra: map[string]yaml.Node{
			"note": {Kind: yaml.ScalarNode, Tag: "!!str", Value: "hidden-metadata"},
		},
		Body: []byte("\n# Hidden Markdown body\n"),
	})
	handler := web.NewHandler(root, "https://github.com/example/project", "research/next")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	body := response.Body.String()
	for _, want := range []string{
		"&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt; &amp; trial",
		`href="https://github.com/example/project/tree/research%2Fnext/experiments/actual%20folder%20&amp;%20notes"`,
		`datetime="2026-09-24T13:12:03Z"`,
		"Sep 24, 2026 · 13:12:03 UTC",
		"research/next",
		filepath.Base(root),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing %q", want)
		}
	}
	for _, unwanted := range []string{"<script>", "Hidden Markdown body", "hidden-metadata", "hidden-parent", "/experiments/a-newer"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("response contains %q", unwanted)
		}
	}
	if newer, earlier := strings.Index(body, "actual folder &amp; notes"), strings.Index(body, "z-earlier"); newer < 0 || earlier < 0 || newer >= earlier {
		t.Error("experiments are not ordered newest first")
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
}

func TestRefreshReadsCurrentFiles(t *testing.T) {
	root := t.TempDir()
	record := experiment.Record{
		ID: "one", Title: "First title",
		CreatedAt: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC),
	}
	writeExperiment(t, root, "one", record)
	handler := web.NewHandler(root, "https://github.com/example/project", "main")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "First title") {
		t.Fatalf("initial response: %d %s", response.Code, response.Body)
	}
	record.Title = "Updated title"
	writeExperiment(t, root, "one", record)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Updated title") || strings.Contains(response.Body.String(), "First title") {
		t.Fatalf("refresh did not reflect changed files: %d %s", response.Code, response.Body)
	}
}

func TestEmptyCatalogAndRoutes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("Private repository file"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := web.NewHandler(root, "https://github.com/example/project", "main")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "No experiments yet") || !strings.Contains(response.Body.String(), "expledger new my-idea") {
		t.Fatalf("empty catalog response: %d %s", response.Code, response.Body)
	}
	for _, path := range []string{"/README.md", "/experiments/one/README.md", "/missing"} {
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

func writeExperiment(t *testing.T, root, folder string, record experiment.Record) {
	t.Helper()
	dir := filepath.Join(root, "experiments", folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := record.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
