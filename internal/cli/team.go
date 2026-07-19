package cli

import (
	"context"
	"fmt"

	"github.com/kildenhq/kilden-cli/internal/api"
	"github.com/spf13/cobra"
)

func teamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "team",
		Short:   "List your teams and choose the active one",
		Aliases: []string{"teams"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listTeams(cmd.Context())
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "List your teams",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listTeams(cmd.Context())
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "use [team-id]",
		Short: "Switch the active team (interactive if no id given)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			if len(args) == 1 {
				id = args[0]
			}
			return useTeam(cmd.Context(), id)
		},
	})
	return cmd
}

func listTeams(ctx context.Context) error {
	client, _, err := mustClient()
	if err != nil {
		return err
	}
	teams, err := client.Teams(ctx)
	if err != nil {
		return err
	}

	tw := newTab()
	fmt.Fprintln(tw, "\tID\tNAME\tROLE")
	for _, t := range teams {
		marker := " "
		if t.IsCurrent {
			marker = "*"
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", marker, t.ID, t.Name, t.RoleLabel)
	}
	return tw.Flush()
}

func useTeam(ctx context.Context, id string) error {
	client, _, err := mustClient()
	if err != nil {
		return err
	}
	teams, err := client.Teams(ctx)
	if err != nil {
		return err
	}
	if len(teams) == 0 {
		return fmt.Errorf("you are not a member of any team")
	}

	team, err := pickTeam(teams, id)
	if err != nil {
		return err
	}

	if _, err := client.SwitchTeam(ctx, team.ID); err != nil {
		return err
	}
	fmt.Printf("Active team is now %s.\n", team.Name)
	return nil
}

func pickTeam(teams []api.Team, id string) (*api.Team, error) {
	if id != "" {
		for i := range teams {
			if fmt.Sprintf("%d", teams[i].ID) == id {
				return &teams[i], nil
			}
		}
		return nil, fmt.Errorf("team %q is not one of yours", id)
	}

	labels := make([]string, len(teams))
	for i, t := range teams {
		labels[i] = fmt.Sprintf("%s (%s)", t.Name, t.RoleLabel)
	}
	idx, err := selectFrom("Select a team:", labels)
	if err != nil {
		return nil, err
	}
	return &teams[idx], nil
}
