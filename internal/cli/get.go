package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mgm/mgm-cli/internal/ui"
)

func newGetCommand() *cobra.Command {
	var (
		sel  selectionFlags
		raw  bool
	)
	c := &cobra.Command{
		Use:   "get <KEY>",
		Short: "Print the value of a single secret",
		Args:  cobra.ExactArgs(1),
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
			s, err := rt.Client.GetSecret(ctx, projectID, environment, folder, args[0])
			if err != nil {
				return err
			}
			if raw {
				fmt.Fprintln(ui.Out, s.SecretValue)
			} else {
				ui.KV(s.SecretKey, s.SecretValue)
			}
			return nil
		},
	}
	addSelectionFlags(c, &sel)
	c.Flags().BoolVar(&raw, "raw", false, "Print only the value (no key prefix)")
	return c
}
