package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mgm/mgm-cli/internal/dotenv"
	"github.com/mgm/mgm-cli/internal/ui"
)

func newPushCommand() *cobra.Command {
	var (
		sel         selectionFlags
		file        string
		yes         bool
		dryRun      bool
		deleteOrph  bool
		onlyChanged bool
	)
	c := &cobra.Command{
		Use:   "push",
		Short: "Push a local .env file to Infisical",
		Example: "  mgm env push\n" +
			"  mgm env push --env prod --yes\n" +
			"  mgm env push --env stg --file .env.staging\n" +
			"  mgm env push --env prod --delete-orphans --yes",
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

			if file == "" {
				file = ".env"
			}
			localEntries, err := dotenv.Read(file)
			if err != nil {
				return err
			}
			if len(localEntries) == 0 {
				return fmt.Errorf("no entries found in %s", file)
			}
			local := dotenv.ToMap(localEntries)

			remoteSecrets, err := rt.Client.ListSecrets(ctx, projectID, environment, folder)
			if err != nil {
				return err
			}
			remote := make(map[string]string, len(remoteSecrets))
			for _, s := range remoteSecrets {
				remote[s.SecretKey] = s.SecretValue
			}

			diff := dotenv.Compare(local, remote)
			if diff.Empty() {
				ui.Infof("Nothing to push — %s already matches %s/%s%s", file, projectID, environment, folder)
				return nil
			}

			ui.Title(fmt.Sprintf("Push plan: %s -> %s · %s · %s", file, projectID, environment, folder))
			ui.Infof("%s", diff.Render())

			if dryRun {
				return nil
			}

			if !yes {
				if !ui.IsInteractive() {
					return fmt.Errorf("refusing to push without --yes in non-interactive mode")
				}
				ok, err := ui.Confirm(fmt.Sprintf("Apply %d additions, %d updates, %d deletions?",
					len(diff.Added), len(diff.Changed), len(diff.Removed)), false)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("aborted")
				}
			}

			for _, e := range diff.Added {
				if err := rt.Client.CreateSecret(ctx, projectID, environment, folder, e.Key, e.Value, ""); err != nil {
					return fmt.Errorf("create %s: %w", e.Key, err)
				}
			}
			for _, c := range diff.Changed {
				if onlyChanged && c.Local == c.Other {
					continue
				}
				if err := rt.Client.UpdateSecret(ctx, projectID, environment, folder, c.Key, c.Local); err != nil {
					return fmt.Errorf("update %s: %w", c.Key, err)
				}
			}
			if deleteOrph {
				for _, k := range diff.Removed {
					if err := rt.Client.DeleteSecret(ctx, projectID, environment, folder, k); err != nil {
						return fmt.Errorf("delete %s: %w", k, err)
					}
				}
			}

			ui.Successf("Pushed: +%d ~%d -%d", len(diff.Added), len(diff.Changed), boolN(deleteOrph, len(diff.Removed)))
			if !deleteOrph && len(diff.Removed) > 0 {
				ui.Warnf("%d remote secrets not present locally were left untouched (pass --delete-orphans to remove)", len(diff.Removed))
			}
			return nil
		},
	}
	addSelectionFlags(c, &sel)
	c.Flags().StringVarP(&file, "file", "f", "", "Source file (default .env)")
	c.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "Show the change set without applying")
	c.Flags().BoolVar(&deleteOrph, "delete-orphans", false, "Delete remote secrets that don't exist locally")
	c.Flags().BoolVar(&onlyChanged, "only-changed", false, "Skip updates whose value didn't actually change")
	return c
}

func boolN(b bool, n int) int {
	if b {
		return n
	}
	return 0
}
