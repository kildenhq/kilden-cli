package cli

import (
	"context"
	"fmt"

	"github.com/kildenhq/kilden-cli/internal/api"
	"github.com/kildenhq/kilden-cli/internal/auth"
	"github.com/spf13/cobra"
)

// The CLI asks for the full scope set up front, so a single sign-in can read
// and manage account/data. The panel still enforces team/project authorization
// per request regardless of the token's scopes.
const loginScopes = "account:read account:write data:read data:write"

func loginCmd() *cobra.Command {
	var host string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in to Kilden using the OAuth device flow",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLogin(cmd.Context(), host)
		},
	}
	cmd.Flags().StringVar(&host, "host", "", "Panel host (default https://app.kilden.io, or $KILDEN_HOST)")
	return cmd
}

func runLogin(ctx context.Context, hostFlag string) error {
	host := resolveHost(hostFlag)
	clientID := resolveClientID()

	dc, err := auth.RequestDeviceCode(ctx, host, clientID, loginScopes)
	if err != nil {
		return err
	}

	verify := dc.VerificationURIComplete
	if verify == "" {
		verify = dc.VerificationURI
	}

	fmt.Printf("\n  First, copy your one-time code:\n\n      %s\n\n", bold(dc.UserCode))
	fmt.Printf("  Then open this URL and confirm the code matches:\n\n      %s\n\n", dc.VerificationURI)
	_ = openBrowser(verify)
	fmt.Println("  Waiting for you to approve in the browser …")

	creds, err := auth.PollForToken(ctx, host, clientID, dc)
	if err != nil {
		return err
	}
	if err := creds.Save(); err != nil {
		return err
	}

	me, err := api.New(creds).Whoami(ctx)
	if err != nil {
		// The token is saved; a whoami hiccup should not read as a login
		// failure. Report it softly.
		fmt.Printf("\n  ✓ Signed in to %s (could not load your profile: %v)\n", host, err)
		return nil
	}

	fmt.Printf("\n  ✓ Signed in as %s\n", bold(me.Email))
	switch {
	case me.CurrentProject != nil:
		fmt.Printf("    Active project: %s\n", me.CurrentProject.Name)
	default:
		fmt.Println("    No active project yet — run `kd project use` to pick one.")
	}
	return nil
}
