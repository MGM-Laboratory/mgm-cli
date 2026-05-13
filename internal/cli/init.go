package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/MGM-Laboratory/mgm-cli/internal/projectfile"
	"github.com/MGM-Laboratory/mgm-cli/internal/ui"
)

func newInitCommand() *cobra.Command {
	var (
		sel   selectionFlags
		force bool
	)
	c := &cobra.Command{
		Use:   "init",
		Short: "Pin the current directory to a project/environment/folder",
		Long: "Writes a .mgm.yaml in the current directory recording the chosen project, environment, " +
			"and folder so subsequent commands skip the picker. Commit it to git.",
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

			cwd, _ := os.Getwd()
			pf := &projectfile.ProjectFile{
				ProjectID:   projectID,
				Environment: environment,
				Folder:      folder,
			}
			path, err := projectfile.Save(cwd, pf)
			if err != nil {
				return err
			}
			ui.Successf("Wrote %s", path)
			fmt.Fprintf(ui.Out, "  project_id: %s\n  environment: %s\n  folder: %s\n", projectID, environment, folder)
			_ = force
			return nil
		},
	}
	addSelectionFlags(c, &sel)
	c.Flags().BoolVar(&force, "force", false, "Overwrite an existing .mgm.yaml")
	return c
}
