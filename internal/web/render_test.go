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
		Project: "Explicit / project & notes", RepositoryURL: "https://github.com/owner/repo", Branch: "research/next",
	})
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	for _, want := range []string{
		"Explicit / project &amp; notes",
		"&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt; &amp; trial",
		"first # record &amp; notes",
		"research/next",
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
	for _, options := range []web.PageOptions{
		{Project: "Local", Branch: "main"},
		{Project: "Detached", RepositoryURL: "https://github.com/owner/repo"},
		{Project: "Local"},
	} {
		body, err := web.Render(records, options)
		if err != nil {
			t.Fatal(err)
		}
		page := string(body)
		if !strings.Contains(page, "Local experiment") || strings.Contains(page, `class="github"`) || strings.Contains(page, "href=") {
			t.Fatalf("local catalog contains missing content or remote links: %s", page)
		}
		if options.Branch == "" && strings.Contains(page, `class="branch"`) {
			t.Fatal("detached catalog contains an empty branch badge")
		}
	}
}
