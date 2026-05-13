package cli

import "github.com/spf13/cobra"

// newEnvCommand builds `mgm env ...`.
func newEnvCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "env",
		Short: "Manage environment variables and secrets via Infisical",
		Long: "Pull, push, view, and edit secrets stored in Infisical. " +
			"Run `mgm env configure` first to save credentials, or set MGM_CLIENT_ID / MGM_CLIENT_SECRET / MGM_HOST_URL.",
	}
	c.AddCommand(
		newConfigureCommand(),
		newListCommand(),
		newPullCommand(),
		newPushCommand(),
		newGetCommand(),
		newSetCommand(),
		newDeleteCommand(),
		newDiffCommand(),
		newRunCommand(),
		newExportCommand(),
		newInitCommand(),
		newStatusCommand(),
		newWhoamiCommand(),
		newProjectsCommand(),
		newEnvironmentsCommand(),
		newFoldersCommand(),
	)
	return c
}
