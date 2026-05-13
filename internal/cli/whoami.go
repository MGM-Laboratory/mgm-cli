package cli

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/mgm/mgm-cli/internal/ui"
)

func newWhoamiCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "whoami",
		Short: "Show identity metadata for the active credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			rt, err := resolveRuntime(ctx, globalProfile)
			if err != nil {
				return err
			}
			info, err := rt.Client.Whoami(ctx)
			if err != nil {
				ui.KV("host", rt.Profile.HostURL)
				ui.KV("client_id", rt.Profile.ClientID)
				ui.Warnf("identity endpoint not available: %v", err)
				return nil
			}
			enc := json.NewEncoder(ui.Out)
			enc.SetIndent("", "  ")
			return enc.Encode(info)
		},
	}
	return c
}
