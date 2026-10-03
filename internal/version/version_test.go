package version

import (
	"runtime/debug"
	"testing"
)

func TestResolve(t *testing.T) {
	for _, tt := range []struct {
		name, embedded, module, want string
	}{
		{name: "local", module: "(devel)", want: "devel"},
		{name: "missing", want: "devel"},
		{name: "release archive", embedded: "v0.1.0", module: "(devel)", want: "v0.1.0"},
		{name: "snapshot", embedded: "devel", module: "v0.1.0", want: "devel"},
		{name: "go install", module: "v0.1.0", want: "v0.1.0"},
		{name: "prerelease", module: "v0.2.0-rc.1", want: "v0.2.0-rc.1"},
		{name: "dirty checkout", module: "v0.1.0+dirty", want: "devel (v0.1.0+dirty)"},
		{name: "dirty pseudo-version", module: "v0.1.1-0.20261003000000-abcdef123456+dirty", want: "devel (v0.1.1-0.20261003000000-abcdef123456+dirty)"},
		{name: "no base", module: "v0.0.0-20261003000000-abcdef123456", want: "devel (v0.0.0-20261003000000-abcdef123456)"},
		{name: "release base", module: "v0.1.1-0.20261003000000-abcdef123456", want: "devel (v0.1.1-0.20261003000000-abcdef123456)"},
		{name: "prerelease base", module: "v0.2.0-rc.1.0.20261003000000-abcdef123456", want: "devel (v0.2.0-rc.1.0.20261003000000-abcdef123456)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			info := &debug.BuildInfo{Main: debug.Module{Version: tt.module}}
			if got := resolve(tt.embedded, info); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
	if got := resolve("", nil); got != "devel" {
		t.Fatalf("missing build info: %q", got)
	}
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0", Replace: &debug.Module{Path: "../local"}}}
	if got := resolve("", info); got != "devel" {
		t.Fatalf("replaced module: %q", got)
	}
}
