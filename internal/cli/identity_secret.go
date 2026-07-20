package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func identitySecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "identity-secret",
		Short:   "Manage the project's identity-verification secrets",
		Aliases: []string{"identity-secrets", "is"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listIdentitySecrets(cmd.Context(), "")
		},
	}

	var project string
	ls := &cobra.Command{
		Use:   "ls",
		Short: "List the active project's identity secrets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listIdentitySecrets(cmd.Context(), project)
		},
	}
	ls.Flags().StringVar(&project, "project", "", "Project id (defaults to the active project)")
	cmd.AddCommand(ls)

	var createProject, kid string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a secret under a kid (its value is shown only once)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return createIdentitySecret(cmd.Context(), createProject, kid)
		},
	}
	create.Flags().StringVar(&createProject, "project", "", "Project id (defaults to the active project)")
	create.Flags().StringVar(&kid, "kid", "", "Key id used to rotate secrets (auto-generated if omitted)")
	cmd.AddCommand(create)

	var disableProject string
	disable := &cobra.Command{
		Use:   "disable <id>",
		Short: "Revoke a secret by id (from `identity-secret ls`)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return disableIdentitySecret(cmd.Context(), disableProject, args[0])
		},
	}
	disable.Flags().StringVar(&disableProject, "project", "", "Project id (defaults to the active project)")
	cmd.AddCommand(disable)

	return cmd
}

func listIdentitySecrets(ctx context.Context, projectFlag string) error {
	client, _, err := mustClient()
	if err != nil {
		return err
	}
	projectID, err := currentProjectID(ctx, client, projectFlag)
	if err != nil {
		return err
	}
	secrets, err := client.IdentitySecrets(ctx, projectID)
	if err != nil {
		return err
	}
	if len(secrets) == 0 {
		fmt.Println("No identity secrets for this project.")
		return nil
	}

	tw := newTab()
	fmt.Fprintln(tw, "ID\tKID\tSTATUS\tCREATED")
	for _, s := range secrets {
		status := "active"
		if s.RevokedAt != "" {
			status = "revoked"
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", s.ID, s.Kid, status, s.CreatedAt)
	}
	return tw.Flush()
}

func createIdentitySecret(ctx context.Context, projectFlag, kid string) error {
	if kid == "" {
		kid = generateKid()
	}

	client, _, err := mustClient()
	if err != nil {
		return err
	}
	projectID, err := currentProjectID(ctx, client, projectFlag)
	if err != nil {
		return err
	}

	secret, err := client.CreateIdentitySecret(ctx, projectID, kid)
	if err != nil {
		return err
	}

	fmt.Printf("Created identity secret (kid %s): %s\n", secret.Kid, bold(secret.Secret))
	fmt.Println("Store it now — the secret is shown only this once.")
	return nil
}

func disableIdentitySecret(ctx context.Context, projectFlag, idArg string) error {
	id, err := strconv.Atoi(idArg)
	if err != nil {
		return fmt.Errorf("id must be a number from `kd identity-secret ls`, got %q", idArg)
	}

	client, _, err := mustClient()
	if err != nil {
		return err
	}
	projectID, err := currentProjectID(ctx, client, projectFlag)
	if err != nil {
		return err
	}

	secret, err := client.DisableIdentitySecret(ctx, projectID, id)
	if err != nil {
		return err
	}

	fmt.Printf("Disabled identity secret (kid %s).\n", secret.Kid)
	return nil
}

// generateKid builds a unique, spec-valid kid when the user doesn't supply one.
func generateKid() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand never fails in practice; keep a stable, valid fallback.
		return "cli-key"
	}
	return "cli-" + hex.EncodeToString(b[:])
}
