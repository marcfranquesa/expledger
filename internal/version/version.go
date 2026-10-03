// Package version identifies the application independently of its record schema.
package version

import (
	"regexp"
	"runtime/debug"
	"strings"
)

// release is set only by release tooling via -ldflags -X.
var release string

var pseudoVersion = regexp.MustCompile(`[.-][0-9]{14}-[0-9a-f]{12}(\+incompatible)?$`)

func String() string {
	info, _ := debug.ReadBuildInfo()
	return resolve(release, info)
}

func resolve(embedded string, info *debug.BuildInfo) string {
	if embedded != "" {
		return embedded
	}
	if info == nil || info.Main.Replace != nil {
		return "devel"
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return "devel"
	}
	if pseudoVersion.MatchString(v) || strings.HasSuffix(v, "+dirty") {
		return "devel (" + v + ")"
	}
	return v
}
