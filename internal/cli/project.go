package cli

import (
	"context"
	"fmt"

	"github.com/kildenhq/kilden-cli/internal/api"
	"github.com/spf13/cobra"
)

func projectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Short:   "List the current team's projects and choose the active one",
		Aliases: []string{"projects", "proj"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listProjects(cmd.Context())
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "List the current team's projects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listProjects(cmd.Context())
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "use [project-id]",
		Short: "Switch the active project (interactive if no id given)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			if len(args) == 1 {
				id = args[0]
			}
			return useProject(cmd.Context(), id)
		},
	})

	var name, timezone string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a project in the current team",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return createProject(cmd.Context(), name, timezone)
		},
	}
	create.Flags().StringVar(&name, "name", "", "Project name (prompted if omitted)")
	create.Flags().StringVar(&timezone, "timezone", "UTC", "IANA timezone, e.g. America/Santiago")
	cmd.AddCommand(create)

	return cmd
}

// currentTeamID reads the active team from the account context.
func currentTeamID(ctx context.Context, client *api.Client) (int, error) {
	cur, err := client.Context(ctx)
	if err != nil {
		return 0, err
	}
	if cur.Team == nil {
		return 0, fmt.Errorf("no active team — run `kd team use` first")
	}
	return cur.Team.ID, nil
}

func listProjects(ctx context.Context) error {
	client, _, err := mustClient()
	if err != nil {
		return err
	}
	teamID, err := currentTeamID(ctx, client)
	if err != nil {
		return err
	}
	projects, err := client.Projects(ctx, teamID)
	if err != nil {
		return err
	}
	cur, _ := client.Context(ctx)

	tw := newTab()
	fmt.Fprintln(tw, "\tID\tNAME\tTIMEZONE")
	for _, p := range projects {
		marker := " "
		if cur != nil && cur.Project != nil && cur.Project.ID == p.ID {
			marker = "*"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", marker, p.ID, p.Name, p.Timezone)
	}
	return tw.Flush()
}

func useProject(ctx context.Context, id string) error {
	client, _, err := mustClient()
	if err != nil {
		return err
	}
	teamID, err := currentTeamID(ctx, client)
	if err != nil {
		return err
	}
	projects, err := client.Projects(ctx, teamID)
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		return fmt.Errorf("this team has no projects yet — run `kd project create`")
	}

	project, err := pickProject(projects, id)
	if err != nil {
		return err
	}
	if _, err := client.SwitchProject(ctx, project.ID); err != nil {
		return err
	}
	fmt.Printf("Active project is now %s.\n", project.Name)
	return nil
}

func pickProject(projects []api.Project, id string) (*api.Project, error) {
	if id != "" {
		for i := range projects {
			if projects[i].ID == id {
				return &projects[i], nil
			}
		}
		return nil, fmt.Errorf("project %q is not in the current team", id)
	}

	labels := make([]string, len(projects))
	for i, p := range projects {
		labels[i] = p.Name
	}
	idx, err := selectFrom("Select a project:", labels)
	if err != nil {
		return nil, err
	}
	return &projects[idx], nil
}

func createProject(ctx context.Context, name, timezone string) error {
	client, _, err := mustClient()
	if err != nil {
		return err
	}
	teamID, err := currentTeamID(ctx, client)
	if err != nil {
		return err
	}

	if name == "" {
		name, err = prompt("Project name: ")
		if err != nil {
			return err
		}
	}
	if name == "" {
		return fmt.Errorf("a project name is required")
	}

	project, err := client.CreateProject(ctx, teamID, name, timezone)
	if err != nil {
		return err
	}
	fmt.Printf("Created project %s (%s).\n", project.Name, project.ID)
	if project.PublicKey != "" {
		fmt.Printf("Public write key: %s\n", project.PublicKey)
	}
	return nil
}
