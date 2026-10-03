package web_test

import (
	"strings"
	"testing"
	"time"

	"github.com/marcfranquesa/expledger/internal/experiment"
	"github.com/marcfranquesa/expledger/internal/web"
)

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
		records := []experiment.Record{{ID: "trial ?#% 雪", Title: "Trial", CreatedAt: time.Now()}}
		body, err := web.Render(records, web.PageOptions{RemoteURL: base})
		if err != nil {
			t.Fatal(err)
		}
		want := `href="https://example.com/space%20name/experiments/trial%20%3F%23%25%20%E9%9B%AA"`
		if strings.Count(string(body), want) != 2 {
			t.Fatalf("expected escaped link in both list and graph: %s", want)
		}
		if strings.Contains(string(body), `class="branch"`) {
			t.Fatal("inferred branch badge")
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
