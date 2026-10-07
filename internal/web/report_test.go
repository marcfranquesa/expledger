package web_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/marcfranquesa/expledger/internal/experiment"
	"github.com/marcfranquesa/expledger/internal/report"
	"github.com/marcfranquesa/expledger/internal/web"
)

func TestRenderPortableReportAndSafeMarkdown(t *testing.T) {
	value := 1.25
	seriesName := `loss </script><img src=x onerror=alert(1)>`
	records := []experiment.Record{{ID: "trial & 雪", Title: `Trial <script>alert(1)</script>`}, {ID: "plain", Title: "Plain"}}
	layout := &report.Report{Blocks: []report.Block{
		{Type: "markdown", Markdown: "# Introduction\n\n**Measured** results.\n\n<script>alert(1)</script>\n\n[bad](javascript:alert%281%29) <javascript:alert(1)>\n\n[local](results/metrics.csv)\n\n[official](https://example.com/docs?q=1&lang=en)\n\n![Plot & example](https://example.com/private.png)\n"},
		{Type: "row", Blocks: []report.Block{{Type: "line", Title: `Loss <img src=x>`, X: `step "<&`, Data: &report.Data{X: []float64{0, 1}, Series: []report.Series{{Name: seriesName, Values: []*float64{&value, nil}}}}}, {Type: "line", Title: "Throughput", Message: "Results unavailable"}}},
		{Type: "markdown", Markdown: "| Mechanism | Contrast | Evidence |\n| --- | ---: | --- |\n| **Prefix** | 0.125 | [local](results/metrics.csv) |\n| Control | -0.5 | [bad](javascript:alert%281%29) ![Plot](plots/control.png) |\n"},
		{Type: "markdown", Markdown: "## Conclusions\n\nMore work needed."},
	}}
	body, err := web.Render(records, web.PageOptions{Project: "Report project", Reports: map[string]*report.Report{records[0].ID: layout}})
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	for _, want := range []string{
		`class="report-link" href="#report-`, `id="reports"`, `class="report-row"`,
		`<table>`, `<thead>`, `<th>Mechanism</th>`, `<tbody>`,
		`<td><strong>Prefix</strong></td>`, `<td style="text-align:right">0.125</td>`, `<td style="text-align:right">-0.5</td>`,
		`<td><span>local (<code>results/metrics.csv</code>)</span>`,
		`<td><span>bad (<code>javascript:alert%281%29</code>)</span>`, `[Image: Plot]`,
		`<h1>Introduction</h1>`, `<strong>Measured</strong>`, `<h2>Conclusions</h2>`,
		`<span>local (<code>results/metrics.csv</code>)</span>`,
		`href="https://example.com/docs?q=1&amp;lang=en"`, `[Image: Plot &amp; example]`,
		`Loss &lt;img src=x&gt;`, `step &#34;&lt;&amp;`, `loss &lt;/script&gt;&lt;img src=x onerror=alert(1)&gt;`,
		`<td>1.25</td>`, `<td>—</td>`, `Results unavailable`, `class="chart-table" open`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("report missing %q", want)
		}
	}
	for _, unsafe := range []string{`<script>alert(1)`, `<img src=x`, `href="javascript:`, `href="results/`, `src="https:`, `fetch(`, `setTimeout(`} {
		if strings.Contains(page, unsafe) {
			t.Errorf("report contains unsafe or nonportable content %q", unsafe)
		}
	}
	if strings.Count(page, `data-report-link>View report`) != 2 {
		t.Error("expected a report link in list and graph, only for the experiment with a report")
	}
	match := regexp.MustCompile(`<script type="application/json" class="chart-data">([^<]+)</script>`).FindStringSubmatch(page)
	if len(match) != 2 {
		t.Fatal("embedded chart data missing or contains literal markup")
	}
	var data report.Data
	if err := json.Unmarshal([]byte(match[1]), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.X) != 2 || data.Series[0].Name != seriesName || data.Series[0].Values[1] != nil {
		t.Fatalf("chart data changed: %+v", data)
	}
	intro, row, conclusion := strings.Index(page, `<h1>Introduction`), strings.Index(page, `class="report-row"`), strings.Index(page, `<h2>Conclusions`)
	if !(intro < row && row < conclusion) {
		t.Fatal("report block order changed")
	}
}

func TestReportURLStableAcrossCatalogOrder(t *testing.T) {
	layout := &report.Report{Blocks: []report.Block{{Type: "markdown", Markdown: "Report text"}}}
	record := experiment.Record{ID: "trial & 雪", Title: "Trial"}
	urls := []string{}
	for _, records := range [][]experiment.Record{{record, {ID: "other"}}, {{ID: "other"}, record}} {
		body, err := web.Render(records, web.PageOptions{Reports: map[string]*report.Report{record.ID: layout}})
		if err != nil {
			t.Fatal(err)
		}
		match := regexp.MustCompile(`class="report-link" href="([^"]+)"`).FindStringSubmatch(string(body))
		if len(match) != 2 {
			t.Fatal("report link missing")
		}
		urls = append(urls, match[1])
	}
	if urls[0] != urls[1] {
		t.Fatalf("report URL changed with order: %v", urls)
	}
}

func TestReportLegendUsesPlotStroke(t *testing.T) {
	value := 1.0
	series := make([]report.Series, 8)
	for i := range series {
		series[i] = report.Series{Name: "Series", Values: []*float64{&value}}
	}
	layout := &report.Report{Blocks: []report.Block{{Type: "line", Title: "Lines", X: "step", Data: &report.Data{X: []float64{0}, Series: series}}}}
	body, err := web.Render([]experiment.Record{{ID: "trial"}}, web.PageOptions{Reports: map[string]*report.Report{"trial": layout}})
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	swatches := regexp.MustCompile(`<li class="series-[0-7]"><svg class="chart-swatch"[^>]*><line class="chart-series"[^>]*/></svg>`).FindAllString(page, -1)
	if len(swatches) != len(series) {
		t.Fatalf("got %d SVG line legend swatches, want %d", len(swatches), len(series))
	}
	if !strings.Contains(page, `[data-chart-type="line"] .chart-series { stroke: var(--series-color); stroke-dasharray: var(--series-dash);`) {
		t.Fatal("plot and legend lines do not share their scoped color and dash rule")
	}
	if strings.Contains(page, "border-top-style:") {
		t.Fatal("legend has a separate approximate dash pattern")
	}
	dashRules := regexp.MustCompile(`[^{}]+\{ --series-dash: [^}]+}`).FindAllString(page, -1)
	if len(dashRules) != len(series) {
		t.Fatalf("got %d series dash rules, want %d", len(dashRules), len(series))
	}
	for _, rule := range dashRules {
		if !strings.Contains(rule, `[data-chart-type="line"]`) {
			t.Errorf("line dash pattern applies to other chart types: %s", rule)
		}
	}
}
