package web

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"github.com/marcfranquesa/expledger/internal/report"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type reportView struct {
	experimentView
	Anchor string
	Blocks []reportBlockView
}

type reportBlockView struct {
	Type, Title, X, Message, Key string
	Markdown                     template.HTML
	JSON                         template.JS
	Series                       []string
	Rows                         [][]string
	Blocks                       []reportBlockView
}

func reportAnchor(id string) string { return "report-" + hex.EncodeToString([]byte(id)) }

func prepareReport(view experimentView, source *report.Report) (reportView, error) {
	result := reportView{experimentView: view, Anchor: reportAnchor(view.ID)}
	var prepare func([]report.Block, string) ([]reportBlockView, error)
	prepare = func(blocks []report.Block, prefix string) ([]reportBlockView, error) {
		views := make([]reportBlockView, 0, len(blocks))
		for i, block := range blocks {
			key := prefix + "-" + strconv.Itoa(i)
			item := reportBlockView{Type: block.Type, Title: block.Title, X: block.X, Message: block.Message, Key: key}
			switch block.Type {
			case "markdown":
				var body bytes.Buffer
				if err := reportMarkdown.Convert([]byte(block.Markdown), &body); err != nil {
					return nil, err
				}
				item.Markdown = template.HTML(body.String())
			case "row":
				children, err := prepare(block.Blocks, key)
				if err != nil {
					return nil, err
				}
				item.Blocks = children
			case "line":
				if block.Data != nil {
					encoded, err := json.Marshal(block.Data)
					if err != nil {
						return nil, err
					}
					item.JSON = template.JS(encoded)
					for _, series := range block.Data.Series {
						item.Series = append(item.Series, series.Name)
					}
					for j, x := range block.Data.X {
						row := []string{strconv.FormatFloat(x, 'g', -1, 64)}
						for _, series := range block.Data.Series {
							value := "—"
							if j < len(series.Values) && series.Values[j] != nil {
								value = strconv.FormatFloat(*series.Values[j], 'g', -1, 64)
							}
							row = append(row, value)
						}
						item.Rows = append(item.Rows, row)
					}
				}
			default:
				return nil, fmt.Errorf("unsupported block type %q", block.Type)
			}
			views = append(views, item)
		}
		return views, nil
	}
	var err error
	result.Blocks, err = prepare(source.Blocks, result.Anchor)
	return result, err
}

// Images are represented by their alt text: report pages never load an asset
// that is unavailable in the single-file build or initiate remote image requests.
type reportAssetRenderer struct{}

func (reportAssetRenderer) RegisterFuncs(register renderer.NodeRendererFuncRegisterer) {
	register.Register(ast.KindImage, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			_, err := w.WriteString("<span class=\"report-image-alt\">[Image: " + html.EscapeString(string(node.Text(source))) + "]</span>")
			return ast.WalkSkipChildren, err
		}
		return ast.WalkContinue, nil
	})
	register.Register(ast.KindLink, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		link := node.(*ast.Link)
		if entering {
			if !safeReportURL(string(link.Destination)) {
				_, err := w.WriteString("<span>" + html.EscapeString(string(node.Text(source))) + " (<code>" + html.EscapeString(string(link.Destination)) + "</code>)</span>")
				return ast.WalkSkipChildren, err
			}
			_, err := w.WriteString("<a href=\"" + html.EscapeString(string(util.URLEscape(link.Destination, true))) + "\">")
			return ast.WalkContinue, err
		}
		_, err := w.WriteString("</a>")
		return ast.WalkContinue, err
	})
	register.Register(ast.KindAutoLink, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link := node.(*ast.AutoLink)
		destination := string(link.URL(source))
		if link.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(strings.ToLower(destination), "mailto:") {
			destination = "mailto:" + destination
		}
		label := html.EscapeString(string(link.Label(source)))
		if safeReportURL(destination) {
			label = "<a href=\"" + html.EscapeString(string(util.URLEscape([]byte(destination), false))) + "\">" + label + "</a>"
		}
		_, err := w.WriteString(label)
		return ast.WalkSkipChildren, err
	})
}

func safeReportURL(destination string) bool {
	if strings.HasPrefix(destination, "#") {
		return true
	}
	parsed, err := url.Parse(destination)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.Host != ""
	case "mailto":
		return true
	}
	return false
}

var reportMarkdown = goldmark.New(goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(reportAssetRenderer{}, 100))))
