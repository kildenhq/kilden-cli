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

// insightCmd is the shared config quartet over saved insights.
func insightCmd() *cobra.Command {
	return configResourceCmd{
		use: "insight", aliasPlural: "insights", idName: "slug",
		short: "List, inspect and manage saved insights (config-as-code)",
		list: func(ctx context.Context, c *api.Client, p string) (applyDoc, error) {
			out, err := c.Insights(ctx, p)
			return applyDoc{Insights: out}, err
		},
		get: func(ctx context.Context, c *api.Client, p, id string) (any, error) {
			return c.Insight(ctx, p, id)
		},
		remove: func(ctx context.Context, c *api.Client, p, id string) error {
			return c.DeleteInsight(ctx, p, id)
		},
		table: func(d applyDoc) error {
			if len(d.Insights) == 0 {
				fmt.Println("No managed insights for this project. Create one with `kd apply -f spec.yaml`.")
				return nil
			}
			tw := newTab()
			fmt.Fprintln(tw, "SLUG\tTYPE\tNAME")
			for _, in := range d.Insights {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", in.Slug, in.Type, in.Name)
			}
			return tw.Flush()
		},
	}.command()
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
