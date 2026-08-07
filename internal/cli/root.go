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
		Long:          "kd is the Kilden command-line. Sign in with `kd login`, then inspect\nyour teams and projects, query events, manage write keys, and keep cohorts,\nflags, insights, in-app units, campaigns and experiments as code with\n`kd apply`.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}

	root.AddCommand(
		loginCmd(),
		initCmd(),
		logoutCmd(),
		whoamiCmd(),
		teamCmd(),
		projectCmd(),
		eventsCmd(),
		propertiesCmd(),
		tailCmd(),
		keyCmd(),
		identitySecretCmd(),
		insightCmd(),
		cohortCmd(),
		flagCmd(),
		unitCmd(),
		campaignCmd(),
		experimentCmd(),
		applyCmd(),
	)

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
