package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mgm/mgm-cli/internal/ui"
)

// `mgm env projects` — list all accessible projects.
func newProjectsCommand() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "projects",
		Short: "List accessible Infisical projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			rt, err := resolveRuntime(ctx, globalProfile)
			if err != nil {
				return err
			}
			projects, err := rt.Client.ListProjects(ctx)
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(ui.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(projects)
			}
			sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
			tw := tabwriter.NewWriter(ui.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tSLUG\tID")
			for _, p := range projects {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", p.Name, p.Slug, p.ID)
			}
			return tw.Flush()
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON")
	return c
}

// `mgm env environments` — list envs for a project.
func newEnvironmentsCommand() *cobra.Command {
	var (
		projectFlag string
		jsonOut     bool
	)
	c := &cobra.Command{
		Use:     "environments",
		Aliases: []string{"envs"},
		Short:   "List environments for a project (e.g. dev, stg, prod)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			rt, err := resolveRuntime(ctx, globalProfile)
			if err != nil {
				return err
			}
			projectID, _, _, err := rt.resolveSelection(ctx, selectionFlags{Project: projectFlag})
			if err != nil {
				return err
			}
			envs, err := rt.Client.ListEnvironments(ctx, projectID)
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(ui.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(envs)
			}
			tw := tabwriter.NewWriter(ui.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "SLUG\tNAME")
			for _, e := range envs {
				fmt.Fprintf(tw, "%s\t%s\n", e.Slug, e.Name)
			}
			return tw.Flush()
		},
	}
	c.Flags().StringVarP(&projectFlag, "project", "p", "", "Project ID or slug")
	c.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON")
	return c
}

// `mgm env folders` — list folders at a path.
func newFoldersCommand() *cobra.Command {
	var (
		sel     selectionFlags
		jsonOut bool
	)
	c := &cobra.Command{
		Use:   "folders",
		Short: "List folders at the selected project/env path",
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
			if jsonOut {
				enc := json.NewEncoder(ui.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(folders)
			}
			ui.Title(fmt.Sprintf("%s · %s · %s", projectID, environment, folder))
			if len(folders) == 0 {
				ui.Infof("  %s", ui.Dim("(no subfolders)"))
				return nil
			}
			sort.Slice(folders, func(i, j int) bool { return folders[i].Name < folders[j].Name })
			for _, f := range folders {
				fmt.Fprintf(ui.Out, "  %s/\n", f.Name)
			}
			return nil
		},
	}
	addSelectionFlags(c, &sel)
	c.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON")
	return c
}
