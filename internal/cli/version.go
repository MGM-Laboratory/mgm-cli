package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mgm/mgm-cli/internal/ui"
	"github.com/mgm/mgm-cli/internal/version"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(ui.Out, "mgm %s\ncommit: %s\nbuilt: %s\n", version.Version, version.Commit, version.Date)
		},
	}
}
