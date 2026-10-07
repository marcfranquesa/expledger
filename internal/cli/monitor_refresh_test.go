package cli

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marcfranquesa/expledger/internal/web"
)

func monitoringCSV(t *testing.T, path string, rows int) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := bufio.NewWriter(file)
	fmt.Fprintln(writer, "step,loss")
	for i := range rows {
		fmt.Fprintf(writer, "%d,%d\n", i, i%101)
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLiveMonitoringRefreshLifecycle(t *testing.T) {
	root := t.TempDir()
	sourceProject(t, root, "{}")
	sourceRecord(t, root, "training", "Training", "")
	writeTestReport(t, root, "training", "Live findings", "step,loss\n0,0\n")
	path := filepath.Join(root, "experiments", "training", "metrics.csv")
	monitoringCSV(t, path, 20000)
	initial, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	load := liveSnapshotLoader(root, nil)
	handler := web.NewLiveHandler(load)
	request := func(revision string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
		request.Header.Set("If-None-Match", revision)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	assertRows := func(want int) {
		t.Helper()
		snapshot, err := load()
		if err != nil {
			t.Fatal(err)
		}
		data := snapshot.Options.Reports["training"].Blocks[1].Data
		if data.TotalRows != want || len(data.X) > 512 || len(data.Table.X) != min(want, 100) {
			t.Fatalf("rows = %d, plot = %d, table = %d; want %d with bounded display", data.TotalRows, len(data.X), len(data.Table.X), want)
		}
	}
	assertRows(20000)
	response := request("")
	if response.Code != http.StatusOK || response.Body.Len() > 300000 || strings.Count(response.Body.String(), "<tbody>") != 1 || strings.Count(response.Body.String(), "<td>") != 200 {
		t.Fatalf("initial bounded response: status %d, bytes %d", response.Code, response.Body.Len())
	}
	revision := response.Header().Get("ETag")
	if revision == "" {
		t.Fatal("missing live revision")
	}
	for range 10 {
		if response := request(revision); response.Code != http.StatusNotModified || response.Body.Len() != 0 {
			t.Fatalf("unchanged refresh: %d, %d bytes", response.Code, response.Body.Len())
		}
	}
	if contents, err := os.ReadFile(path); err != nil || sha256.Sum256(contents) != sha256.Sum256(initial) {
		t.Fatal("monitoring changed the original log")
	}
	appendLog := func(text string) {
		t.Helper()
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString(text); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	appendLog("20000,12")
	assertRows(20000)
	if response := request(revision); response.Code != http.StatusNotModified {
		t.Fatalf("unfinished row changed the snapshot: %d", response.Code)
	}
	appendLog("\n")
	assertRows(20001)
	response = request(revision)
	if response.Code != http.StatusOK || response.Header().Get("ETag") == revision {
		t.Fatal("published append did not update the snapshot")
	}
	revision = response.Header().Get("ETag")
	appendLog("20001,NaN\n")
	if response := request(revision); response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "finite") {
		t.Fatalf("completed invalid row: %d %s", response.Code, response.Body)
	}
	// Repair by atomically publishing a complete log without touching the old inode.
	temporary := filepath.Join(filepath.Dir(path), "next.csv")
	monitoringCSV(t, temporary, 20002)
	if err := os.Rename(temporary, path); err != nil {
		t.Fatal(err)
	}
	assertRows(20002)
	if response := request(revision); response.Code != http.StatusOK {
		t.Fatalf("atomic replacement did not recover: %d", response.Code)
	}
	// A restarted run reuses the path and inode with a shorter complete history.
	monitoringCSV(t, path, 7)
	assertRows(7)
	response = request(revision)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "All 7 recorded rows") {
		t.Fatalf("truncated/restarted log did not replace stale data: %d", response.Code)
	}
}

func TestLiveMonitoringPreservesSourcePrecedence(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	for _, source := range []string{root, other} {
		sourceProject(t, source, "{}")
		sourceRecord(t, source, "same", "Training", "")
		writeTestReport(t, source, "same", source, "step,loss\n0,1\n")
	}
	sourceWrite(t, filepath.Join(other, "experiments", "same", "metrics.csv"), "step,loss\n0,NaN\n")
	load := liveSnapshotLoader(root, []string{root, other})
	for range 3 {
		snapshot, err := load()
		if err != nil || snapshot.Options.Reports["same"].Blocks[0].Markdown != root {
			t.Fatalf("winning report: %v", err)
		}
	}
	if _, err := liveSnapshotLoader(root, []string{other, root})(); err == nil {
		t.Fatal("invalid winning source was ignored")
	}
}
