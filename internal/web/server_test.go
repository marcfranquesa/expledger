package web_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marcfranquesa/expledger/internal/web"
)

func TestExperiments(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "project")
	handler := web.NewHandler(root, web.PageOptions{Project: filepath.Base(root), RepositoryURL: "https://github.com/example/project", Branch: "research/next"})
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
		"research/next",
		filepath.Base(root),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing %q", want)
		}
	}
	for _, unwanted := range []string{"fixture body", "Extra fixture metadata", "based_on"} {
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
}

func TestEscapesMetadataAndFolderURL(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "project"))); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "experiments", "actual folder & notes")
	if err := os.Rename(filepath.Join(root, "experiments", "20260924-long-title"), folder); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(folder, "README.md")
	data, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "title:") {
			lines[i] = `title: '<script>alert("x")</script> & trial'`
		}
		if strings.HasPrefix(line, "id:") {
			lines[i] = "id: actual folder & notes"
		}
	}
	if err := os.WriteFile(readme, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := web.NewHandler(root, web.PageOptions{Project: filepath.Base(root), RepositoryURL: "https://github.com/example/project", Branch: "research/next"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK ||
		!strings.Contains(body, "&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt; &amp; trial") ||
		!strings.Contains(body, `href="https://github.com/example/project/tree/research%2Fnext/experiments/actual%20folder%20&amp;%20notes"`) ||
		strings.Contains(body, "<script>") || strings.Contains(body, "/experiments/20260924-long-title") {
		t.Fatalf("incorrect escaping or folder URL: %d %s", response.Code, body)
	}
}

func TestRefreshReadsCurrentFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "project"))); err != nil {
		t.Fatal(err)
	}
	handler := web.NewHandler(root, web.PageOptions{Project: filepath.Base(root), RepositoryURL: "https://github.com/example/project", Branch: "main"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Baseline model") {
		t.Fatalf("initial response: %d %s", response.Code, response.Body)
	}
	readme := filepath.Join(root, "experiments", "20260924-baseline", "README.md")
	data, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), "title: Baseline model", "title: Updated baseline", 1)
	if err := os.WriteFile(readme, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Updated baseline") || strings.Contains(response.Body.String(), "Baseline model") {
		t.Fatalf("refresh did not reflect changed files: %d %s", response.Code, response.Body)
	}
}

func TestRefreshRecoversAfterREADMERepair(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "experiments", "example")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("invalid README"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := web.NewHandler(root, web.PageOptions{
		Project: "Example", RepositoryURL: "https://github.com/example/project", Branch: "main",
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "README.md") {
		t.Fatalf("invalid README response: %d %s", response.Code, response.Body)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("invalid README response may be cached")
	}
	content := "---\nid: example\ntitle: Repaired example\ncreated_at: 2026-09-24T12:00:00Z\n---\n"
	if err := os.WriteFile(readme, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Repaired example") {
		t.Fatalf("repaired README response: %d %s", response.Code, response.Body)
	}
}

func TestEmptyCatalogAndRoutes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("Private repository file"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := web.NewHandler(root, web.PageOptions{Project: filepath.Base(root), RepositoryURL: "https://github.com/example/project", Branch: "main"})
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
