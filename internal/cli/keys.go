package cli

import (
	"context"
	"fmt"

	"github.com/kildenhq/kilden-cli/internal/api"
	"github.com/spf13/cobra"
)

func keyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "key",
		Short:   "List and create write keys for the active project",
		Aliases: []string{"keys"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listKeys(cmd.Context(), "")
		},
	}

	var project string
	ls := &cobra.Command{
		Use:   "ls",
		Short: "List the active project's write keys",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listKeys(cmd.Context(), project)
		},
	}
	ls.Flags().StringVar(&project, "project", "", "Project id (defaults to the active project)")
	cmd.AddCommand(ls)

	var createProject, kind, label string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a write key (its full value is shown only once)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return createKey(cmd.Context(), createProject, kind, label)
		},
	}
	create.Flags().StringVar(&createProject, "project", "", "Project id (defaults to the active project)")
	create.Flags().StringVar(&kind, "kind", "public", "Key kind: public (wk_) or secret (sk_)")
	create.Flags().StringVar(&label, "label", "", "A label to recognize the key later")
	cmd.AddCommand(create)

	return cmd
}

func listKeys(ctx context.Context, projectFlag string) error {
	client, _, err := mustClient()
	if err != nil {
		return err
	}
	projectID, err := currentProjectID(ctx, client, projectFlag)
	if err != nil {
		return err
	}
	keys, err := client.Keys(ctx, projectID)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		fmt.Println("No active keys for this project.")
		return nil
	}

	tw := newTab()
	fmt.Fprintln(tw, "KIND\tLABEL\tKEY")
	for _, k := range keys {
		label := k.Label
		if label == "" {
			label = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", k.Kind, label, k.Key)
	}
	return tw.Flush()
}

func createKey(ctx context.Context, projectFlag, kind, label string) error {
	if kind != api.KindPublic && kind != api.KindSecret {
		return fmt.Errorf("--kind must be %q or %q", api.KindPublic, api.KindSecret)
	}

	client, _, err := mustClient()
	if err != nil {
		return err
	}
	projectID, err := currentProjectID(ctx, client, projectFlag)
	if err != nil {
		return err
	}

	key, err := client.CreateKey(ctx, projectID, kind, label)
	if err != nil {
		return err
	}

	fmt.Printf("Created %s key: %s\n", key.Kind, bold(key.Key))
	if key.Kind == api.KindSecret {
		fmt.Println("Store it now — the full secret is shown only this once.")
	}
	return nil
}
