package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/coder/websocket"
	"github.com/kildenhq/kilden-cli/internal/api"
	"github.com/spf13/cobra"
)

func tailCmd() *cobra.Command {
	var project, event string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Stream events live as they arrive (Ctrl+C to stop)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tailEvents(cmd.Context(), project, event, jsonOut)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project id (defaults to the active project)")
	cmd.Flags().StringVar(&event, "event", "", "Only show events with this name")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print each event as raw JSON")
	return cmd
}

func tailEvents(ctx context.Context, projectFlag, event string, jsonOut bool) error {
	client, _, err := mustClient()
	if err != nil {
		return err
	}
	projectID, err := currentProjectID(ctx, client, projectFlag)
	if err != nil {
		return err
	}

	// Status goes to stderr so `kd tail --json | jq` stays clean.
	fmt.Fprintln(os.Stderr, "Streaming live events — Ctrl+C to stop.")

	attempt := 0
	for ctx.Err() == nil {
		err := streamTail(ctx, client, projectID, event, jsonOut)
		if ctx.Err() != nil {
			return nil // Ctrl+C — a clean stop, not an error.
		}

		// The server closes the socket normally about every 10 minutes so the
		// client refreshes its ticket; reconnect at once. Anything else backs
		// off exponentially (network blip, panel restart).
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
			attempt = 0
		} else {
			attempt++
			fmt.Fprintf(os.Stderr, "reconnecting (%v)…\n", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff(attempt)):
		}
	}
	return nil
}

// streamTail fetches a fresh ticket, dials the websocket and prints frames
// until the connection ends. It returns the error that ended it so the caller
// can decide whether to reconnect.
func streamTail(ctx context.Context, client *api.Client, projectID, event string, jsonOut bool) error {
	ticket, err := client.LivetailTicket(ctx, projectID)
	if err != nil {
		return err
	}

	conn, _, err := websocket.Dial(ctx, ticket.URL+"?ticket="+url.QueryEscape(ticket.Ticket), nil)
	if err != nil {
		return err
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		printTailEvent(data, event, jsonOut)
	}
}

func printTailEvent(data []byte, filter string, jsonOut bool) {
	var e api.LiveEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return // Skip a malformed frame rather than kill the stream.
	}
	if filter != "" && e.Event != filter {
		return
	}
	if jsonOut {
		fmt.Println(string(data))
		return
	}

	ts := e.Timestamp
	if t, err := time.Parse(time.RFC3339Nano, e.Timestamp); err == nil {
		ts = t.Local().Format("15:04:05")
	}
	who := e.DistinctID
	if !e.Verified {
		who += " (unverified)"
	}
	fmt.Printf("%s  %s  %s\n", ts, bold(e.Event), who)
}

// backoff returns 0 for the first attempt, then 1s, 2s, 4s… capped at 15s.
func backoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	// Clamp the exponent before shifting: 1<<4 = 16s already exceeds the cap,
	// and an unbounded attempt count would overflow the shift.
	if attempt > 5 {
		attempt = 5
	}
	d := time.Duration(1<<uint(attempt-1)) * time.Second
	if d > 15*time.Second {
		d = 15 * time.Second
	}
	return d
}
