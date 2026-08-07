package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The installed binary must be called `kd`.
//
// Every command in the README, in docs.kilden.io and in the panel's own
// onboarding is spelled `kd …`, and the release tarballs ship a binary with
// that name. `go install` does not read any of that: it names the binary after
// the last element of the package's import path. With `package main` at the
// module root that element is `kilden-cli`, so the Go install path handed
// people a binary whose name matched no documented command — and nothing
// failed, because installing succeeded.
//
// This is the only invariant worth pinning, and it cannot be pinned by reading
// the source: it is a property of where the package sits.
func TestGoInstallProducesKd(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles the whole module")
	}

	bin := t.TempDir()

	// The import path and not `.`: the test asserts what somebody typing the
	// README's install line gets, and that name comes from the path.
	const pkg = "github.com/kildenhq/kilden-cli/cmd/kd"

	cmd := exec.Command("go", "install", pkg)
	cmd.Env = append(os.Environ(), "GOBIN="+bin)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go install %s: %v\n%s", pkg, err, out)
	}

	entries, err := os.ReadDir(bin)
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}

	if len(names) != 1 || filepath.Base(names[0]) != "kd" {
		t.Fatalf("installed binary is %v, want [kd]", names)
	}
}
