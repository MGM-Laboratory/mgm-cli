package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/MGM-Laboratory/mgm-cli/internal/dotenv"
	"github.com/MGM-Laboratory/mgm-cli/internal/ui"
)

func newDiffCommand() *cobra.Command {
	var (
		sel  selectionFlags
		file string
	)
	c := &cobra.Command{
		Use:   "diff",
		Short: "Show differences between a local .env file and Infisical",
		Long: "Compares the local .env (or --file) against the secrets in Infisical for the selected project/env/folder. " +
			"Exits with code 1 if there are differences (handy for CI).",
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
			ui.Title(fmt.Sprintf("%s vs %s · %s · %s", file, projectID, environment, folder))
			fmt.Fprint(ui.Out, diff.Render())
			fmt.Fprintln(ui.Out)
			if !diff.Empty() {
				os.Exit(1)
			}
			return nil
		},
	}
	addSelectionFlags(c, &sel)
	c.Flags().StringVarP(&file, "file", "f", "", "Local file to compare against (default .env)")
	return c
}
