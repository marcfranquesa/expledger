package web

import (
	_ "embed"
	"html/template"
)

//go:embed templates/index.html
var indexHTML string

//go:embed templates/report.html
var reportHTML string

//go:embed assets/styles/catalog.css
var catalogCSS string

//go:embed assets/styles/report.css
var reportCSS string

//go:embed assets/styles/palette.css
var paletteCSS string

//go:embed assets/charts/line.css
var lineCSS string

//go:embed assets/graph.js
var graphJS string

//go:embed assets/charts/common.js
var chartsJS string

//go:embed assets/charts/line.js
var lineJS string

//go:embed assets/reports.js
var reportsJS string

//go:embed assets/live.js
var liveJS string

//go:embed assets/app.js
var appJS string

var indexTemplate = template.Must(template.Must(template.New("index").Parse(indexHTML)).Parse(reportHTML))

func pageAssets(live bool) ([]template.CSS, []template.JS) {
	styles := []template.CSS{template.CSS(catalogCSS), template.CSS(reportCSS), template.CSS(paletteCSS), template.CSS(lineCSS)}
	scripts := []template.JS{template.JS(graphJS), template.JS(chartsJS), template.JS(lineJS), template.JS(reportsJS)}
	if live {
		scripts = append(scripts, template.JS(liveJS))
	}
	return styles, append(scripts, template.JS(appJS))
}
