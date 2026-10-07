package report

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/marcfranquesa/expledger/internal/catalog"
)

// BenchmarkMonitorReader measures file-backed reads; fixture construction and
// appending a record are outside the timer and allocation counters.
func BenchmarkMonitorReader(b *testing.B) {
	for _, rows := range []int{10_000, 100_000, 1_000_000} {
		b.Run(fmt.Sprintf("rows%d", rows), func(b *testing.B) {
			for _, mode := range []string{"StaticRead", "WarmRefresh", "AppendRefresh"} {
				b.Run(mode, func(b *testing.B) {
					root, path := monitorReaderFixture(b, rows)
					reader := NewReader(true)
					read := func() (*Report, error) {
						if mode == "StaticRead" {
							return Read(root, "monitor", catalog.Layout{})
						}
						reports, err := reader.ReadMany(root, []string{"monitor"}, catalog.Layout{})
						return reports["monitor"], err
					}
					result, err := read()
					if err != nil {
						b.Fatal(err)
					}
					info, err := os.Stat(path)
					if err != nil {
						b.Fatal(err)
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := range b.N {
						if mode == "AppendRefresh" {
							b.StopTimer()
							monitorReaderAppend(b, path, rows+i)
							b.StartTimer()
						}
						result, err = read()
						if err != nil {
							b.Fatal(err)
						}
					}
					b.StopTimer()
					b.ReportMetric(float64(info.Size()), "csvBytes")
					for i, block := range result.Blocks {
						if block.Data == nil || block.Data.Table == nil {
							b.Fatal("missing chart projection")
						}
						b.ReportMetric(float64(len(block.Data.X)), fmt.Sprintf("chart%dPlotRows", i+1))
						b.ReportMetric(float64(len(block.Data.Table.X)), fmt.Sprintf("chart%dTableRows", i+1))
					}
				})
			}
		})
	}
}

func monitorReaderFixture(b *testing.B, rows int) (string, string) {
	b.Helper()
	root := b.TempDir()
	dir := filepath.Join(root, "experiments", "monitor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.Fatal(err)
	}
	manifest := "blocks:\n" +
		"  - {type: line, title: Loss, source: metrics.csv, x: step, y: [train_loss, validation_loss]}\n" +
		"  - {type: line, title: Throughput, source: metrics.csv, x: step, y: [tokens_per_second]}\n"
	if err := os.WriteFile(filepath.Join(dir, "report.yaml"), []byte(manifest), 0o644); err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(dir, "metrics.csv")
	file, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	writer := bufio.NewWriter(file)
	if _, err := writer.WriteString("step,train_loss,validation_loss,tokens_per_second\n"); err != nil {
		b.Fatal(err)
	}
	for step := range rows {
		if _, err := writer.WriteString(monitorReaderRow(step, rows)); err != nil {
			b.Fatal(err)
		}
	}
	if err := writer.Flush(); err != nil {
		b.Fatal(err)
	}
	if err := file.Close(); err != nil {
		b.Fatal(err)
	}
	return root, path
}

func monitorReaderRow(step, rows int) string {
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

func monitorReaderAppend(b *testing.B, path string, step int) {
	b.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := file.WriteString(monitorReaderRow(step, 0)); err != nil {
		file.Close()
		b.Fatal(err)
	}
	if err := file.Close(); err != nil {
		b.Fatal(err)
	}
}
