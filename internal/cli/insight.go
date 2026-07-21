package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/kildenhq/kilden-cli/internal/api"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// applyDoc is the config-as-code spec `kd apply` reads. It is deliberately a
// top-level map of resource lists so it can grow (dashboards, campaigns…)
// without breaking existing specs.
type applyDoc struct {
	Insights []api.Insight `json:"insights" yaml:"insights"`
}

// parseApplyDoc decodes a YAML or JSON spec. YAML is a JSON superset, so one
// decoder handles both.
func parseApplyDoc(raw []byte) (applyDoc, error) {
	var doc applyDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return doc, fmt.Errorf("parse spec: %w", err)
	}
	return doc, nil
}

// applyCmd is the declarative entry point: `kd apply -f spec.yaml`.
func applyCmd() *cobra.Command {
	var file, project string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Create or update config from a spec file (idempotent)",
		Long: "Apply a YAML or JSON spec of saved insights. Re-applying the same spec\n" +
			"updates each resource in place by slug and never duplicates. Read from a\n" +
			"file with -f, or from stdin with `-f -`.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return applySpec(cmd.Context(), file, project)
		},
	}
	cmd.Flags().StringVarP(&file, "filename", "f", "", "Spec file (YAML or JSON); - for stdin")
	cmd.Flags().StringVar(&project, "project", "", "Project id (defaults to the active project)")
	_ = cmd.MarkFlagRequired("filename")
	return cmd
}

func applySpec(ctx context.Context, file, projectFlag string) error {
	raw, err := readInput(file)
	if err != nil {
		return err
	}
	doc, err := parseApplyDoc(raw)
	if err != nil {
		return err
	}
	if len(doc.Insights) == 0 {
		return fmt.Errorf("the spec has no insights to apply")
	}
	for i, in := range doc.Insights {
		if in.Slug == "" {
			return fmt.Errorf("insights[%d]: every insight needs a slug (its stable identity)", i)
		}
	}

	client, _, err := mustClient()
	if err != nil {
		return err
	}
	projectID, err := currentProjectID(ctx, client, projectFlag)
	if err != nil {
		return err
	}

	for _, in := range doc.Insights {
		stored, err := client.ApplyInsight(ctx, projectID, in.Slug, in)
		if err != nil {
			return fmt.Errorf("apply insight %q: %w", in.Slug, err)
		}
		fmt.Printf("applied insight %s (%s)\n", bold(stored.Slug), stored.Type)
	}
	return nil
}

// insightCmd groups the imperative helpers around saved insights.
func insightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "insight",
		Short:   "List, inspect and manage saved insights (config-as-code)",
		Aliases: []string{"insights"},
	}

	var lsProject, lsOutput string
	ls := &cobra.Command{
		Use:   "ls",
		Short: "List the active project's managed insights",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listInsights(cmd.Context(), lsProject, lsOutput)
		},
	}
	ls.Flags().StringVar(&lsProject, "project", "", "Project id (defaults to the active project)")
	ls.Flags().StringVarP(&lsOutput, "output", "o", "table", "Output: table, yaml or json")
	cmd.AddCommand(ls)

	var getProject, getOutput string
	get := &cobra.Command{
		Use:   "get <slug>",
		Short: "Show one insight by slug",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return getInsight(cmd.Context(), getProject, args[0], getOutput)
		},
	}
	get.Flags().StringVar(&getProject, "project", "", "Project id (defaults to the active project)")
	get.Flags().StringVarP(&getOutput, "output", "o", "yaml", "Output: yaml or json")
	cmd.AddCommand(get)

	var rmProject string
	rm := &cobra.Command{
		Use:     "rm <slug>",
		Short:   "Delete an insight by slug",
		Aliases: []string{"delete"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeInsight(cmd.Context(), rmProject, args[0])
		},
	}
	rm.Flags().StringVar(&rmProject, "project", "", "Project id (defaults to the active project)")
	cmd.AddCommand(rm)

	var exportProject, exportOutput string
	export := &cobra.Command{
		Use:   "export",
		Short: "Dump all managed insights as an apply-able spec",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return exportInsights(cmd.Context(), exportProject, exportOutput)
		},
	}
	export.Flags().StringVar(&exportProject, "project", "", "Project id (defaults to the active project)")
	export.Flags().StringVarP(&exportOutput, "output", "o", "yaml", "Output: yaml or json")
	cmd.AddCommand(export)

	return cmd
}

func listInsights(ctx context.Context, projectFlag, output string) error {
	client, projectID, err := insightClient(ctx, projectFlag)
	if err != nil {
		return err
	}
	insights, err := client.Insights(ctx, projectID)
	if err != nil {
		return err
	}
	if output == "yaml" || output == "json" {
		return renderDoc(applyDoc{Insights: insights}, output)
	}
	if len(insights) == 0 {
		fmt.Println("No managed insights for this project. Create one with `kd apply -f spec.yaml`.")
		return nil
	}
	tw := newTab()
	fmt.Fprintln(tw, "SLUG\tTYPE\tNAME")
	for _, in := range insights {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", in.Slug, in.Type, in.Name)
	}
	return tw.Flush()
}

func getInsight(ctx context.Context, projectFlag, slug, output string) error {
	client, projectID, err := insightClient(ctx, projectFlag)
	if err != nil {
		return err
	}
	in, err := client.Insight(ctx, projectID, slug)
	if err != nil {
		return err
	}
	return renderValue(in, output)
}

func removeInsight(ctx context.Context, projectFlag, slug string) error {
	client, projectID, err := insightClient(ctx, projectFlag)
	if err != nil {
		return err
	}
	if err := client.DeleteInsight(ctx, projectID, slug); err != nil {
		return err
	}
	fmt.Printf("deleted insight %s\n", bold(slug))
	return nil
}

func exportInsights(ctx context.Context, projectFlag, output string) error {
	client, projectID, err := insightClient(ctx, projectFlag)
	if err != nil {
		return err
	}
	insights, err := client.Insights(ctx, projectID)
	if err != nil {
		return err
	}
	return renderDoc(applyDoc{Insights: insights}, output)
}

// insightClient loads the client and resolves the target project in one step.
func insightClient(ctx context.Context, projectFlag string) (*api.Client, string, error) {
	client, _, err := mustClient()
	if err != nil {
		return nil, "", err
	}
	projectID, err := currentProjectID(ctx, client, projectFlag)
	if err != nil {
		return nil, "", err
	}
	return client, projectID, nil
}

func renderDoc(doc applyDoc, output string) error {
	return renderValue(doc, output)
}

func renderValue(v any, output string) error {
	switch output {
	case "json":
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	case "yaml", "":
		b, err := yaml.Marshal(v)
		if err != nil {
			return err
		}
		fmt.Print(string(b))
	default:
		return fmt.Errorf("--output must be table, yaml or json")
	}
	return nil
}

// readInput reads a spec from a file, or from stdin when the path is empty or
// "-".
func readInput(file string) ([]byte, error) {
	if file == "" || file == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(file)
}
