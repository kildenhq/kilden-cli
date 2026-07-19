// Package cli wires the `kd` command tree.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

// Execute runs the root command. version is stamped at build time.
func Execute(version string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	root := &cobra.Command{
		Use:           "kd",
		Short:         "Kilden — sign in and manage your account from the terminal",
		Long:          "kd is the Kilden command-line. Sign in with `kd login`, then inspect\nyour teams and projects, query events and manage write keys.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}

	root.AddCommand(
		loginCmd(),
		logoutCmd(),
		whoamiCmd(),
		teamCmd(),
		projectCmd(),
		eventsCmd(),
		propertiesCmd(),
		keyCmd(),
	)

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
