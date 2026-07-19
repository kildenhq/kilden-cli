package cli

import (
	"context"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func eventsCmd() *cobra.Command {
	var project, event string
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Show recent events for the active project (newest first)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listEvents(cmd.Context(), project, event)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project id (defaults to the active project)")
	cmd.Flags().StringVar(&event, "event", "", "Filter by event name")
	return cmd
}

func listEvents(ctx context.Context, projectFlag, event string) error {
	client, _, err := mustClient()
	if err != nil {
		return err
	}
	projectID, err := currentProjectID(ctx, client, projectFlag)
	if err != nil {
		return err
	}

	filters := url.Values{}
	if event != "" {
		filters.Set("event", event)
	}
	page, err := client.Events(ctx, projectID, filters)
	if err != nil {
		return err
	}
	if len(page.Events) == 0 {
		fmt.Println("No events yet for this project.")
		return nil
	}

	tw := newTab()
	fmt.Fprintln(tw, "TIMESTAMP\tEVENT\tDISTINCT ID")
	for _, e := range page.Events {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Timestamp, e.Event, e.DistinctID)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if page.HasMore {
		fmt.Println("… more events available (showing the most recent page).")
	}
	return nil
}

func propertiesCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "properties",
		Short: "Show the event names and property keys seen for the active project",
		Long: "Kilden properties are schema-less: there is nothing to create. This lists\n" +
			"the event names and property keys actually observed in recent events.",
		Aliases: []string{"props"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listCatalog(cmd.Context(), project)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project id (defaults to the active project)")
	return cmd
}

func listCatalog(ctx context.Context, projectFlag string) error {
	client, _, err := mustClient()
	if err != nil {
		return err
	}
	projectID, err := currentProjectID(ctx, client, projectFlag)
	if err != nil {
		return err
	}
	cat, err := client.Catalog(ctx, projectID)
	if err != nil {
		return err
	}

	fmt.Println(bold("Events"))
	if len(cat.Events) == 0 {
		fmt.Println("  (none observed)")
	}
	tw := newTab()
	for _, e := range cat.Events {
		fmt.Fprintf(tw, "  %s\t%d\n", e.Name, e.Count)
	}
	_ = tw.Flush()

	fmt.Println(bold("\nProperties"))
	if len(cat.Properties) == 0 {
		fmt.Println("  (none observed)")
	}
	tw = newTab()
	for _, p := range cat.Properties {
		fmt.Fprintf(tw, "  %s\t%d\n", p.Key, p.Count)
	}
	return tw.Flush()
}
