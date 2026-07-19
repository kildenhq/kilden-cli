package cli

import (
	"fmt"

	"github.com/kildenhq/kilden-cli/internal/config"
	"github.com/spf13/cobra"
)

func logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Forget the stored credentials on this machine",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := config.Clear(); err != nil {
				return err
			}
			fmt.Println("Signed out. Your credentials were removed from this machine.")
			return nil
		},
	}
}
