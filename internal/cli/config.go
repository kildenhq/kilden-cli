package cli

import (
	"context"
	"fmt"

	"github.com/kildenhq/kilden-cli/internal/api"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// applyDoc is the config-as-code spec `kd apply` reads (docs/55 §3.5). One key
// per collection, not a `kind:` per item, so a spec can grow without breaking
// the ones already written.
type applyDoc struct {
	Cohorts     []api.Cohort     `json:"cohorts,omitempty" yaml:"cohorts,omitempty"`
	Flags       []api.Flag       `json:"flags,omitempty" yaml:"flags,omitempty"`
	Insights    []api.Insight    `json:"insights,omitempty" yaml:"insights,omitempty"`
	Units       []api.Unit       `json:"units,omitempty" yaml:"units,omitempty"`
	Campaigns   []api.Campaign   `json:"campaigns,omitempty" yaml:"campaigns,omitempty"`
	Experiments []api.Experiment `json:"experiments,omitempty" yaml:"experiments,omitempty"`
}

// resource is one collection of the apply document: how to name it, how to
// pull its identities out for duplicate checking, and how to push it.
type resource struct {
	name string
	// count reports how many items the document carries.
	count func(applyDoc) int
	// ids returns each item's stable identity, in document order.
	ids func(applyDoc) []string
	// label names the identity field in errors ("slug" or "key").
	label string
	// apply pushes item i and reports what to print.
	apply func(context.Context, *api.Client, string, applyDoc, int) (string, error)
}

// resources is the APPLY ORDER, and it is a dependency order rather than an
// alphabetical one (docs/59 §6): a campaign can name a cohort, and an
// experiment names a flag, so those have to exist first. Declared in exactly
// one place so the order cannot drift between commands.
var resources = []resource{
	{
		name: "cohort", label: "slug",
		count: func(d applyDoc) int { return len(d.Cohorts) },
		ids:   func(d applyDoc) []string { return mapSlice(d.Cohorts, func(c api.Cohort) string { return c.Slug }) },
		apply: func(ctx context.Context, c *api.Client, p string, d applyDoc, i int) (string, error) {
			out, err := c.ApplyCohort(ctx, p, d.Cohorts[i])
			if err != nil {
				return "", err
			}
			return out.Slug, nil
		},
	},
	{
		name: "flag", label: "key",
		count: func(d applyDoc) int { return len(d.Flags) },
		ids:   func(d applyDoc) []string { return mapSlice(d.Flags, func(f api.Flag) string { return f.Key }) },
		apply: func(ctx context.Context, c *api.Client, p string, d applyDoc, i int) (string, error) {
			out, err := c.ApplyFlag(ctx, p, d.Flags[i])
			if err != nil {
				return "", err
			}
			return out.Key, nil
		},
	},
	{
		name: "insight", label: "slug",
		count: func(d applyDoc) int { return len(d.Insights) },
		ids:   func(d applyDoc) []string { return mapSlice(d.Insights, func(i api.Insight) string { return i.Slug }) },
		apply: func(ctx context.Context, c *api.Client, p string, d applyDoc, i int) (string, error) {
			out, err := c.ApplyInsight(ctx, p, d.Insights[i].Slug, d.Insights[i])
			if err != nil {
				return "", err
			}
			return out.Slug, nil
		},
	},
	{
		name: "unit", label: "slug",
		count: func(d applyDoc) int { return len(d.Units) },
		ids:   func(d applyDoc) []string { return mapSlice(d.Units, func(u api.Unit) string { return u.Slug }) },
		apply: func(ctx context.Context, c *api.Client, p string, d applyDoc, i int) (string, error) {
			out, err := c.ApplyUnit(ctx, p, d.Units[i])
			if err != nil {
				return "", err
			}
			return out.Slug, nil
		},
	},
	{
		name: "campaign", label: "slug",
		count: func(d applyDoc) int { return len(d.Campaigns) },
		ids:   func(d applyDoc) []string { return mapSlice(d.Campaigns, func(c api.Campaign) string { return c.Slug }) },
		apply: func(ctx context.Context, c *api.Client, p string, d applyDoc, i int) (string, error) {
			out, err := c.ApplyCampaign(ctx, p, d.Campaigns[i])
			if err != nil {
				return "", err
			}
			return out.Slug, nil
		},
	},
	{
		name: "experiment", label: "key",
		count: func(d applyDoc) int { return len(d.Experiments) },
		ids: func(d applyDoc) []string {
			return mapSlice(d.Experiments, func(e api.Experiment) string { return e.Key })
		},
		apply: func(ctx context.Context, c *api.Client, p string, d applyDoc, i int) (string, error) {
			out, err := c.ApplyExperiment(ctx, p, d.Experiments[i])
			if err != nil {
				return "", err
			}
			return out.Key, nil
		},
	},
}

func mapSlice[T any](in []T, f func(T) string) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

// validateDoc refuses a spec that cannot mean one thing: an item with no
// identity, or two items claiming the same one. Both would make an apply
// non-idempotent, which is the only property the format exists for.
func validateDoc(doc applyDoc) error {
	total := 0
	for _, r := range resources {
		total += r.count(doc)
		seen := map[string]int{}
		for i, id := range r.ids(doc) {
			if id == "" {
				return fmt.Errorf("%ss[%d]: every %s needs a %s (its stable identity)", r.name, i, r.name, r.label)
			}
			if first, dup := seen[id]; dup {
				return fmt.Errorf("duplicate %s %q in %ss[%d] and %ss[%d] — each %s must be unique",
					r.label, id, r.name, first, r.name, i, r.label)
			}
			seen[id] = i
		}
	}
	if total == 0 {
		return fmt.Errorf("the spec has nothing to apply — expected one of: %s", resourceNames())
	}
	return nil
}

func resourceNames() string {
	names := make([]string, len(resources))
	for i, r := range resources {
		names[i] = r.name + "s"
	}
	return joinComma(names)
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// configResourceCmd builds the ls/get/rm/export quartet every config resource
// shares, so adding one is a table entry and not a copy of four commands.
type configResourceCmd struct {
	use, aliasPlural, short, idName string
	list                            func(context.Context, *api.Client, string) (applyDoc, error)
	get                             func(context.Context, *api.Client, string, string) (any, error)
	remove                          func(context.Context, *api.Client, string, string) error
	// table renders the list view; nil falls back to the yaml document.
	table func(applyDoc) error
}

func (r configResourceCmd) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     r.use,
		Short:   r.short,
		Aliases: []string{r.aliasPlural},
	}

	var lsProject, lsOutput string
	ls := &cobra.Command{
		Use:   "ls",
		Short: "List the active project's managed " + r.aliasPlural,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, projectID, err := insightClient(cmd.Context(), lsProject)
			if err != nil {
				return err
			}
			doc, err := r.list(cmd.Context(), client, projectID)
			if err != nil {
				return err
			}
			if lsOutput == "yaml" || lsOutput == "json" || r.table == nil {
				return renderDoc(doc, lsOutput)
			}
			return r.table(doc)
		},
	}
	ls.Flags().StringVar(&lsProject, "project", "", "Project id (defaults to the active project)")
	ls.Flags().StringVarP(&lsOutput, "output", "o", "table", "Output: table, yaml or json")
	cmd.AddCommand(ls)

	var getProject, getOutput string
	get := &cobra.Command{
		Use:   "get <" + r.idName + ">",
		Short: "Show one " + r.use + " by " + r.idName,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, projectID, err := insightClient(cmd.Context(), getProject)
			if err != nil {
				return err
			}
			out, err := r.get(cmd.Context(), client, projectID, args[0])
			if err != nil {
				return err
			}
			return renderValue(out, getOutput)
		},
	}
	get.Flags().StringVar(&getProject, "project", "", "Project id (defaults to the active project)")
	get.Flags().StringVarP(&getOutput, "output", "o", "yaml", "Output: yaml or json")
	cmd.AddCommand(get)

	var rmProject string
	rm := &cobra.Command{
		Use:     "rm <" + r.idName + ">",
		Short:   "Delete a " + r.use + " by " + r.idName,
		Aliases: []string{"delete"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, projectID, err := insightClient(cmd.Context(), rmProject)
			if err != nil {
				return err
			}
			if err := r.remove(cmd.Context(), client, projectID, args[0]); err != nil {
				return err
			}
			fmt.Printf("deleted %s %s\n", r.use, bold(args[0]))
			return nil
		},
	}
	rm.Flags().StringVar(&rmProject, "project", "", "Project id (defaults to the active project)")
	cmd.AddCommand(rm)

	var exportProject, exportOutput string
	export := &cobra.Command{
		Use:   "export",
		Short: "Dump all managed " + r.aliasPlural + " as an apply-able spec",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, projectID, err := insightClient(cmd.Context(), exportProject)
			if err != nil {
				return err
			}
			doc, err := r.list(cmd.Context(), client, projectID)
			if err != nil {
				return err
			}
			return renderDoc(doc, exportOutput)
		},
	}
	export.Flags().StringVar(&exportProject, "project", "", "Project id (defaults to the active project)")
	export.Flags().StringVarP(&exportOutput, "output", "o", "yaml", "Output: yaml or json")
	cmd.AddCommand(export)

	return cmd
}

func cohortCmd() *cobra.Command {
	cmd := configResourceCmd{
		use: "cohort", aliasPlural: "cohorts", idName: "slug",
		short: "List, inspect and manage cohorts (config-as-code)",
		list: func(ctx context.Context, c *api.Client, p string) (applyDoc, error) {
			out, err := c.Cohorts(ctx, p)
			return applyDoc{Cohorts: out}, err
		},
		get: func(ctx context.Context, c *api.Client, p, id string) (any, error) {
			return c.Cohort(ctx, p, id)
		},
		remove: func(ctx context.Context, c *api.Client, p, id string) error {
			return c.DeleteCohort(ctx, p, id)
		},
		table: func(d applyDoc) error {
			if len(d.Cohorts) == 0 {
				fmt.Println("No managed cohorts for this project. Create one with `kd apply -f spec.yaml`.")
				return nil
			}
			tw := newTab()
			fmt.Fprintln(tw, "SLUG\tNAME\tMEMBERS")
			for _, c := range d.Cohorts {
				fmt.Fprintf(tw, "%s\t%s\t%d\n", c.Slug, c.Name, c.MembersCount)
			}
			return tw.Flush()
		},
	}.command()

	// Materialize is imperative, so it lives here rather than in the document.
	var project string
	materialize := &cobra.Command{
		Use:   "materialize <slug>",
		Short: "Recompute membership now instead of waiting for the sweep",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, projectID, err := insightClient(cmd.Context(), project)
			if err != nil {
				return err
			}
			out, err := client.MaterializeCohort(cmd.Context(), projectID, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("materialized %s: %d members\n", bold(out.Slug), out.MembersCount)
			return nil
		},
	}
	materialize.Flags().StringVar(&project, "project", "", "Project id (defaults to the active project)")
	cmd.AddCommand(materialize)

	return cmd
}

func flagCmd() *cobra.Command {
	return configResourceCmd{
		use: "flag", aliasPlural: "flags", idName: "key",
		short: "List, inspect and manage feature flags (config-as-code)",
		list: func(ctx context.Context, c *api.Client, p string) (applyDoc, error) {
			out, err := c.Flags(ctx, p)
			return applyDoc{Flags: out}, err
		},
		get: func(ctx context.Context, c *api.Client, p, id string) (any, error) {
			return c.Flag(ctx, p, id)
		},
		remove: func(ctx context.Context, c *api.Client, p, id string) error {
			return c.DeleteFlag(ctx, p, id)
		},
		table: func(d applyDoc) error {
			if len(d.Flags) == 0 {
				fmt.Println("No flags in this project yet. Create one with `kd apply -f spec.yaml`.")
				return nil
			}
			tw := newTab()
			fmt.Fprintln(tw, "KEY\tNAME\tACTIVE\tROLLOUT\tVARIANTS")
			for _, f := range d.Flags {
				fmt.Fprintf(tw, "%s\t%s\t%t\t%d%%\t%d\n", f.Key, f.Name, f.Active, f.RolloutPercentage, len(f.Variants))
			}
			return tw.Flush()
		},
	}.command()
}

func unitCmd() *cobra.Command {
	return configResourceCmd{
		use: "unit", aliasPlural: "units", idName: "slug",
		short: "List, inspect and manage in-app units and tours (config-as-code)",
		list: func(ctx context.Context, c *api.Client, p string) (applyDoc, error) {
			out, err := c.Units(ctx, p)
			return applyDoc{Units: out}, err
		},
		get: func(ctx context.Context, c *api.Client, p, id string) (any, error) {
			return c.Unit(ctx, p, id)
		},
		remove: func(ctx context.Context, c *api.Client, p, id string) error {
			return c.DeleteUnit(ctx, p, id)
		},
		table: func(d applyDoc) error {
			if len(d.Units) == 0 {
				fmt.Println("No managed in-app units for this project. Create one with `kd apply -f spec.yaml`.")
				return nil
			}
			tw := newTab()
			fmt.Fprintln(tw, "SLUG\tTYPE\tNAME\tSTATUS")
			for _, u := range d.Units {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", u.Slug, u.Type, u.Name, u.Status)
			}
			return tw.Flush()
		},
	}.command()
}

func campaignCmd() *cobra.Command {
	return configResourceCmd{
		use: "campaign", aliasPlural: "campaigns", idName: "slug",
		short: "List, inspect and manage campaigns (config-as-code)",
		list: func(ctx context.Context, c *api.Client, p string) (applyDoc, error) {
			out, err := c.Campaigns(ctx, p)
			return applyDoc{Campaigns: out}, err
		},
		get: func(ctx context.Context, c *api.Client, p, id string) (any, error) {
			return c.Campaign(ctx, p, id)
		},
		remove: func(ctx context.Context, c *api.Client, p, id string) error {
			return c.DeleteCampaign(ctx, p, id)
		},
		table: func(d applyDoc) error {
			if len(d.Campaigns) == 0 {
				fmt.Println("No managed campaigns for this project. Create one with `kd apply -f spec.yaml`.")
				return nil
			}
			tw := newTab()
			fmt.Fprintln(tw, "SLUG\tNAME\tSTATUS\tSTEPS")
			for _, c := range d.Campaigns {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", c.Slug, c.Name, c.Status, len(c.Nodes))
			}
			return tw.Flush()
		},
	}.command()
}

func experimentCmd() *cobra.Command {
	return configResourceCmd{
		use: "experiment", aliasPlural: "experiments", idName: "key",
		short: "List, inspect and manage experiments (config-as-code)",
		list: func(ctx context.Context, c *api.Client, p string) (applyDoc, error) {
			out, err := c.Experiments(ctx, p)
			return applyDoc{Experiments: out}, err
		},
		get: func(ctx context.Context, c *api.Client, p, id string) (any, error) {
			return c.Experiment(ctx, p, id)
		},
		remove: func(ctx context.Context, c *api.Client, p, id string) error {
			return c.DeleteExperiment(ctx, p, id)
		},
		table: func(d applyDoc) error {
			if len(d.Experiments) == 0 {
				fmt.Println("No experiments in this project yet. Create one with `kd apply -f spec.yaml`.")
				return nil
			}
			tw := newTab()
			fmt.Fprintln(tw, "KEY\tFLAG\tNAME\tSTATUS")
			for _, e := range d.Experiments {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Key, e.Flag, e.Name, e.Status)
			}
			return tw.Flush()
		},
	}.command()
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
		Long: "Apply a YAML or JSON spec of cohorts, feature flags, saved insights,\n" +
			"in-app units, campaigns and experiments. Re-applying the same spec updates\n" +
			"each resource in place by its slug or key and never duplicates. Read from a\n" +
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
	if err := validateDoc(doc); err != nil {
		return err
	}

	client, _, err := mustClient()
	if err != nil {
		return err
	}
	projectID, err := currentProjectID(ctx, client, projectFlag)
	if err != nil {
		return err
	}

	// In dependency order, not document order: a campaign may name a cohort and
	// an experiment names a flag, so those have to land first.
	for _, r := range resources {
		for i := range r.count(doc) {
			id, err := r.apply(ctx, client, projectID, doc, i)
			if err != nil {
				return fmt.Errorf("apply %s %q: %w", r.name, r.ids(doc)[i], err)
			}
			fmt.Printf("applied %s %s\n", r.name, bold(id))
		}
	}
	return nil
}
