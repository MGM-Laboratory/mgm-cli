package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/MGM-Laboratory/mgm-cli/internal/ui"
)

func newListCommand() *cobra.Command {
	var (
		sel       selectionFlags
		jsonOut   bool
		showVals  bool
	)
	c := &cobra.Command{
		Use:   "list",
		Short: "List projects, folders, and secret keys in Infisical",
		Long: "Without flags, opens an interactive picker to choose a project, environment, and folder, " +
			"and prints the secret keys at that path. With --project/--env/--folder, prints directly.",
		Example: "  mgm env list\n  mgm env list --project my-app --env prod\n  mgm env list --project my-app --env prod --folder /backend --show-values",
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

			folders, err := rt.Client.ListFolders(ctx, projectID, environment, folder)
			if err != nil {
				return err
			}
			secrets, err := rt.Client.ListSecrets(ctx, projectID, environment, folder)
			if err != nil {
				return err
			}

			if jsonOut {
				out := map[string]any{
					"projectId":   projectID,
					"environment": environment,
					"folder":      folder,
					"folders":     folders,
					"secrets":     secrets,
				}
				enc := json.NewEncoder(ui.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			ui.Title(fmt.Sprintf("%s · %s · %s", projectID, environment, folder))

			if len(folders) > 0 {
				ui.Infof("\n%s", ui.Key("Folders"))
				sort.Slice(folders, func(i, j int) bool { return folders[i].Name < folders[j].Name })
				for _, f := range folders {
					fmt.Fprintf(ui.Out, "  %s/\n", f.Name)
					_ = path.Join(folder, f.Name)
				}
			}

			ui.Infof("\n%s", ui.Key("Secrets"))
			if len(secrets) == 0 {
				ui.Infof("  %s", ui.Dim("(none)"))
				return nil
			}
			sort.Slice(secrets, func(i, j int) bool { return secrets[i].SecretKey < secrets[j].SecretKey })
			tw := tabwriter.NewWriter(ui.Out, 0, 0, 2, ' ', 0)
			for _, s := range secrets {
				if showVals {
					fmt.Fprintf(tw, "  %s\t%s\n", s.SecretKey, s.SecretValue)
				} else {
					fmt.Fprintf(tw, "  %s\t%s\n", s.SecretKey, ui.Dim("(hidden — use --show-values)"))
				}
			}
			return tw.Flush()
		},
	}
	addSelectionFlags(c, &sel)
	c.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON instead of human output")
	c.Flags().BoolVar(&showVals, "show-values", false, "Reveal secret values")
	return c
}

// addSelectionFlags adds the standard --project/--env/--folder set used by
// every command that needs a selection.
func addSelectionFlags(c *cobra.Command, sel *selectionFlags) {
	c.Flags().StringVarP(&sel.Project, "project", "p", "", "Project ID or slug (default: from .mgm.yaml or profile)")
	c.Flags().StringVarP(&sel.Environment, "env", "e", "", "Environment slug, e.g. dev, stg, prod")
	c.Flags().StringVarP(&sel.Folder, "folder", "F", "", "Secret folder path, e.g. /backend (default /)")
}
