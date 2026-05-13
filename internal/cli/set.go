package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MGM-Laboratory/mgm-cli/internal/ui"
)

func newSetCommand() *cobra.Command {
	var (
		sel     selectionFlags
		comment string
	)
	c := &cobra.Command{
		Use:   "set <KEY=VALUE> [KEY=VALUE...]",
		Short: "Create or update one or more secrets",
		Args:  cobra.MinimumNArgs(1),
		Example: "  mgm env set DATABASE_URL=postgres://...\n" +
			"  mgm env set --env prod API_KEY=xxx STRIPE_KEY=yyy",
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

			pairs := make(map[string]string, len(args))
			for _, a := range args {
				i := strings.IndexByte(a, '=')
				if i <= 0 {
					return fmt.Errorf("invalid pair %q (expected KEY=VALUE)", a)
				}
				pairs[a[:i]] = a[i+1:]
			}

			existing, err := rt.Client.ListSecrets(ctx, projectID, environment, folder)
			if err != nil {
				return err
			}
			have := map[string]struct{}{}
			for _, s := range existing {
				have[s.SecretKey] = struct{}{}
			}

			for k, v := range pairs {
				if _, ok := have[k]; ok {
					if err := rt.Client.UpdateSecret(ctx, projectID, environment, folder, k, v); err != nil {
						return fmt.Errorf("update %s: %w", k, err)
					}
					ui.Successf("updated %s", k)
				} else {
					if err := rt.Client.CreateSecret(ctx, projectID, environment, folder, k, v, comment); err != nil {
						return fmt.Errorf("create %s: %w", k, err)
					}
					ui.Successf("created %s", k)
				}
			}
			return nil
		},
	}
	addSelectionFlags(c, &sel)
	c.Flags().StringVar(&comment, "comment", "", "Comment to attach to newly-created secrets")
	return c
}
