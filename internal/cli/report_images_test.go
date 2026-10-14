package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/marcfranquesa/expledger/internal/web"
)

const imageReport = `blocks:
  - type: row
    blocks:
      - type: image
        source: results/plot.png
        alt: Measured results
        title: Validation plot
      - type: markdown
        source: README.md
`

func writeReportPNG(t *testing.T, dir string, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(1, 1, color.RGBA{R: shade, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "results"), 0o755); err != nil {
		t.Fatal(err)
	}
	sourceWrite(t, filepath.Join(dir, "results", "plot.png"), encoded.String())
	return encoded.Bytes()
}

func imageDataURL(data []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

func TestBuildImageReportIsPortableAndFailureKeepsSnapshot(t *testing.T) {
	root := t.TempDir()
	sourceProject(t, root, "experiments_dir: trials\nexperiment_format: '{date}/{slug}'\n")
	dir, err := catalog.Create(root, "images", time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC), catalog.CreateOptions{
		Layout: catalog.Layout{ExperimentsDir: "trials", ExperimentFormat: "{date}/{slug}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceWrite(t, filepath.Join(dir, "report.yaml"), imageReport)
	sourceWrite(t, filepath.Join(dir, "README.md"), "## Image findings\n\nThe image accompanies the notes.")
	data := writeReportPNG(t, dir, 100)
	build := func() error {
		return Run(context.Background(), []string{"build"}, root, time.Time{}, Streams{})
	}
	if err := build(); err != nil {
		t.Fatal(err)
	}
	snapshotPath := filepath.Join(root, "dist", "index.html")
	snapshot, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{imageDataURL(data), `alt="Measured results"`, "Validation plot", "Image findings"} {
		if !bytes.Contains(snapshot, []byte(want)) {
			t.Errorf("offline image report missing %q", want)
		}
	}
	if err := os.Remove(filepath.Join(dir, "results", "plot.png")); err != nil {
		t.Fatal(err)
	}
	if saved, err := os.ReadFile(snapshotPath); err != nil || !bytes.Equal(saved, snapshot) {
		t.Fatalf("snapshot depends on original image: %v", err)
	}
	if err := build(); err != nil {
		t.Fatal(err)
	}
	snapshot, err = os.ReadFile(snapshotPath)
	if err != nil || !bytes.Contains(snapshot, []byte("Image unavailable")) {
		t.Fatalf("missing image placeholder: %v", err)
	}
	sourceWrite(t, filepath.Join(dir, "results", "plot.png"), "not an image")
	if err := build(); err == nil || !strings.Contains(err.Error(), "results/plot.png") {
		t.Fatalf("corrupt image build error = %v", err)
	}
	if saved, err := os.ReadFile(snapshotPath); err != nil || !bytes.Equal(saved, snapshot) {
		t.Fatalf("failed image build changed prior snapshot: %v", err)
	}
}

func TestServeImageReportsRefreshWinningSourceAndKeepArtifactsPrivate(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	for _, project := range []string{root, other} {
		sourceProject(t, project, "{}")
		sourceRecord(t, project, "same", "Images", "")
		dir := filepath.Join(project, "experiments", "same")
		sourceWrite(t, filepath.Join(dir, "report.yaml"), imageReport)
		sourceWrite(t, filepath.Join(dir, "README.md"), "Image findings")
	}
	dir := filepath.Join(root, "experiments", "same")
	first := writeReportPNG(t, dir, 100)
	// The losing source must not supply artifacts or break the winning report.
	writeReportPNG(t, filepath.Join(other, "experiments", "same"), 50)
	sourceWrite(t, filepath.Join(other, "experiments", "same", "results", "plot.png"), "invalid losing image")
	handler := web.NewLiveHandler(func() (web.Snapshot, error) { return loadSnapshot(root, []string{root, other}) })
	request := func(path string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		return response
	}
	if response := request("/"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), imageDataURL(first)) {
		t.Fatalf("initial image report: status %d", response.Code)
	}
	second := writeReportPNG(t, dir, 200)
	if response := request("/snapshot"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), imageDataURL(second)) || strings.Contains(response.Body.String(), imageDataURL(first)) {
		t.Fatalf("image replacement did not refresh: status %d", response.Code)
	}
	for _, path := range []string{"/results/plot.png", "/experiments/same/results/plot.png"} {
		if response := request(path); response.Code != http.StatusNotFound {
			t.Errorf("source image served at %s: status %d", path, response.Code)
		}
	}
	sourceWrite(t, filepath.Join(dir, "results", "plot.png"), "corrupt refresh")
	if response := request("/snapshot"); response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "<article") {
		t.Fatalf("invalid image returned partial snapshot: status %d", response.Code)
	}
	writeReportPNG(t, dir, 200)
	if response := request("/snapshot"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), imageDataURL(second)) {
		t.Fatalf("image report did not recover: status %d", response.Code)
	}
}
