package cli

import (
	"github.com/spf13/cobra"

	"github.com/MGM-Laboratory/mgm-cli/internal/version"
)

var globalProfile string

// NewRootCommand assembles the full `mgm` command tree.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "mgm",
		Short:         "MGM internal CLI",
		Long:          "mgm is the MGM internal CLI. The env subcommand manages secrets stored in self-hosted Infisical (https://secrets.labmgm.org).",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version.Version,
	}
	root.SetVersionTemplate("mgm {{.Version}}\n")
	root.PersistentFlags().StringVar(&globalProfile, "profile", "", "Config profile to use (default: $MGM_PROFILE or \"default\")")

	root.AddCommand(newEnvCommand())
	root.AddCommand(newVersionCommand())
	root.AddCommand(newCompletionCommand())
	return root
}
