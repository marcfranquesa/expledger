package web_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/experiment"
	"github.com/marcfranquesa/expledger/internal/web"
)

func TestRenderInlineAssets(t *testing.T) {
	for _, live := range []bool{false, true} {
		name := "static"
		if live {
			name = "live"
		}
		t.Run(name, func(t *testing.T) {
			body, err := web.Render([]experiment.Record{{ID: "trial", Title: "Trial"}}, web.PageOptions{Live: live})
			if err != nil {
				t.Fatal(err)
			}
			page := string(body)
			if regexp.MustCompile(`<(script|link)\b[^>]*(src=|rel="stylesheet")`).MatchString(page) {
				t.Fatal("page requires an external script or stylesheet")
			}
			scripts := regexp.MustCompile(`(?s)<script type="text/javascript">(.*?)</script>`).FindAllStringSubmatch(page, -1)
			want := []string{"function initializeGraph(", "const Charts =", "function initializeLineChart(", "function initializeReports("}
			if live {
				want = append(want, "function initializeLive(")
			}
			want = append(want, "(() => {")
			if len(scripts) != len(want) {
				t.Fatalf("got %d inline scripts, want %d", len(scripts), len(want))
			}
			for i, marker := range want {
				if !strings.Contains(scripts[i][1], marker) {
					t.Errorf("inline script %d missing dependency %q", i, marker)
				}
			}
			for _, marker := range []string{"fetch(", "setTimeout(", `class="refresh-status" role="status"`} {
				if strings.Contains(page, marker) != live {
					t.Errorf("live content %q inclusion does not match Live=%t", marker, live)
				}
			}
		})
	}
}

func TestRenderInMemory(t *testing.T) {
	records := []experiment.Record{
		{
			ID: "first # record & notes", Title: `<script>alert("x")</script> & trial`,
			CreatedAt: time.Date(2026, 9, 24, 12, 30, 0, 123456789, time.FixedZone("local", -4*60*60)),
		},
		{
			ID: "second", Title: "Later timestamp, supplied second",
			CreatedAt: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC),
		},
	}
	body, err := web.Render(records, web.PageOptions{
		Project: "Explicit / project & notes", RemoteURL: "https://github.com/owner/repo/tree/research%2Fnext/experiments/",
	})
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	for _, want := range []string{
		"Explicit / project &amp; notes",
		"&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt; &amp; trial",
		"first # record &amp; notes",
		`href="https://github.com/owner/repo/tree/research%2Fnext/experiments/first%20%23%20record%20&amp;%20notes"`,
		`datetime="2026-09-24T16:30:00.123456789Z"`,
		"Sep 24, 2026 · 16:30:00 UTC",
		"Later timestamp, supplied second",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page missing %q", want)
		}
	}
	if strings.Contains(page, "<script>") {
		t.Error("page contains an unescaped script tag")
	}
	if strings.Index(page, "first # record &amp; notes") >= strings.Index(page, "Later timestamp, supplied second") {
		t.Fatal("renderer changed the supplied record order")
	}
}

func TestRenderWithoutRemoteLinks(t *testing.T) {
	records := []experiment.Record{{ID: "local", Title: "Local experiment", CreatedAt: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)}}
	options := web.PageOptions{Project: "Local"}
	body, err := web.Render(records, options)
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	if !strings.Contains(page, "Local experiment") || strings.Contains(page, `class="remote-link"`) || strings.Contains(page, "href=") {
		t.Fatalf("local catalog contains missing content or remote links: %s", page)
	}
	if strings.Contains(page, `class="branch"`) {
		t.Fatal("catalog contains an inferred branch badge")
	}
}

func TestRenderRemoteDirectoryAndEscapedID(t *testing.T) {
	for _, base := range []string{"https://example.com/space%20name/experiments", "https://example.com/space%20name/experiments/"} {
		for _, tt := range []struct{ id, escaped string }{
			{"trial ?#% 雪", "trial%20%3F%23%25%20%E9%9B%AA"},
			{"2026-10-06/trial ?#% 雪", "2026-10-06/trial%20%3F%23%25%20%E9%9B%AA"},
			{"group ?#% 雪/trial ?#% 雪", "group%20%3F%23%25%20%E9%9B%AA/trial%20%3F%23%25%20%E9%9B%AA"},
		} {
			records := []experiment.Record{{ID: tt.id, Title: "Trial", CreatedAt: time.Now()}}
			body, err := web.Render(records, web.PageOptions{RemoteURL: base})
			if err != nil {
				t.Fatal(err)
			}
			want := `href="https://example.com/space%20name/experiments/` + tt.escaped + `"`
			if strings.Count(string(body), want) != 2 {
				t.Fatalf("expected escaped link in both list and graph: %s", want)
			}
			if strings.Contains(string(body), `class="branch"`) {
				t.Fatal("inferred branch badge")
			}
		}
	}
}

func TestGraphEscapingAndReferences(t *testing.T) {
	records := []experiment.Record{{ID: `child\"><svg onload=alert(1)>`, Title: `研究 & <img src=x onerror=alert(1)>`, BasedOn: []string{`parent\"><script>alert(1)</script>`, `parent\"><script>alert(1)</script>`}}}
	body, err := web.Render(records, web.PageOptions{Project: "Graph", RemoteURL: "javascript:alert(1)"})
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	for _, unsafe := range []string{`<svg onload`, `<img src=x`, `<script>alert(1)`, `href="javascript:`} {
		if strings.Contains(page, unsafe) {
			t.Errorf("unsafe metadata: %s", unsafe)
		}
	}
	for _, want := range []string{`Missing experiment`, `研究 &amp; &lt;img`, `parent\&#34;&gt;&lt;script&gt;`} {
		if !strings.Contains(page, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Count(page, `<path data-from=`) != 1 {
		t.Fatal("duplicate parent edges")
	}
	if strings.Count(page, `data-node=`) != 2 {
		t.Fatal("missing known or reference node")
	}
}

func TestGraphVerticalMarkup(t *testing.T) {
	body, err := web.Render([]experiment.Record{{ID: "root", Title: "Root"}, {ID: "child", Title: "Child", BasedOn: []string{"root"}}}, web.PageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	if strings.Count(page, `class="graph-row"`) != 2 {
		t.Fatal("expected two dependency rows")
	}
	for _, want := range []string{`data-rank="1"`, `aria-labelledby="graph-title-0 graph-id-0"`, `id="graph-title-0"`, `id="graph-id-0"`} {
		if !strings.Contains(page, want) {
			t.Errorf("missing accessible node label: %s", want)
		}
	}
}

func TestRenderSourceRemoteLinksAndOfflinePage(t *testing.T) {
	records := []experiment.Record{
		{ID: "local", Title: "Local"},
		{ID: "other", Title: "Other"},
		{ID: "fallback", Title: "Fallback"},
	}
	body, err := web.Render(records, web.PageOptions{
		Project: "Sources", RemoteURL: "https://example.com/default",
		RemoteURLs: map[string]string{"local": "", "other": "https://example.com/other/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	for _, link := range []string{"https://example.com/other/other", "https://example.com/default/fallback"} {
		if got := strings.Count(page, `href="`+link+`"`); got != 2 {
			t.Errorf("link %q appears %d times; want list and graph", link, got)
		}
	}
	for _, absent := range []string{"https://example.com/default/local", "https://example.com/default/other", "fetch(", "setTimeout(", "refresh-status\" role"} {
		if strings.Contains(page, absent) {
			t.Errorf("static page contains %q", absent)
		}
	}
}
