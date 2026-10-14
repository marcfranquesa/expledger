package report

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marcfranquesa/expledger/internal/catalog"
)

func imageBytes(t *testing.T, format string) []byte {
	t.Helper()
	var encoded bytes.Buffer
	pixels := image.NewRGBA(image.Rect(0, 0, 3, 2))
	pixels.Set(1, 1, color.RGBA{R: 255, A: 255})
	var err error
	switch format {
	case "png":
		err = png.Encode(&encoded, pixels)
	case "jpeg":
		err = jpeg.Encode(&encoded, pixels, nil)
	case "gif":
		err = gif.Encode(&encoded, pixels, nil)
	default:
		t.Fatalf("unknown image fixture format %q", format)
	}
	if err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func imageManifest(source string) string {
	return fmt.Sprintf("blocks: [{type: image, source: %q, alt: 'A measured result'}]\n", source)
}

func TestReadImagesInReportOrder(t *testing.T) {
	pngData, jpegData := imageBytes(t, "png"), imageBytes(t, "jpeg")
	root, _ := reportFixture(t, `blocks:
  - type: markdown
    source: intro.md
  - type: row
    blocks:
      - type: image
        source: ./results/plot.jpg
        alt: A measured result
        title: Measured result
      - type: image
        source: results/photo.png
        alt: Experiment photograph
  - type: image
    source: results/plot.jpg
    alt: Repeated result
`, map[string]string{"intro.md": "Introduction", "results/plot.jpg": string(pngData), "results/photo.png": string(jpegData)})
	content, err := Read(root, "example", catalog.Layout{})
	if err != nil {
		t.Fatal(err)
	}
	if len(content.Blocks) != 3 || content.Blocks[0].Markdown != "Introduction" || content.Blocks[1].Type != "row" {
		t.Fatalf("report order changed: %#v", content)
	}
	first, second := content.Blocks[1].Blocks[0], content.Blocks[1].Blocks[1]
	if first.Source != "results/plot.jpg" || first.Title != "Measured result" || first.Alt != "A measured result" {
		t.Fatalf("image fields changed: %#v", first)
	}
	for _, tc := range []struct {
		block Block
		mime  string
		data  []byte
	}{{first, "image/png", pngData}, {second, "image/jpeg", jpegData}} {
		if tc.block.Image == nil || tc.block.Image.MIME != tc.mime || tc.block.Image.Width != 3 || tc.block.Image.Height != 2 || !bytes.Equal(tc.block.Image.Data, tc.data) {
			t.Fatalf("image = %#v, want preserved %s bytes and dimensions", tc.block.Image, tc.mime)
		}
	}
	if content.Blocks[2].Image != first.Image {
		t.Fatal("repeated normalized image source did not reuse its validated image")
	}
}

func TestImageManifestValidation(t *testing.T) {
	for _, tc := range []struct{ name, block string }{
		{"missing source", "{type: image, alt: Figure}"},
		{"missing alt", "{type: image, source: figure.png}"},
		{"empty alt", "{type: image, source: figure.png, alt: ''}"},
		{"blank alt", "{type: image, source: figure.png, alt: '   '}"},
		{"numeric alt", "{type: image, source: figure.png, alt: 12}"},
		{"null alt", "{type: image, source: figure.png, alt: null}"},
		{"list alt", "{type: image, source: figure.png, alt: [Figure]}"},
		{"empty title", "{type: image, source: figure.png, alt: Figure, title: ''}"},
		{"blank title", "{type: image, source: figure.png, alt: Figure, title: '   '}"},
		{"numeric title", "{type: image, source: figure.png, alt: Figure, title: 12}"},
		{"null title", "{type: image, source: figure.png, alt: Figure, title: null}"},
		{"chart fields", "{type: image, source: figure.png, alt: Figure, x: step}"},
		{"children", "{type: image, source: figure.png, alt: Figure, blocks: []}"},
		{"unknown field", "{type: image, source: figure.png, alt: Figure, width: 50}"},
		{"nested row", "{type: row, blocks: [{type: row, blocks: [{type: image, source: figure.png, alt: Figure}]}]}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parse([]byte("blocks: [" + tc.block + "]\n")); err == nil {
				t.Fatalf("accepted invalid image block %s", tc.block)
			}
		})
	}
	for _, source := range []string{"", "/tmp/figure.png", "../figure.png", "results/../figure.png", "results\\figure.png", "figure\x00.png"} {
		t.Run(fmt.Sprintf("source %q", source), func(t *testing.T) {
			if _, err := parse([]byte(imageManifest(source))); err == nil {
				t.Fatalf("accepted unsafe image source %q", source)
			}
		})
	}
}

func TestReadMissingImage(t *testing.T) {
	root, _ := reportFixture(t, imageManifest("results/not-produced.png"), nil)
	content, err := Read(root, "example", catalog.Layout{})
	if err != nil {
		t.Fatal(err)
	}
	if block := content.Blocks[0]; block.Image != nil || block.Message != "Image unavailable" {
		t.Fatalf("missing image = %#v", block)
	}
}

func pngDimensions(t *testing.T, width, height uint32) []byte {
	t.Helper()
	data := imageBytes(t, "png")
	binary.BigEndian.PutUint32(data[16:20], width)
	binary.BigEndian.PutUint32(data[20:24], height)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	return data
}

func pngChunk(kind string, data []byte) []byte {
	chunk := make([]byte, len(data)+12)
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(data)))
	copy(chunk[4:8], kind)
	copy(chunk[8:], data)
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
	return chunk
}

func TestParseImageRejectsAPNG(t *testing.T) {
	data := imageBytes(t, "png")
	animation := make([]byte, 8)
	binary.BigEndian.PutUint32(animation[:4], 2)
	frame := make([]byte, 26)
	binary.BigEndian.PutUint32(frame[4:8], 3)
	binary.BigEndian.PutUint32(frame[8:12], 2)
	binary.BigEndian.PutUint16(frame[20:22], 1)
	binary.BigEndian.PutUint16(frame[22:24], 10)
	apng := append(bytes.Clone(data[:33]), pngChunk("acTL", animation)...)
	apng = append(apng, pngChunk("fcTL", frame)...)
	apng = append(apng, data[33:len(data)-12]...)
	binary.BigEndian.PutUint32(frame[:4], 1)
	apng = append(apng, pngChunk("fcTL", frame)...)
	second := make([]byte, 4)
	binary.BigEndian.PutUint32(second, 2)
	for offset := 33; offset < len(data)-12; {
		size := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		if string(data[offset+4:offset+8]) == "IDAT" {
			second = append(second, data[offset+8:offset+8+size]...)
		}
		offset += size + 12
	}
	apng = append(apng, pngChunk("fdAT", second)...)
	apng = append(apng, data[len(data)-12:]...)
	if _, _, err := image.Decode(bytes.NewReader(apng)); err != nil {
		t.Fatalf("APNG fixture does not decode as PNG: %v", err)
	}
	if _, err := parseImage(apng); err == nil || !strings.Contains(err.Error(), "animated PNG images are unsupported") {
		t.Fatalf("APNG rejection = %v", err)
	}

	metadata := append(bytes.Clone(data[:33]), pngChunk("tEXt", []byte("Comment\x00acTL appears only in metadata"))...)
	metadata = append(metadata, data[33:]...)
	parsed, err := parseImage(metadata)
	if err != nil || !bytes.Equal(parsed.Data, metadata) {
		t.Fatalf("ordinary PNG metadata containing acTL was rejected: %v", err)
	}
	trailing := append(bytes.Clone(data), pngChunk("acTL", animation)...)
	if _, err := parseImage(trailing); err != nil {
		t.Fatalf("PNG chunks after IEND were inspected: %v", err)
	}
}

func TestParseImageRejectsMalformedPNGChunks(t *testing.T) {
	data := imageBytes(t, "png")
	tooLong := pngChunk("IDAT", nil)
	binary.BigEndian.PutUint32(tooLong[:4], ^uint32(0))
	justTooLong := pngChunk("IDAT", nil)
	binary.BigEndian.PutUint32(justTooLong[:4], 1)
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"short chunk header", append(bytes.Clone(data[:33]), make([]byte, 11)...)},
		{"huge chunk length", append(bytes.Clone(data[:33]), tooLong...)},
		{"chunk length exceeds remaining", append(bytes.Clone(data[:33]), justTooLong...)},
		{"missing IEND", data[:len(data)-12]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseImage(tc.data); err == nil || !strings.Contains(err.Error(), "invalid png image: truncated chunk") {
				t.Fatalf("malformed PNG chunk error = %v", err)
			}
		})
	}
}

func TestReadRejectsInvalidImages(t *testing.T) {
	pngData, jpegData := imageBytes(t, "png"), imageBytes(t, "jpeg")
	corruptPNG := bytes.Clone(pngData)
	corruptPNG[len(corruptPNG)-5] ^= 1
	for _, tc := range []struct {
		name, message string
		data          []byte
	}{
		{"empty", "image file is empty", nil},
		{"text", "valid PNG or JPEG", []byte("not an image")},
		{"SVG", "valid PNG or JPEG", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)},
		{"GIF", "PNG or JPEG", imageBytes(t, "gif")},
		{"truncated PNG", "invalid png image", pngData[:len(pngData)-12]},
		{"corrupt PNG", "invalid png image", corruptPNG},
		{"truncated JPEG", "invalid jpeg image", jpegData[:len(jpegData)-16]},
		{"oversized dimensions", "16000000 pixels", pngDimensions(t, 4001, 4000)},
		{"overflow dimensions", "dimension overflow", pngDimensions(t, 1<<30, 1<<30)},
		{"zero dimensions", "valid PNG or JPEG", pngDimensions(t, 0, 2)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := "blocks: [{type: row, blocks: [{type: image, source: results/figure.png, alt: Figure}]}]\n"
			root, _ := reportFixture(t, manifest, map[string]string{"results/figure.png": string(tc.data)})
			_, err := Read(root, "example", catalog.Layout{})
			if err == nil {
				t.Fatal("accepted invalid image")
			}
			for _, context := range []string{"experiments/example/report.yaml", "blocks[0].blocks[0]", "results/figure.png", tc.message} {
				if !strings.Contains(err.Error(), context) {
					t.Errorf("error %v lacks %q", err, context)
				}
			}
		})
	}
}

func TestReadImageLimits(t *testing.T) {
	t.Run("file bytes", func(t *testing.T) {
		root, dir := reportFixture(t, imageManifest("figure.png"), nil)
		file, err := os.Create(filepath.Join(dir, "figure.png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(maxImageBytes + 1); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(root, "example", catalog.Layout{}); err == nil || !strings.Contains(err.Error(), "8388608 bytes") {
			t.Fatalf("oversized image error = %v", err)
		}
	})
	t.Run("pixel boundary", func(t *testing.T) {
		var data bytes.Buffer
		if err := png.Encode(&data, image.NewGray(image.Rect(0, 0, 4000, 4000))); err != nil {
			t.Fatal(err)
		}
		root, _ := reportFixture(t, imageManifest("figure.png"), map[string]string{"figure.png": data.String()})
		content, err := Read(root, "example", catalog.Layout{})
		if err != nil {
			t.Fatal(err)
		}
		if content.Blocks[0].Image.Width*content.Blocks[0].Image.Height != maxImagePixels {
			t.Fatal("image at pixel boundary changed")
		}
	})
	t.Run("repeated image bytes", func(t *testing.T) {
		data := make([]byte, maxImageBytes)
		copy(data, imageBytes(t, "png"))
		manifest := "blocks:\n" + strings.Repeat("  - {type: image, source: ./figure.png, alt: Figure}\n", 2)
		root, dir := reportFixture(t, manifest, map[string]string{"figure.png": string(data)})
		content, err := Read(root, "example", catalog.Layout{})
		if err != nil || len(content.Blocks) != 2 || content.Blocks[0].Image != content.Blocks[1].Image {
			t.Fatalf("two repeated 8 MiB images = (%#v, %v)", content, err)
		}
		writeReportFile(t, filepath.Join(dir, "report.yaml"), manifest+"  - {type: row, blocks: [{type: image, source: figure.png, alt: Figure}]}\n")
		if _, err := Read(root, "example", catalog.Layout{}); err == nil || !strings.Contains(err.Error(), "image data exceeds 16777216 bytes") {
			t.Fatalf("repeated image budget error = %v", err)
		}
	})
	t.Run("combined unique source bytes", func(t *testing.T) {
		data := make([]byte, 6<<20)
		copy(data, imageBytes(t, "png"))
		manifest := `blocks:
  - {type: image, source: figure.png, alt: Figure}
  - {type: line, title: Loss, source: first.csv, x: step, y: [loss]}
  - {type: line, title: Loss, source: second.csv, x: step, y: [loss]}
`
		csv := "step,loss,unused\n0,1," + strings.Repeat("x", 6<<20) + "\n"
		root, _ := reportFixture(t, manifest, map[string]string{"figure.png": string(data), "first.csv": csv, "second.csv": csv})
		if _, err := Read(root, "example", catalog.Layout{}); err == nil || !strings.Contains(err.Error(), "report sources exceed 16777216 bytes") {
			t.Fatalf("combined source budget error = %v", err)
		}
	})
}

func TestImageLoaderKeepsValidatedSnapshot(t *testing.T) {
	data := imageBytes(t, "png")
	_, dir := reportFixture(t, "", map[string]string{"figure.png": string(data)})
	directory, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	loader := sourceLoader{root: directory, files: make(map[string]sourceFile)}
	blocks := []Block{{Type: "image", Source: "figure.png", Alt: "First"}, {Type: "image", Source: "figure.png", Alt: "Second"}}
	if err := loader.load(blocks[:1], "blocks"); err != nil {
		t.Fatal(err)
	}
	writeReportFile(t, filepath.Join(dir, "figure.png"), "invalid replacement")
	if err := loader.load(blocks[1:], "blocks"); err != nil {
		t.Fatal(err)
	}
	if blocks[0].Image != blocks[1].Image || !bytes.Equal(blocks[1].Image.Data, data) || loader.bytes != len(data) || loader.imageBytes != 2*len(data) {
		t.Fatal("repeated image did not preserve one validated snapshot and per-reference accounting")
	}
}

func TestReadImageFilesystem(t *testing.T) {
	for _, tc := range []struct{ name, target string }{
		{"confined symlink", "actual.png"},
		{"dangling symlink", "missing.png"},
		{"escaping symlink", "../../outside.png"},
		{"directory", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := string(imageBytes(t, "png"))
			root, dir := reportFixture(t, imageManifest("figure.png"), map[string]string{"actual.png": data})
			writeReportFile(t, filepath.Join(root, "outside.png"), data)
			if tc.target == "" {
				if err := os.Mkdir(filepath.Join(dir, "figure.png"), 0o755); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Symlink(tc.target, filepath.Join(dir, "figure.png")); err != nil {
				t.Fatal(err)
			}
			content, err := Read(root, "example", catalog.Layout{})
			if tc.name == "confined symlink" {
				if err != nil || content.Blocks[0].Image == nil {
					t.Fatalf("confined image = (%#v, %v)", content, err)
				}
			} else if err == nil {
				t.Fatal("accepted unresolved, escaping, or nonregular image source")
			}
		})
	}
}
