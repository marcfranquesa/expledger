package report

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marcfranquesa/expledger/internal/catalog"
)

func reportFixture(t *testing.T, manifest string, sources map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "experiments", "example")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if manifest != "" {
		writeReportFile(t, filepath.Join(dir, "report.yaml"), manifest)
	}
	for name, contents := range sources {
		writeReportFile(t, filepath.Join(dir, filepath.FromSlash(name)), contents)
	}
	return root, dir
}

func writeReportFile(t *testing.T, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func lineManifest(source string) string {
	return fmt.Sprintf("blocks:\n  - type: line\n    title: Training loss\n    source: %q\n    x: step\n    y: [train_loss, validation_loss]\n", source)
}

func TestReadComposedReport(t *testing.T) {
	root, _ := reportFixture(t, `blocks:
  - type: markdown
    source: introduction.md
  - type: row
    blocks:
      - type: line
        title: Training loss
        source: results/metrics.csv
        x: step
        y: [train_loss, validation_loss]
      - type: line
        title: Throughput
        source: results/metrics.csv
        x: step
        y: [tokens_per_second]
  - type: markdown
    source: conclusions.md
`, map[string]string{
		"introduction.md": "# Experiment\n\nTesting a hypothesis.\n",
		"conclusions.md":  "The validation loss improves.\n",
		"results/metrics.csv": "step,train_loss,validation_loss,tokens_per_second\r\n" +
			"0,3, 3.5 ,10\r\n1,2, ,12\r\n2,1,1.5,15\r\n",
	})
	report, err := Read(root, "example", catalog.Layout{})
	if err != nil {
		t.Fatal(err)
	}
	if report == nil || len(report.Blocks) != 3 {
		t.Fatalf("report = %#v, want three top-level blocks", report)
	}
	if got := report.Blocks[0].Markdown; got != "# Experiment\n\nTesting a hypothesis.\n" {
		t.Fatalf("introduction = %q", got)
	}
	row := report.Blocks[1]
	if row.Type != "row" || len(row.Blocks) != 2 {
		t.Fatalf("row = %#v", row)
	}
	line := row.Blocks[0]
	if line.Title != "Training loss" || line.Data == nil {
		t.Fatalf("line = %#v", line)
	}
	if got := fmt.Sprint(line.Data.X); got != "[0 1 2]" {
		t.Fatalf("x = %s, want preserved numeric file order", got)
	}
	if len(line.Data.Series) != 2 || line.Data.Series[0].Name != "train_loss" || line.Data.Series[1].Name != "validation_loss" {
		t.Fatalf("series = %#v", line.Data.Series)
	}
	for i, want := range []float64{3, 2, 1} {
		got := line.Data.Series[0].Values[i]
		if got == nil || *got != want {
			t.Fatalf("training value %d = %v, want %v", i, got, want)
		}
	}
	validation := line.Data.Series[1].Values
	if len(validation) != 3 || validation[0] == nil || *validation[0] != 3.5 || validation[1] != nil || validation[2] == nil || *validation[2] != 1.5 {
		t.Fatalf("validation values = %#v, want numeric values and an explicit gap", validation)
	}
	throughput := row.Blocks[1]
	if throughput.Data == nil || len(throughput.Data.Series) != 1 || throughput.Data.Series[0].Name != "tokens_per_second" || *throughput.Data.Series[0].Values[2] != 15 {
		t.Fatalf("throughput = %#v", throughput)
	}
	if report.Blocks[2].Markdown != "The validation loss improves.\n" {
		t.Fatalf("conclusion = %q", report.Blocks[2].Markdown)
	}
}

func TestReadNoManifest(t *testing.T) {
	root, _ := reportFixture(t, "", nil)
	report, err := Read(root, "example", catalog.Layout{})
	if err != nil || report != nil {
		t.Fatalf("Read = (%#v, %v), want absent report", report, err)
	}
}

func TestReadManySelectedReports(t *testing.T) {
	root, _ := reportFixture(t, "blocks: [{type: markdown, source: intro.md}]\n", map[string]string{"intro.md": "Selected notes"})
	if err := os.Mkdir(filepath.Join(root, "experiments", "without-report"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeReportFile(t, filepath.Join(root, "experiments", "unselected", "report.yaml"), "invalid report")
	reports, err := ReadMany(root, []string{"example", "without-report"}, catalog.Layout{})
	if err != nil || len(reports) != 1 || reports["example"].Blocks[0].Markdown != "Selected notes" || reports["without-report"] != nil {
		t.Fatalf("ReadMany = (%#v, %v), want only the selected manifest", reports, err)
	}
	if reports, err = ReadMany(t.TempDir(), nil, catalog.Layout{}); err != nil || len(reports) != 0 {
		t.Fatalf("empty selection = (%#v, %v)", reports, err)
	}
}

func TestReadManyRejectsInexactNamesAndDirectorySymlinks(t *testing.T) {
	for _, kind := range []string{"inexact name", "experiment symlink", "experiments symlink", "invalid ID"} {
		t.Run(kind, func(t *testing.T) {
			root, _ := reportFixture(t, "blocks: [{type: markdown, source: intro.md}]\n", map[string]string{"intro.md": "Selected notes"})
			ids := []string{"example", "EXAMPLE"}
			switch kind {
			case "experiment symlink":
				if err := os.Symlink("example", filepath.Join(root, "experiments", "linked")); err != nil {
					t.Fatal(err)
				}
				ids[1] = "linked"
			case "experiments symlink":
				if err := os.Rename(filepath.Join(root, "experiments"), filepath.Join(root, "real-experiments")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("real-experiments", filepath.Join(root, "experiments")); err != nil {
					t.Fatal(err)
				}
				ids = ids[:1]
			case "invalid ID":
				ids[1] = "../example"
			}
			reports, err := ReadMany(root, ids, catalog.Layout{})
			if err == nil || reports != nil {
				t.Fatalf("ReadMany accepted %s or returned partial reports: (%#v, %v)", kind, reports, err)
			}
			if kind == "inexact name" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("inexact name error = %v, want not found", err)
			}
		})
	}
}

func TestReadManyConfiguredNestedLayout(t *testing.T) {
	root := t.TempDir()
	layout := catalog.Layout{ExperimentsDir: "research/trials", ExperimentFormat: "{date}/{slug}"}
	ids := []string{"2026-10-06/baseline", "2026-10-06/variant", "2026-10-07/without-report"}
	for _, id := range ids {
		dir := filepath.Join(root, filepath.FromSlash(layout.ExperimentsDir), filepath.FromSlash(id))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(id, "without-report") {
			continue
		}
		writeReportFile(t, filepath.Join(dir, "report.yaml"), "blocks: [{type: markdown, source: intro.md}]\n")
		writeReportFile(t, filepath.Join(dir, "intro.md"), "Notes for "+id)
	}
	writeReportFile(t, filepath.Join(root, "experiments", "unused", "report.yaml"), "invalid default report")
	reports, err := ReadMany(root, ids, layout)
	if err != nil || len(reports) != 2 || reports[ids[2]] != nil {
		t.Fatalf("configured batch = (%#v, %v)", reports, err)
	}
	for _, id := range ids[:2] {
		if reports[id].Blocks[0].Markdown != "Notes for "+id {
			t.Errorf("report %s loaded the wrong source", id)
		}
	}
	if report, err := Read(root, ids[0], layout); err != nil || report.Blocks[0].Markdown != "Notes for "+ids[0] {
		t.Fatalf("configured single read = (%#v, %v)", report, err)
	}
	for _, invalid := range []catalog.Layout{{ExperimentsDir: "../trials"}, {ExperimentsDir: "research/./trials"}, {ExperimentFormat: "{unknown}"}} {
		if reports, err := ReadMany(root, nil, invalid); err == nil || reports != nil {
			t.Fatalf("accepted invalid layout: (%#v, %v)", reports, err)
		}
	}
}

func TestReadManyConfiguredPathsRejectSymlinksAndInexactNames(t *testing.T) {
	for _, component := range []string{"research", "research/trials", "research/trials/Group", "research/trials/Group/example"} {
		t.Run(component, func(t *testing.T) {
			root := t.TempDir()
			layout := catalog.Layout{ExperimentsDir: "research/trials"}
			dir := filepath.Join(root, "research", "trials", "Group", "example")
			writeReportFile(t, filepath.Join(dir, "report.yaml"), "blocks: [{type: markdown, source: intro.md}]\n")
			writeReportFile(t, filepath.Join(dir, "intro.md"), "Confined notes")
			path := filepath.Join(root, filepath.FromSlash(component))
			if err := os.Rename(path, path+"-real"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Base(path)+"-real", path); err != nil {
				t.Fatal(err)
			}
			if reports, err := ReadMany(root, []string{"Group/example"}, layout); err == nil || reports != nil {
				t.Fatalf("accepted symlink component %s: (%#v, %v)", component, reports, err)
			}
		})
	}
	root := t.TempDir()
	writeReportFile(t, filepath.Join(root, "research", "trials", "Group", "example", "report.yaml"), "blocks: [{type: markdown, source: intro.md}]\n")
	writeReportFile(t, filepath.Join(root, "research", "trials", "Group", "example", "intro.md"), "Notes")
	for _, tc := range []struct{ directory, id string }{{"Research/trials", "Group/example"}, {"research/Trials", "Group/example"}, {"research/trials", "group/example"}, {"research/trials", "Group/Example"}} {
		if reports, err := ReadMany(root, []string{tc.id}, catalog.Layout{ExperimentsDir: tc.directory}); !errors.Is(err, os.ErrNotExist) || reports != nil {
			t.Fatalf("accepted inexact path %s/%s: (%#v, %v)", tc.directory, tc.id, reports, err)
		}
	}
}

func TestReadManyRejectsNestedExperimentAndEscapingReportSources(t *testing.T) {
	for _, kind := range []string{"ancestor metadata file", "ancestor metadata directory", "ancestor metadata symlink", "sibling source"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			layout := catalog.Layout{ExperimentsDir: "research/trials"}
			dir := filepath.Join(root, "research", "trials", "group", "example")
			writeReportFile(t, filepath.Join(dir, "report.yaml"), lineManifest("results.csv"))
			writeReportFile(t, filepath.Join(dir, "results.csv"), "step,train_loss,validation_loss\n0,1,2\n")
			ancestor := filepath.Join(filepath.Dir(dir), "experiment.yaml")
			switch kind {
			case "ancestor metadata file":
				writeReportFile(t, ancestor, "invalid ancestor metadata still defines an experiment")
			case "ancestor metadata directory":
				if err := os.Mkdir(ancestor, 0o755); err != nil {
					t.Fatal(err)
				}
			case "ancestor metadata symlink":
				if err := os.Symlink("missing.yaml", ancestor); err != nil {
					t.Fatal(err)
				}
			case "sibling source":
				writeReportFile(t, filepath.Join(filepath.Dir(dir), "sibling", "results.csv"), "step,train_loss,validation_loss\n0,9,8\n")
				if err := os.Remove(filepath.Join(dir, "results.csv")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("../sibling/results.csv", filepath.Join(dir, "results.csv")); err != nil {
					t.Fatal(err)
				}
			}
			reports, err := ReadMany(root, []string{"group/example"}, layout)
			if err == nil || reports != nil {
				t.Fatalf("accepted %s: (%#v, %v)", kind, reports, err)
			}
			if kind == "sibling source" && !strings.Contains(err.Error(), "research/trials/group/example/report.yaml") {
				t.Fatalf("source error lacks configured report context: %v", err)
			}
		})
	}
}

func TestReadUnavailableResults(t *testing.T) {
	for _, tc := range []struct {
		name, csv string
		missing   bool
	}{
		{name: "not produced yet", missing: true},
		{name: "empty"},
		{name: "headers only", csv: "step,train_loss,validation_loss\n"},
		{name: "all gaps", csv: "step,train_loss,validation_loss\n0,, \n1, ,\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sources := map[string]string{}
			if !tc.missing {
				sources["results.csv"] = tc.csv
			}
			root, _ := reportFixture(t, lineManifest("results.csv"), sources)
			report, err := Read(root, "example", catalog.Layout{})
			if err != nil {
				t.Fatal(err)
			}
			if got := report.Blocks[0].Message; got != "Results unavailable" {
				t.Fatalf("message = %q", got)
			}
		})
	}
}

func TestReadStrictManifest(t *testing.T) {
	for _, tc := range []struct{ name, manifest string }{
		{"empty document", "\n"},
		{"scalar root", "hello\n"},
		{"sequence root", "- blocks: []\n"},
		{"missing blocks", "{}\n"},
		{"null blocks", "blocks: null\n"},
		{"empty blocks", "blocks: []\n"},
		{"multiple documents", "blocks: [{type: markdown, source: intro.md}]\n---\nblocks: [{type: markdown, source: intro.md}]\n"},
		{"unknown root field", "blocks: [{type: markdown, source: intro.md}]\ntheme: blue\n"},
		{"duplicate root key", "blocks: [{type: markdown, source: intro.md}]\nblocks: [{type: markdown, source: intro.md}]\n"},
		{"duplicate block key", "blocks: [{type: markdown, source: intro.md, source: other.md}]\n"},
		{"unknown block field", "blocks: [{type: markdown, source: intro.md, width: 10}]\n"},
		{"unknown type", "blocks: [{type: scatter, source: data.csv}]\n"},
		{"missing type", "blocks: [{source: intro.md}]\n"},
		{"non string type", "blocks: [{type: 1, source: intro.md}]\n"},
		{"non string source", "blocks: [{type: markdown, source: 12}]\n"},
		{"markdown with chart fields", "blocks: [{type: markdown, source: intro.md, x: step}]\n"},
		{"markdown with children", "blocks: [{type: markdown, source: intro.md, blocks: []}]\n"},
		{"line without title", "blocks: [{type: line, source: data.csv, x: step, y: [loss]}]\n"},
		{"line blank title", "blocks: [{type: line, title: '   ', source: data.csv, x: step, y: [loss]}]\n"},
		{"line without x", "blocks: [{type: line, title: Loss, source: data.csv, y: [loss]}]\n"},
		{"line without y", "blocks: [{type: line, title: Loss, source: data.csv, x: step}]\n"},
		{"line scalar y", "blocks: [{type: line, title: Loss, source: data.csv, x: step, y: loss}]\n"},
		{"line empty y", "blocks: [{type: line, title: Loss, source: data.csv, x: step, y: []}]\n"},
		{"line duplicate y", "blocks: [{type: line, title: Loss, source: data.csv, x: step, y: [loss, loss]}]\n"},
		{"line with children", "blocks: [{type: line, title: Loss, source: data.csv, x: step, y: [loss], blocks: []}]\n"},
		{"row without children", "blocks: [{type: row}]\n"},
		{"row empty children", "blocks: [{type: row, blocks: []}]\n"},
		{"row with source", "blocks: [{type: row, source: intro.md, blocks: [{type: markdown, source: intro.md}]}]\n"},
		{"nested row", "blocks: [{type: row, blocks: [{type: row, blocks: [{type: markdown, source: intro.md}]}]}]\n"},
		{"alias", "blocks: [&text {type: markdown, source: intro.md}, *text]\n"},
		{"merge", "blocks: [{type: markdown, source: intro.md, <<: {source: other.md}}]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := reportFixture(t, tc.manifest, map[string]string{"intro.md": "Hello", "data.csv": "step,loss\n0,1\n"})
			if _, err := Read(root, "example", catalog.Layout{}); err == nil {
				t.Fatal("Read accepted malformed report.yaml")
			}
		})
	}
}

func TestReadInvalidCSV(t *testing.T) {
	for _, tc := range []struct{ name, csv string }{
		{"missing x", "iteration,train_loss,validation_loss\n0,1,2\n"},
		{"missing y", "step,train_loss\n0,1\n"},
		{"header names exact", "step, train_loss,validation_loss\n0,1,2\n"},
		{"duplicate headers", "step,train_loss,train_loss,validation_loss\n0,1,2,3\n"},
		{"empty header", "step,train_loss,validation_loss,\n0,1,2,3\n"},
		{"short row", "step,train_loss,validation_loss\n0,1\n"},
		{"long row", "step,train_loss,validation_loss\n0,1,2,3\n"},
		{"bad quoting", "step,train_loss,validation_loss\n0,\"1,2\n"},
		{"empty x", "step,train_loss,validation_loss\n,1,2\n"},
		{"text x", "step,train_loss,validation_loss\nstart,1,2\n"},
		{"NaN x", "step,train_loss,validation_loss\nNaN,1,2\n"},
		{"infinite x", "step,train_loss,validation_loss\n+Inf,1,2\n"},
		{"overflow x", "step,train_loss,validation_loss\n1e400,1,2\n"},
		{"duplicate x", "step,train_loss,validation_loss\n0,1,2\n0,2,3\n"},
		{"descending x", "step,train_loss,validation_loss\n1,1,2\n0,2,3\n"},
		{"text y", "step,train_loss,validation_loss\n0,missing,2\n"},
		{"NaN y", "step,train_loss,validation_loss\n0,NaN,2\n"},
		{"infinite y", "step,train_loss,validation_loss\n0,1,-Inf\n"},
		{"overflow y", "step,train_loss,validation_loss\n0,1,1e400\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := reportFixture(t, lineManifest("results.csv"), map[string]string{"results.csv": tc.csv})
			_, err := Read(root, "example", catalog.Layout{})
			if err == nil {
				t.Fatal("Read accepted malformed CSV")
			}
			if !strings.Contains(err.Error(), "results.csv") {
				t.Fatalf("error lacks source context: %v", err)
			}
		})
	}
}

func TestReadNestedChartErrorContext(t *testing.T) {
	manifest := "blocks: [{type: row, blocks: [{type: line, title: Loss, source: results.csv, x: step, y: [loss]}]}]\n"
	root, _ := reportFixture(t, manifest, map[string]string{"results.csv": "step,loss\n0,invalid\n"})
	_, err := Read(root, "example", catalog.Layout{})
	if err == nil {
		t.Fatal("Read accepted nonnumeric chart value")
	}
	for _, context := range []string{"experiments/example/report.yaml", "blocks[0].blocks[0]", "results.csv", "loss"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("error lacks %q context: %v", context, err)
		}
	}
}

func TestReadRejectsUnsafeIDs(t *testing.T) {
	root, _ := reportFixture(t, "", nil)
	for _, id := range []string{"", ".", "..", "nested//other", "nested/./other", "nested/experiment.yaml", "example\\other", "../example", "/example"} {
		t.Run(fmt.Sprintf("%q", id), func(t *testing.T) {
			if _, err := Read(root, id, catalog.Layout{}); err == nil {
				t.Fatalf("Read accepted id %q", id)
			}
		})
	}
}

func TestReadRejectsUnsafeSources(t *testing.T) {
	for _, source := range []string{"", "/tmp/results.csv", "../results.csv", "data/../results.csv", "data\\results.csv"} {
		t.Run(fmt.Sprintf("%q", source), func(t *testing.T) {
			root, _ := reportFixture(t, lineManifest(source), nil)
			if _, err := Read(root, "example", catalog.Layout{}); err == nil {
				t.Fatalf("Read accepted source %q", source)
			}
		})
	}
}

func TestReadSourceFilesystem(t *testing.T) {
	t.Run("missing markdown", func(t *testing.T) {
		root, _ := reportFixture(t, "blocks: [{type: markdown, source: intro.md}]\n", nil)
		if _, err := Read(root, "example", catalog.Layout{}); err == nil || !strings.Contains(err.Error(), "intro.md") {
			t.Fatalf("Read error = %v, want missing markdown source context", err)
		}
	})
	t.Run("source directory", func(t *testing.T) {
		root, dir := reportFixture(t, lineManifest("results.csv"), nil)
		if err := os.Mkdir(filepath.Join(dir, "results.csv"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(root, "example", catalog.Layout{}); err == nil {
			t.Fatal("Read accepted directory as CSV source")
		}
	})
	t.Run("manifest directory", func(t *testing.T) {
		root, dir := reportFixture(t, "", nil)
		if err := os.Mkdir(filepath.Join(dir, "report.yaml"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(root, "example", catalog.Layout{}); err == nil {
			t.Fatal("Read accepted directory as manifest")
		}
	})
}

func TestReadSymlinks(t *testing.T) {
	t.Run("confined source", func(t *testing.T) {
		root, dir := reportFixture(t, lineManifest("results.csv"), map[string]string{"data/actual.csv": "step,train_loss,validation_loss\n0,1,2\n"})
		if err := os.Symlink("data/actual.csv", filepath.Join(dir, "results.csv")); err != nil {
			t.Fatal(err)
		}
		if report, err := Read(root, "example", catalog.Layout{}); err != nil || report.Blocks[0].Data == nil {
			t.Fatalf("Read = (%#v, %v), want confined symlink to work", report, err)
		}
	})
	for _, tc := range []struct{ name, target string }{
		{"dangling source", "not-created.csv"},
		{"cross experiment", "../other/results.csv"},
		{"external source", "../../outside.csv"},
		{"outside missing source", "../../missing.csv"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, dir := reportFixture(t, lineManifest("results.csv"), nil)
			writeReportFile(t, filepath.Join(root, "experiments", "other", "results.csv"), "step,train_loss,validation_loss\n0,1,2\n")
			writeReportFile(t, filepath.Join(root, "outside.csv"), "step,train_loss,validation_loss\n0,1,2\n")
			if err := os.Symlink(tc.target, filepath.Join(dir, "results.csv")); err != nil {
				t.Fatal(err)
			}
			if _, err := Read(root, "example", catalog.Layout{}); err == nil {
				t.Fatal("Read accepted unresolvable or escaping source symlink")
			}
		})
	}
	t.Run("dangling manifest", func(t *testing.T) {
		root, dir := reportFixture(t, "", nil)
		if err := os.Symlink("missing.yaml", filepath.Join(dir, "report.yaml")); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(root, "example", catalog.Layout{}); err == nil {
			t.Fatal("dangling manifest was treated as absent")
		}
	})
	t.Run("dangling intermediate source directory", func(t *testing.T) {
		root, dir := reportFixture(t, lineManifest("results/metrics.csv"), nil)
		if err := os.Symlink("not-created", filepath.Join(dir, "results")); err != nil {
			t.Fatal(err)
		}
		_, err := Read(root, "example", catalog.Layout{})
		if err == nil {
			t.Fatal("dangling source directory symlink was treated as unavailable")
		}
		for _, context := range []string{"report.yaml", "blocks[0]", "results/metrics.csv"} {
			if !strings.Contains(err.Error(), context) {
				t.Fatalf("error lacks %q context: %v", context, err)
			}
		}
	})
	t.Run("experiment symlink", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "experiments"), 0o755); err != nil {
			t.Fatal(err)
		}
		target := t.TempDir()
		if err := os.Symlink(target, filepath.Join(root, "experiments", "example")); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(root, "example", catalog.Layout{}); err == nil {
			t.Fatal("Read accepted experiment directory symlink")
		}
	})
	t.Run("experiments symlink", func(t *testing.T) {
		root := t.TempDir()
		target := t.TempDir()
		if err := os.Mkdir(filepath.Join(target, "example"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, "experiments")); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(root, "example", catalog.Layout{}); err == nil {
			t.Fatal("Read accepted experiments directory symlink")
		}
	})
}

func TestReadLimits(t *testing.T) {
	t.Run("manifest bytes", func(t *testing.T) {
		root, _ := reportFixture(t, "#"+strings.Repeat("a", 256*1024)+"\nblocks: [{type: markdown, source: intro.md}]\n", map[string]string{"intro.md": "Hello"})
		if _, err := Read(root, "example", catalog.Layout{}); err == nil {
			t.Fatal("Read accepted oversized manifest")
		}
	})
	t.Run("markdown bytes", func(t *testing.T) {
		root, _ := reportFixture(t, "blocks: [{type: markdown, source: intro.md}]\n", map[string]string{"intro.md": strings.Repeat("a", 1024*1024+1)})
		if _, err := Read(root, "example", catalog.Layout{}); err == nil {
			t.Fatal("Read accepted oversized markdown")
		}
	})
	t.Run("CSV bytes", func(t *testing.T) {
		root, dir := reportFixture(t, lineManifest("results.csv"), nil)
		file, err := os.Create(filepath.Join(dir, "results.csv"))
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(maxCSVBytes + 1); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(root, "example", catalog.Layout{}); err == nil || !strings.Contains(err.Error(), "bytes") {
			t.Fatalf("Read error = %v, want oversized CSV bytes", err)
		}
	})
	t.Run("total unique source bytes", func(t *testing.T) {
		loader := sourceLoader{}
		for range 2 {
			if err := loader.account("first.csv", maxCSVBytes); err != nil {
				t.Fatal(err)
			}
		}
		if err := loader.account("second.csv", maxCSVBytes); err != nil {
			t.Fatal(err)
		}
		if err := loader.account("third.csv", 1); err == nil {
			t.Fatal("accepted excessive total source bytes")
		}
	})
	t.Run("blocks including rows", func(t *testing.T) {
		manifest := "blocks:\n  - type: row\n    blocks:\n" + strings.Repeat("      - {type: markdown, source: intro.md}\n", 64)
		root, _ := reportFixture(t, manifest, map[string]string{"intro.md": "Hello"})
		if _, err := Read(root, "example", catalog.Layout{}); err == nil {
			t.Fatal("Read accepted 65 blocks including the row")
		}
	})
	t.Run("series", func(t *testing.T) {
		manifest := "blocks: [{type: line, title: Loss, source: missing.csv, x: step, y: [a,b,c,d,e,f,g,h,i]}]\n"
		root, _ := reportFixture(t, manifest, nil)
		if _, err := Read(root, "example", catalog.Layout{}); err == nil {
			t.Fatal("Read accepted nine series")
		}
	})

}

func TestReadCountsRepeatedNormalizedSourceOnce(t *testing.T) {
	var manifest strings.Builder
	manifest.WriteString("blocks:\n")
	for i := range 60 {
		name := "shared.md"
		if i%2 == 1 {
			name = "./shared.md"
		}
		fmt.Fprintf(&manifest, "  - {type: markdown, source: %s}\n", name)
	}
	contents := strings.Repeat("a", 300*1024)
	root, _ := reportFixture(t, manifest.String(), map[string]string{"shared.md": contents})
	report, err := Read(root, "example", catalog.Layout{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Blocks) != 60 || report.Blocks[59].Markdown != contents {
		t.Fatal("repeated source was not loaded correctly")
	}
}

func TestMarkdownLoaderKeepsConsistentSnapshot(t *testing.T) {
	_, dir := reportFixture(t, "", map[string]string{"shared.md": "Original notes\n"})
	directory, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	loader := sourceLoader{root: directory, files: make(map[string]sourceFile)}
	first, err := loader.source("shared.md", maxMarkdownBytes)
	if err != nil {
		t.Fatal(err)
	}
	writeReportFile(t, filepath.Join(dir, "shared.md"), "Changed notes\n")
	second, err := loader.source("shared.md", maxMarkdownBytes)
	if err != nil {
		t.Fatal(err)
	}
	if string(first.data) != "Original notes\n" || string(second.data) != string(first.data) {
		t.Fatalf("sources = %q, %q, want one consistent snapshot", first.data, second.data)
	}
	if loader.bytes != int64(len(first.data)) {
		t.Fatalf("source bytes = %d, want %d once", loader.bytes, len(first.data))
	}
}

func TestReadAcceptsLimits(t *testing.T) {
	t.Run("eight series", func(t *testing.T) {
		manifest := "blocks: [{type: line, title: Metrics, source: results.csv, x: step, y: [a,b,c,d,e,f,g,h]}]\n"
		root, _ := reportFixture(t, manifest, map[string]string{"results.csv": "step,a,b,c,d,e,f,g,h\n0,1,2,3,4,5,6,7,8\n"})
		report, err := Read(root, "example", catalog.Layout{})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Blocks[0].Data.Series) != 8 {
			t.Fatal("expected all eight selected series")
		}
	})
	t.Run("10000 rows and 100000 selected values", func(t *testing.T) {
		var csv strings.Builder
		csv.WriteString("step,loss\n")
		for i := range 10000 {
			fmt.Fprintf(&csv, "%d,1\n", i)
		}
		manifest := "blocks:\n" + strings.Repeat("  - {type: line, title: Loss, source: results.csv, x: step, y: [loss]}\n", 5)
		root, _ := reportFixture(t, manifest, map[string]string{"results.csv": csv.String()})
		report, err := Read(root, "example", catalog.Layout{})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Blocks) != 5 || report.Blocks[4].Data.TotalRows != 10000 || len(report.Blocks[4].Data.X) > maxPlotRows {
			t.Fatal("expected a bounded projection of every CSV row")
		}
	})
	t.Run("64 blocks", func(t *testing.T) {
		manifest := "blocks:\n" + strings.Repeat("  - {type: markdown, source: intro.md}\n", 64)
		root, _ := reportFixture(t, manifest, map[string]string{"intro.md": "Hello"})
		report, err := Read(root, "example", catalog.Layout{})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Blocks) != 64 {
			t.Fatal("expected all 64 blocks")
		}
	})
}
