package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/web"
)

func BenchmarkMonitorLiveRefresh(b *testing.B) {
	for _, rows := range []int{10_000, 100_000, 1_000_000} {
		b.Run(fmt.Sprintf("rows%d", rows), func(b *testing.B) {
			for _, mode := range []string{"ColdHTTP", "WarmHTTP", "AppendHTTP", "Render"} {
				b.Run(mode, func(b *testing.B) {
					root, path := monitorSnapshotFixture(b, rows)
					snapshot, err := loadSnapshot(root, nil)
					if err != nil {
						b.Fatal(err)
					}
					snapshot.Options.Live = true
					body, err := web.Render(snapshot.Records, snapshot.Options)
					if err != nil {
						b.Fatal(err)
					}
					if mode == "Render" {
						if dir := os.Getenv("EXPLEDGER_MONITOR_ARTIFACTS"); dir != "" {
							if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("monitor-%d.html", rows)), body, 0o644); err != nil {
								b.Fatal(err)
							}
						}
						if dir := os.Getenv("EXPLEDGER_MONITOR_FIXTURE_DIR"); dir != "" && rows == 1_000_000 {
							monitorCopyFixture(b, root, dir)
						}
					}
					builtBytes := len(body)
					info, err := os.Stat(path)
					if err != nil {
						b.Fatal(err)
					}
					handler := web.NewLiveHandler(liveSnapshotLoader(root, nil))
					etag := ""
					request := func() *httptest.ResponseRecorder {
						response := httptest.NewRecorder()
						req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
						if etag != "" {
							req.Header.Set("If-None-Match", etag)
						}
						handler.ServeHTTP(response, req)
						return response
					}
					if mode == "WarmHTTP" || mode == "AppendHTTP" {
						initial := request()
						if initial.Code != http.StatusOK {
							b.Fatalf("initial HTTP %d: %s", initial.Code, initial.Body)
						}
						etag = initial.Header().Get("ETag")
					}
					var response *httptest.ResponseRecorder
					b.ReportAllocs()
					b.ResetTimer()
					for i := range b.N {
						if mode == "ColdHTTP" || mode == "AppendHTTP" {
							b.StopTimer()
							if mode == "ColdHTTP" {
								handler = web.NewLiveHandler(liveSnapshotLoader(root, nil))
								etag = ""
							} else {
								monitorSnapshotAppend(b, path, rows+i)
							}
							b.StartTimer()
						}
						if mode == "Render" {
							body, err = web.Render(snapshot.Records, snapshot.Options)
							if err != nil {
								b.Fatal(err)
							}
						} else {
							response = request()
							if response.Code != http.StatusOK && response.Code != http.StatusNotModified {
								b.Fatalf("HTTP %d: %s", response.Code, response.Body)
							}
							etag = response.Header().Get("ETag")
						}
					}
					b.StopTimer()
					if mode == "AppendHTTP" {
						snapshot, err = loadSnapshot(root, nil)
						if err != nil {
							b.Fatal(err)
						}
					}
					b.ReportMetric(float64(builtBytes), "builtHTMLBytes")
					b.ReportMetric(float64(info.Size()), "csvBytes")
					if response != nil {
						b.ReportMetric(float64(response.Body.Len()), "responseBytes")
						b.ReportMetric(float64(response.Code), "httpStatus")
					}
					for i, block := range snapshot.Options.Reports["monitor"].Blocks {
						b.ReportMetric(float64(len(block.Data.X)), fmt.Sprintf("chart%dPlotRows", i+1))
						b.ReportMetric(float64(len(block.Data.Table.X)), fmt.Sprintf("chart%dTableRows", i+1))
					}
				})
			}
		})
	}
}

func monitorCopyFixture(b *testing.B, source, target string) {
	b.Helper()
	if err := os.MkdirAll(filepath.Join(target, "experiments", "monitor"), 0o755); err != nil {
		b.Fatal(err)
	}
	for _, name := range []string{"expledger.yaml", "experiments/monitor/experiment.yaml", "experiments/monitor/README.md", "experiments/monitor/report.yaml", "experiments/monitor/metrics.csv"} {
		input, err := os.Open(filepath.Join(source, name))
		if err != nil {
			b.Fatal(err)
		}
		output, err := os.Create(filepath.Join(target, name))
		if err != nil {
			input.Close()
			b.Fatal(err)
		}
		_, copyErr := io.Copy(output, input)
		inputErr, outputErr := input.Close(), output.Close()
		if copyErr != nil || inputErr != nil || outputErr != nil {
			b.Fatalf("copy %s: %v, %v, %v", name, copyErr, inputErr, outputErr)
		}
	}
}

// TestMonitorMemory is an opt-in observation, separate from timed benchmarks and
// deterministic tests. It measures sampled/retained Go heap, not process RSS.
func TestMonitorMemory(t *testing.T) {
	if os.Getenv("EXPLEDGER_MONITOR_MEMORY") != "1" {
		t.Skip("set EXPLEDGER_MONITOR_MEMORY=1 to observe heap usage")
	}
	for _, rows := range []int{10_000, 100_000, 1_000_000} {
		t.Run(fmt.Sprintf("rows%d", rows), func(t *testing.T) {
			root, path := monitorSnapshotFixture(t, rows)
			handler := web.NewLiveHandler(liveSnapshotLoader(root, nil))
			etag := ""
			refresh := func() *httptest.ResponseRecorder {
				response := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
				if etag != "" {
					req.Header.Set("If-None-Match", etag)
				}
				handler.ServeHTTP(response, req)
				etag = response.Header().Get("ETag")
				return response
			}
			for _, phase := range []string{"Cold", "Warm", "Append"} {
				if phase == "Append" {
					monitorSnapshotAppend(t, path, rows)
				}
				runtime.GC()
				var before, after, retained runtime.MemStats
				runtime.ReadMemStats(&before)
				var peak atomic.Uint64
				peak.Store(before.HeapAlloc)
				observe := func() {
					var stats runtime.MemStats
					runtime.ReadMemStats(&stats)
					for old := peak.Load(); stats.HeapAlloc > old; old = peak.Load() {
						if peak.CompareAndSwap(old, stats.HeapAlloc) {
							break
						}
					}
				}
				stop, stopped := make(chan struct{}), make(chan struct{})
				go func() {
					defer close(stopped)
					ticker := time.NewTicker(time.Millisecond)
					defer ticker.Stop()
					for {
						select {
						case <-ticker.C:
							observe()
						case <-stop:
							return
						}
					}
				}()
				response := refresh()
				observe()
				runtime.ReadMemStats(&after)
				close(stop)
				<-stopped
				runtime.GC()
				runtime.ReadMemStats(&retained)
				runtime.KeepAlive(response)
				runtime.KeepAlive(refresh)
				if response.Code != http.StatusOK && response.Code != http.StatusNotModified {
					t.Fatalf("HTTP %d: %s", response.Code, response.Body)
				}
				t.Logf("%s baselineHeapBytes=%d sampledPeakHeapBytes=%d retainedHeapBytes=%d sampledPeakHeapDeltaBytes=%d retainedHeapDeltaBytes=%d allocatedBytes=%d responseBytes=%d httpStatus=%d", phase,
					before.HeapAlloc, peak.Load(), retained.HeapAlloc,
					max(0, int64(peak.Load())-int64(before.HeapAlloc)), int64(retained.HeapAlloc)-int64(before.HeapAlloc),
					after.TotalAlloc-before.TotalAlloc, response.Body.Len(), response.Code)
			}
		})
	}
}

func monitorSnapshotFixture(t testing.TB, rows int) (string, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "experiments", "monitor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"experiment.yaml": "schema: expledger/v1\nid: monitor\ntitle: Training monitor\ncreated_at: 2026-10-06T12:00:00Z\n",
		"README.md":       "# Training monitor\n\nTiming fixture with sampled plots and recent raw values.\n",
		"report.yaml": "blocks:\n" +
			"  - {type: line, title: Loss, source: metrics.csv, x: step, y: [train_loss, validation_loss]}\n" +
			"  - {type: line, title: Throughput, source: metrics.csv, x: step, y: [tokens_per_second]}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "expledger.yaml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "metrics.csv")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := bufio.NewWriter(file)
	if _, err := writer.WriteString("step,train_loss,validation_loss,tokens_per_second\n"); err != nil {
		t.Fatal(err)
	}
	for step := range rows {
		if _, err := writer.WriteString(monitorSnapshotRow(step, rows)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return root, path
}

func monitorSnapshotRow(step, rows int) string {
	train := float64((step*17)%1000) / 100
	validation := fmt.Sprintf("%.3f", float64((step*29)%1000)/100)
	if step == rows/3 {
		train = 1000
	} else if step == rows*2/3 {
		train = -1000
	}
	for run := 1; run <= 5; run++ {
		start := rows * run / 6
		if step >= start && step < start+11 {
			validation = ""
		}
	}
	return fmt.Sprintf("%d,%.3f,%s,%.3f\n", step, train, validation, 1000+float64(step%137))
}

func monitorSnapshotAppend(t testing.TB, path string, step int) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(monitorSnapshotRow(step, 0)); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
