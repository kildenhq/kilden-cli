package main

import (
	"runtime/debug"
	"testing"
)

// Where the version comes from when nobody stamped one.
//
// goreleaser injects `-X main.version=…`, so release tarballs always know what
// they are. `go install` injects nothing — it never has — so every binary from
// the documented Go path introduced itself as "dev". That is not cosmetic: it
// is the answer to "what version are you running?" in every bug report we will
// ever receive from someone who installed the way our own docs recommend.
//
// Go records the module version it resolved in the build info, so the answer
// was already in the binary. It just was not being read.
func TestResolveVersion(t *testing.T) {
	buildInfo := func(v string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: v}}, true
		}
	}
	noBuildInfo := func() (*debug.BuildInfo, bool) { return nil, false }

	cases := []struct {
		name    string
		ldflag  string
		read    func() (*debug.BuildInfo, bool)
		want    string
		explain string
	}{
		{
			name:    "a stamped version always wins",
			ldflag:  "0.2.2",
			read:    buildInfo("v0.2.1"),
			want:    "0.2.2",
			explain: "goreleaser knows the release it is cutting; build info can lag it",
		},
		{
			name:    "go install falls back to the resolved module version",
			ldflag:  "dev",
			read:    buildInfo("v0.2.2"),
			want:    "0.2.2",
			explain: "and without the leading v, so both install paths read alike",
		},
		{
			name:    "a local build stays dev",
			ldflag:  "dev",
			read:    buildInfo("(devel)"),
			want:    "dev",
			explain: "`go build` in a checkout has no version to report, and should not invent one",
		},
		{
			name:    "no build info at all stays dev",
			ldflag:  "dev",
			read:    noBuildInfo,
			want:    "dev",
			explain: "nothing to read is not a reason to crash or to lie",
		},
		{
			name:    "an empty module version stays dev",
			ldflag:  "dev",
			read:    buildInfo(""),
			want:    "dev",
			explain: "`kd version ` reads like a bug, because it is one",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveVersion(c.ldflag, c.read); got != c.want {
				t.Errorf("resolveVersion(%q) = %q, want %q — %s", c.ldflag, got, c.want, c.explain)
			}
		})
	}
}
