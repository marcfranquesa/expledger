package web_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/marcfranquesa/expledger/internal/experiment"
	"github.com/marcfranquesa/expledger/internal/report"
	"github.com/marcfranquesa/expledger/internal/web"
)

func TestRenderEmbeddedImageBlocks(t *testing.T) {
	var pngData, jpegData bytes.Buffer
	pixels := image.NewGray(image.Rect(0, 0, 3, 2))
	if err := png.Encode(&pngData, pixels); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegData, pixels, nil); err != nil {
		t.Fatal(err)
	}
	alt := `A figure " onerror="alert(1) <script>& result`
	title := `Caption <img src=x onerror=alert(1)> & result`
	manifest := fmt.Sprintf(`blocks:
  - {type: markdown, source: intro.md}
  - type: row
    blocks:
      - {type: image, source: figure.jpg, alt: %q, title: %q}
      - {type: image, source: photo.png, alt: Photograph}
  - {type: image, source: missing.png, alt: Pending figure, title: Pending caption}
  - {type: markdown, source: end.md}
`, alt, title)
	root := t.TempDir()
	dir := filepath.Join(root, "experiments", "trial")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"report.yaml": []byte(manifest),
		"intro.md":    []byte("Introduction\n\n![Remote](https://example.com/private.png) ![Local](figure.jpg)"),
		"end.md":      []byte("Conclusion"),
		"figure.jpg":  pngData.Bytes(),
		"photo.png":   jpegData.Bytes(),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	layout, err := report.Read(root, "trial", catalog.Layout{})
	if err != nil {
		t.Fatal(err)
	}
	body, err := web.Render([]experiment.Record{{ID: "trial"}}, web.PageOptions{Reports: map[string]*report.Report{"trial": layout}})
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	images := regexp.MustCompile(`<img src="data:(image/(?:png|jpeg));base64,([^"]+)" alt="([^"]*)" width="3" height="2">`).FindAllStringSubmatch(page, -1)
	if len(images) != 2 {
		t.Fatalf("got %d embedded images, want two safe data URLs with intrinsic dimensions: %s", len(images), regexp.MustCompile(`<img[^>]+>`).FindAllString(page, -1))
	}
	for i, tc := range []struct {
		mime string
		data []byte
	}{{"image/png", pngData.Bytes()}, {"image/jpeg", jpegData.Bytes()}} {
		decoded, err := base64.StdEncoding.DecodeString(html.UnescapeString(images[i][2]))
		if err != nil || images[i][1] != tc.mime || !bytes.Equal(decoded, tc.data) {
			t.Errorf("embedded image %d changed format or bytes", i)
		}
	}
	for _, want := range []string{
		`alt="A figure &#34; onerror=&#34;alert(1) &lt;script&gt;&amp; result"`,
		`Caption &lt;img src=x onerror=alert(1)&gt; &amp; result`,
		`Pending caption`, `Image unavailable`, `[Image: Remote]`, `[Image: Local]`,
		`.report-image img { display: block; max-width: 100%; height: auto;`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("image report missing %q", want)
		}
	}
	for _, unsafe := range []string{`src="https:`, `src="figure.jpg"`, `src="photo.png"`, `<script>alert(1)`, `<img src=x`, `#ZgotmplZ`} {
		if strings.Contains(page, unsafe) {
			t.Errorf("image report contains unsafe or nonportable %q", unsafe)
		}
	}
	if strings.Count(page, `<figure class="report-image"`) != 3 || strings.Count(page, `<figcaption>`) != 2 {
		t.Fatal("image figures did not preserve optional captions and missing-image placeholder")
	}
	intro, row, missing, end := strings.Index(page, `<p>Introduction`), strings.Index(page, `class="report-row"`), strings.Index(page, `Pending caption`), strings.Index(page, `<p>Conclusion`)
	if !(intro < row && row < missing && missing < end) {
		t.Fatal("image report block order changed")
	}
}

func TestRenderRejectsUntrustedImageMIME(t *testing.T) {
	for _, mime := range []string{"image/svg+xml", "text/html", `image/png" onerror="alert(1)`, "image/png;anything"} {
		t.Run(mime, func(t *testing.T) {
			layout := &report.Report{Blocks: []report.Block{{Type: "image", Alt: "Figure", Image: &report.Image{MIME: mime, Data: []byte("unsafe"), Width: 1, Height: 1}}}}
			if _, err := web.Render([]experiment.Record{{ID: "trial"}}, web.PageOptions{Reports: map[string]*report.Report{"trial": layout}}); err == nil || !strings.Contains(err.Error(), "unsupported image MIME type") {
				t.Fatalf("untrusted MIME error = %v", err)
			}
		})
	}
}
