package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func whoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the signed-in account and its active team and project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, creds, err := mustClient()
			if err != nil {
				return err
			}
			me, err := client.Whoami(cmd.Context())
			if err != nil {
				return err
			}

			tw := newTab()
			fmt.Fprintf(tw, "Account:\t%s <%s>\n", me.Name, me.Email)
			if me.CurrentTeam != nil {
				fmt.Fprintf(tw, "Team:\t%s (%s)\n", me.CurrentTeam.Name, me.CurrentTeam.RoleLabel)
			}
			if me.CurrentProject != nil {
				fmt.Fprintf(tw, "Project:\t%s\n", me.CurrentProject.Name)
			} else {
				fmt.Fprintf(tw, "Project:\t(none — run `kd project use`)\n")
			}
			fmt.Fprintf(tw, "Host:\t%s\n", creds.Host)
			return tw.Flush()
		},
	}
}
