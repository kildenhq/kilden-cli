package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kildenhq/kilden-cli/internal/api"
	"github.com/spf13/cobra"
)

// The `kd init` design, in one place (docs/60 §7, wave 5.4, and §6 item 8).
//
// This command is deliberately THIN: provision a project, hand over the key,
// and wait for the first event. It does not detect your framework and it does
// not write files into your repository — 90% of the cost and all of the
// maintenance of an `init` lives there, and it is exactly the part that can be
// handed to the coding agent the user already brought with them.
//
// What is left is the part only we can do, and it is the part that matters: a
// terminal that goes quiet and then says "your first event just landed" is the
// same beat the wizard is built around, delivered where a developer already
// is.

// How often the wait loop asks. Three seconds keeps a five-minute wait at
// twenty requests a minute, well under any rate limit, and still feels
// immediate to somebody watching. A var so tests can shrink it: the cadence is
// not the behaviour they are checking.
var initPollInterval = 3 * time.Second

// Long enough to install a snippet and reload a page; short enough that an
// abandoned terminal does not sit open all afternoon.
const initDefaultWait = 5 * time.Minute

func initCmd() *cobra.Command {
	var name, timezone string
	var wait time.Duration
	var noWait bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a project, print its write key, and wait for the first event",
		Long: "Creates a project in the current team, makes it active, prints the snippet\n" +
			"to install, and then waits until Kilden receives its first real event.\n\n" +
			"It writes nothing into your repository and guesses nothing about your\n" +
			"stack: paste the snippet yourself, or hand it to a coding agent.\n\n" +
			"A wait that times out is not a failure and exits 0 — people install\n" +
			"tomorrow, and a red exit code would be lying about that.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if noWait {
				wait = 0
			}
			return runInit(cmd.Context(), name, timezone, wait)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name (defaults to this directory's name)")
	cmd.Flags().StringVar(&timezone, "timezone", "UTC", "IANA timezone, e.g. America/Santiago")
	cmd.Flags().DurationVar(&wait, "wait", initDefaultWait, "How long to wait for the first event")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "Create the project and exit without waiting")

	return cmd
}

func runInit(ctx context.Context, name, timezone string, wait time.Duration) error {
	client, _, err := mustClient()
	if err != nil {
		return err
	}
	teamID, err := currentTeamID(ctx, client)
	if err != nil {
		return err
	}

	if name == "" {
		name = defaultProjectName()
	}
	if name == "" {
		return fmt.Errorf("could not name the project from this directory — pass --name")
	}

	project, err := client.CreateProject(ctx, teamID, name, timezone)
	if err != nil {
		return err
	}

	// Make it the active project before anything else. `init` is an entry
	// point, not a one-off: every command the person runs next should be
	// about the project they just made.
	if _, err := client.SwitchProject(ctx, project.ID); err != nil {
		return err
	}

	fmt.Printf("Created %s (%s), and it is now your active project.\n\n", project.Name, project.ID)

	if project.PublicKey == "" {
		return fmt.Errorf("the project was created but came back without a write key — run `kd key create`")
	}

	printSnippet(project.PublicKey)

	if wait <= 0 {
		fmt.Println("Not waiting. Run `kd project ls` later, or watch events with `kd tail`.")
		return nil
	}

	return waitForFirstEvent(ctx, client, project, wait)
}

// printSnippet writes the one thing the person has to move: the browser
// snippet with their key already in it. No file is written and no framework is
// guessed — this is text to paste, or to hand to an agent.
func printSnippet(publicKey string) {
	fmt.Println("Paste this before </head> on your site:")
	fmt.Println()
	fmt.Printf("  <script src=\"https://cdn.kilden.io/kilden.js\"></script>\n")
	fmt.Printf("  <script>kilden.init('%s')</script>\n", publicKey)
	fmt.Println()
}

// waitForFirstEvent polls until a real event lands, the deadline passes, or the
// user interrupts.
//
// The API's `activated` excludes the panel's own test button, so this cannot
// congratulate somebody for a button we pressed on their behalf — which would
// be a lie told at the exact moment they are deciding whether to trust us.
func waitForFirstEvent(ctx context.Context, client *api.Client, project *api.Project, wait time.Duration) error {
	fmt.Printf("Waiting up to %s for the first event. Ctrl-C to stop; nothing is lost if you do.\n", wait)

	deadline := time.Now().Add(wait)
	ticker := time.NewTicker(initPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// An interrupted wait is not a failed install. Say so, and say
			// what still works.
			fmt.Println("\nStopped waiting. The project and its key are already set up.")
			return nil
		case <-ticker.C:
			current, err := client.Project(ctx, project.ID)
			if err != nil {
				// A hiccup mid-wait must not end the wait: the install may
				// be seconds away and this is a poll, not a transaction.
				continue
			}
			if current.Activated {
				fmt.Printf("\nYour first event just landed. %s is live.\n", project.Name)
				fmt.Println("Watch them arrive with `kd tail`.")
				return nil
			}
			if time.Now().After(deadline) {
				printWaitTimeout(project.Name)
				return nil
			}
		}
	}
}

// printWaitTimeout says what to check, in the order it is worth checking.
// A timeout is not an error: people install tomorrow.
func printWaitTimeout(name string) {
	fmt.Println("\nNo events yet — that is normal if the snippet is not deployed.")
	fmt.Println("When it is, `kd tail` shows them arriving live.")
	fmt.Printf("Nothing to redo: %s and its key are already set up.\n", name)
}

// defaultProjectName names the project after the directory the person is
// standing in, which is almost always the name they would have typed.
func defaultProjectName() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	base := strings.TrimSpace(filepath.Base(cwd))
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return base
}
