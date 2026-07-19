// Command kd is the Kilden command-line client.
package main

import "github.com/kildenhq/kilden-cli/internal/cli"

// version is overridden at build time via -ldflags "-X main.version=…".
var version = "dev"

func main() {
	cli.Execute(version)
}
