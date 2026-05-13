package cli

import (
	"github.com/spf13/cobra"

	"github.com/MGM-Laboratory/mgm-cli/internal/version"
)

var globalProfile string

// NewRootCommand assembles the full `mgm` command tree.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "mgm",
		Short: "MGM internal CLI",
		Long: "mgm is the MGM internal CLI — a single tool for everyday MGM ops.\n\n" +
			"Namespaces:\n" +
			"  mgm env      Pull/push secrets to self-hosted Infisical\n" +
			"  mgm status   Check MGM service health (Gatus)\n",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version.Version,
	}
	root.SetVersionTemplate("mgm {{.Version}}\n")
	root.PersistentFlags().StringVar(&globalProfile, "profile", "", "Config profile to use (default: $MGM_PROFILE or \"default\")")

	root.AddCommand(newEnvCommand())
	root.AddCommand(newServiceStatusCommand())
	root.AddCommand(newVersionCommand())
	root.AddCommand(newCompletionCommand())
	return root
}
