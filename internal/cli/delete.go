package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mgm/mgm-cli/internal/ui"
)

func newDeleteCommand() *cobra.Command {
	var (
		sel selectionFlags
		yes bool
	)
	c := &cobra.Command{
		Use:     "delete <KEY> [KEY...]",
		Aliases: []string{"rm", "del"},
		Short:   "Delete one or more secrets",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			rt, err := resolveRuntime(ctx, globalProfile)
			if err != nil {
				return err
			}
			projectID, environment, folder, err := rt.resolveSelection(ctx, sel)
			if err != nil {
				return err
			}

			if !yes {
				if !ui.IsInteractive() {
					return fmt.Errorf("refusing to delete without --yes in non-interactive mode")
				}
				ok, err := ui.Confirm(fmt.Sprintf("Delete %d secret(s) from %s/%s%s?", len(args), projectID, environment, folder), false)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("aborted")
				}
			}

			for _, k := range args {
				if err := rt.Client.DeleteSecret(ctx, projectID, environment, folder, k); err != nil {
					return fmt.Errorf("delete %s: %w", k, err)
				}
				ui.Successf("deleted %s", k)
			}
			return nil
		},
	}
	addSelectionFlags(c, &sel)
	c.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	return c
}
