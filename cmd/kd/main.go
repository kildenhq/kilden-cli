// Command kd is the Kilden command-line client.
package main

import (
	"runtime/debug"
	"strings"

	"github.com/kildenhq/kilden-cli/internal/cli"
)

// version is overridden at build time via -ldflags "-X main.version=…".
// Only goreleaser does that, which is why resolveVersion exists.
var version = "dev"

func main() {
	cli.Execute(resolveVersion(version, debug.ReadBuildInfo))
}

// resolveVersion answers "which kd is this?" for both install paths.
//
// Release tarballs carry the version in an ldflag. `go install` carries no
// ldflags — it never has — but Go records the module version it resolved in the
// binary's build info, so the answer was already there and simply unread. Every
// binary installed the way our own docs recommend used to introduce itself as
// "dev", which is the least useful thing it could say in a bug report.
//
// The stamped value wins when present: goreleaser knows which release it is
// cutting, and build info can lag it.
func resolveVersion(stamped string, read func() (*debug.BuildInfo, bool)) string {
	if stamped != "dev" {
		return stamped
	}

	info, ok := read()
	if !ok {
		return stamped
	}

	// "(devel)" is what a plain `go build` in a checkout reports. It is not a
	// version, and printing it would only look like one.
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return stamped
	}

	// Build info spells it "v0.2.2" and goreleaser spells it "0.2.2". Two
	// install paths reporting the same release differently is a distinction
	// with no meaning to anybody reading it.
	return strings.TrimPrefix(v, "v")
}
