package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mgm/mgm-cli/internal/dotenv"
	"github.com/mgm/mgm-cli/internal/ui"
)

func newPullCommand() *cobra.Command {
	var (
		sel    selectionFlags
		file   string
		sorted bool
		force  bool
		stdout bool
	)
	c := &cobra.Command{
		Use:   "pull",
		Short: "Pull secrets from Infisical to a local .env file",
		Example: "  mgm env pull\n" +
			"  mgm env pull --env prod\n" +
			"  mgm env pull --env stg --file .env.staging\n" +
			"  mgm env pull --project my-app --env prod --folder /backend",
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

			secrets, err := rt.Client.ListSecrets(ctx, projectID, environment, folder)
			if err != nil {
				return err
			}

			entries := make([]dotenv.Entry, 0, len(secrets))
			for _, s := range secrets {
				entries = append(entries, dotenv.Entry{Key: s.SecretKey, Value: s.SecretValue})
			}

			if stdout {
				return dotenvToWriter(entries, sorted, ui.Out)
			}

			if file == "" {
				file = ".env"
			}

			if _, statErr := os.Stat(file); statErr == nil && !force {
				if ui.IsInteractive() {
					ok, err := ui.Confirm(fmt.Sprintf("%s exists. Overwrite?", file), false)
					if err != nil {
						return err
					}
					if !ok {
						return fmt.Errorf("aborted")
					}
				} else {
					return fmt.Errorf("%s exists; pass --force to overwrite", file)
				}
			}

			if err := dotenv.Write(file, entries, sorted); err != nil {
				return err
			}
			ui.Successf("Wrote %d secrets to %s (project=%s env=%s folder=%s)", len(entries), file, projectID, environment, folder)
			return nil
		},
	}
	addSelectionFlags(c, &sel)
	c.Flags().StringVarP(&file, "file", "f", "", "Output file (default .env)")
	c.Flags().BoolVar(&sorted, "sort", false, "Alphabetise keys in output")
	c.Flags().BoolVar(&force, "force", false, "Overwrite the output file without confirmation")
	c.Flags().BoolVar(&stdout, "stdout", false, "Write to stdout instead of a file")
	return c
}

func dotenvToWriter(entries []dotenv.Entry, sorted bool, w interface{ Write([]byte) (int, error) }) error {
	tmp, err := os.CreateTemp("", "mgm-env-*.env")
	if err != nil {
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := dotenv.Write(tmp.Name(), entries, sorted); err != nil {
		return err
	}
	b, err := os.ReadFile(tmp.Name())
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}
