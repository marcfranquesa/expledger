package cli

import (
	"bytes"
	"context"
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

const testReport = `blocks:
  - type: markdown
    source: README.md
  - type: line
    title: Training loss
    source: metrics.csv
    x: step
    y: [loss]
`

func writeTestReport(t *testing.T, root, id, prose, csv string) {
	t.Helper()
	writeTestReportAt(t, filepath.Join(root, "experiments", id), prose, csv)
}

func writeTestReportAt(t *testing.T, dir, prose, csv string) {
	t.Helper()
	sourceWrite(t, filepath.Join(dir, "report.yaml"), testReport)
	sourceWrite(t, filepath.Join(dir, "README.md"), prose)
	sourceWrite(t, filepath.Join(dir, "metrics.csv"), csv)
}

func TestBuildAndServeReportsUseEachSourceLayout(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	first := catalog.Layout{ExperimentsDir: "research/trials", ExperimentFormat: "{date}/{slug}"}
	second := catalog.Layout{ExperimentsDir: "runs/remote", ExperimentFormat: "{date}/{slug}"}
	sourceProject(t, root, "experiments_dir: research/trials\nexperiment_format: '{date}/{slug}'\nsources: [../missing]\n")
	sourceProject(t, other, "experiments_dir: runs/remote\nexperiment_format: '{date}/{slug}'\n")
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	firstDir, err := catalog.Create(root, "same", now, catalog.CreateOptions{Layout: first})
	if err != nil {
		t.Fatal(err)
	}
	secondDir, err := catalog.Create(other, "same", now, catalog.CreateOptions{Layout: second})
	if err != nil {
		t.Fatal(err)
	}
	uniqueDir, err := catalog.Create(other, "unique", now, catalog.CreateOptions{Layout: second})
	if err != nil {
		t.Fatal(err)
	}
	writeTestReportAt(t, firstDir, "First custom notes", "step,loss\n1,5\n")
	writeTestReportAt(t, secondDir, "Second custom notes", "step,loss\n1,9\n")
	writeTestReportAt(t, uniqueDir, "Unique custom notes", "step,loss\n1,2\n")
	sourceWrite(t, filepath.Join(secondDir, "report.yaml"), "invalid losing custom report")
	const same, unique = "2026-10-06/same", "2026-10-06/unique"
	snapshot, err := loadSnapshot(root, []string{".", other})
	if err != nil || snapshot.Options.Reports[same].Blocks[0].Markdown != "First custom notes" || snapshot.Options.Reports[unique].Blocks[0].Markdown != "Unique custom notes" {
		t.Fatalf("custom reports = (%#v, %v)", snapshot.Options.Reports, err)
	}
	if err := Run(context.Background(), []string{"build"}, root, time.Time{}, Streams{}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "dist", "index.html"))
	if err != nil || !bytes.Contains(body, []byte("First custom notes")) || !bytes.Contains(body, []byte("Training loss")) || bytes.Contains(body, []byte("Unique custom notes")) || bytes.Contains(body, []byte("Second custom notes")) {
		t.Fatalf("custom local build did not render only its report: %v", err)
	}
	if err := os.Remove(filepath.Join(firstDir, "report.yaml")); err != nil {
		t.Fatal(err)
	}
	snapshot, err = loadSnapshot(root, []string{".", other})
	if err != nil || snapshot.Options.Reports[same] != nil || snapshot.Options.Reports[unique].Blocks[0].Markdown != "Unique custom notes" {
		t.Fatalf("winner without custom report = (%#v, %v)", snapshot.Options.Reports, err)
	}
	sourceWrite(t, filepath.Join(secondDir, "report.yaml"), testReport)
	snapshot, err = loadSnapshot(root, []string{other, "."})
	if err != nil || snapshot.Options.Reports[same].Blocks[0].Markdown != "Second custom notes" {
		t.Fatalf("reordered custom report = (%#v, %v)", snapshot.Options.Reports, err)
	}
}

func TestBuildReportsArePortableAndFailureKeepsSnapshot(t *testing.T) {
	root := t.TempDir()
	sourceProject(t, root, "{}")
	sourceRecord(t, root, "training", "Training", "")
	writeTestReport(t, root, "training", "## Findings\n\nA **useful** result.", "step,loss\n1,5\n2,3\n3,\n4,1\n")
	var out bytes.Buffer
	build := func() error {
		return Run(context.Background(), []string{"build"}, root, time.Time{}, Streams{Out: &out})
	}
	if err := build(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "dist", "index.html")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"View report", "Training loss", "<strong>useful</strong>", "Findings"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("report missing %q", want)
		}
	}
	for _, absent := range []string{"fetch(", "<script src=", "<link rel=\"stylesheet\""} {
		if bytes.Contains(body, []byte(absent)) {
			t.Errorf("offline report contains %q", absent)
		}
	}
	if err := os.Remove(filepath.Join(root, "experiments", "training", "metrics.csv")); err != nil {
		t.Fatal(err)
	}
	if saved, err := os.ReadFile(path); err != nil || !bytes.Equal(saved, body) {
		t.Fatalf("snapshot depends on its source: %v", err)
	}
	if err := build(); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(path)
	if err != nil || !bytes.Contains(body, []byte("Results unavailable")) {
		t.Fatalf("missing result state: %v", err)
	}
	sourceWrite(t, filepath.Join(root, "experiments", "training", "metrics.csv"), "step,wrong\n1,5\n")
	out.Reset()
	if err := build(); err == nil || !strings.Contains(err.Error(), "loss") {
		t.Fatalf("invalid column error: %v", err)
	}
	if saved, err := os.ReadFile(path); err != nil || !bytes.Equal(saved, body) || out.Len() != 0 {
		t.Fatalf("failed report build changed snapshot or printed success: %v", err)
	}
}

func TestServeReportsFollowWinningSourceAndRefresh(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	sourceProject(t, root, "{}")
	sourceProject(t, other, "{}")
	sourceRecord(t, root, "same", "First", "")
	sourceRecord(t, other, "same", "Second", "")
	sourceRecord(t, other, "unique", "Unique", "")
	writeTestReport(t, root, "same", "First notes", "step,loss\n1,5\n")
	writeTestReport(t, other, "same", "Second notes", "step,loss\n1,9\n")
	writeTestReport(t, other, "unique", "Unique notes", "step,loss\n1,2\n")
	sourceWrite(t, filepath.Join(other, "experiments", "same", "report.yaml"), "invalid losing report")
	snapshot, err := loadSnapshot(root, []string{root, other})
	if err != nil || snapshot.Options.Reports["same"].Blocks[0].Markdown != "First notes" || snapshot.Options.Reports["unique"].Blocks[0].Markdown != "Unique notes" {
		t.Fatalf("first report: %+v, %v", snapshot.Options.Reports, err)
	}
	sourceWrite(t, filepath.Join(root, "experiments", "same", "README.md"), "Updated notes")
	snapshot, err = loadSnapshot(root, []string{root, other})
	if err != nil || snapshot.Options.Reports["same"].Blocks[0].Markdown != "Updated notes" {
		t.Fatalf("refreshed report: %+v, %v", snapshot.Options.Reports, err)
	}
	sourceWrite(t, filepath.Join(other, "experiments", "same", "report.yaml"), testReport)
	snapshot, err = loadSnapshot(root, []string{other, root})
	if err != nil || snapshot.Options.Reports["same"].Blocks[0].Markdown != "Second notes" {
		t.Fatalf("reordered report: %+v, %v", snapshot.Options.Reports, err)
	}
	if err := os.Remove(filepath.Join(root, "experiments", "same", "report.yaml")); err != nil {
		t.Fatal(err)
	}
	sourceWrite(t, filepath.Join(other, "experiments", "same", "report.yaml"), "invalid losing report")
	snapshot, err = loadSnapshot(root, []string{root, other})
	if err != nil || snapshot.Options.Reports["same"] != nil || snapshot.Options.Reports["unique"].Blocks[0].Markdown != "Unique notes" {
		t.Fatalf("winner without report inherited another source: %+v, %v", snapshot.Options.Reports, err)
	}
	sourceWrite(t, filepath.Join(other, "experiments", "same", "report.yaml"), testReport)
	if err := os.RemoveAll(filepath.Join(root, "experiments", "same")); err != nil {
		t.Fatal(err)
	}
	snapshot, err = loadSnapshot(root, []string{root, other})
	if err != nil || snapshot.Options.Reports["same"].Blocks[0].Markdown != "Second notes" {
		t.Fatalf("removed winner report: %+v, %v", snapshot.Options.Reports, err)
	}
}

func TestReportHandlerRefreshAndFileIsolation(t *testing.T) {
	root := t.TempDir()
	sourceProject(t, root, "{}")
	sourceRecord(t, root, "training", "Training", "")
	writeTestReport(t, root, "training", "Initial findings", "step,loss\n1,5\n")
	handler := web.NewLiveHandler(func() (web.Snapshot, error) { return loadSnapshot(root, nil) })
	request := func(path string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		return response
	}
	if response := request("/"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Initial findings") {
		t.Fatalf("initial report: %d %s", response.Code, response.Body)
	}
	sourceWrite(t, filepath.Join(root, "experiments", "training", "README.md"), "Updated findings")
	if response := request("/snapshot"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Updated findings") {
		t.Fatalf("report refresh: %d %s", response.Code, response.Body)
	}
	for _, path := range []string{"/experiments/training/report.yaml", "/experiments/training/metrics.csv", "/experiments/training/README.md"} {
		if response := request(path); response.Code != http.StatusNotFound {
			t.Errorf("report source served at %s: %d", path, response.Code)
		}
	}
	sourceWrite(t, filepath.Join(root, "experiments", "training", "report.yaml"), "blocks: [{type: unsupported}]\n")
	if response := request("/snapshot"); response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "Updated findings") {
		t.Fatalf("invalid report returned partial page: %d %s", response.Code, response.Body)
	}
}
