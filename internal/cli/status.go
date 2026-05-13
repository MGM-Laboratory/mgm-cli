package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mgm/mgm-cli/internal/config"
	"github.com/mgm/mgm-cli/internal/projectfile"
	"github.com/mgm/mgm-cli/internal/ui"
)

func newStatusCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "status",
		Short: "Show config, profile, project pinning, and connection status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.New(globalProfile)
			if err != nil {
				return err
			}
			p := cfg.Load()

			ui.Title("Config")
			ui.KV("file", cfg.Path())
			ui.KV("profile", cfg.ProfileName())
			ui.KV("host", p.HostURL)
			if p.ClientID != "" {
				ui.KV("client_id", p.ClientID)
			} else {
				ui.KV("client_id", ui.Dim("(unset)"))
			}
			if p.ClientSecret != "" {
				ui.KV("client_secret", "(set)")
			} else {
				ui.KV("client_secret", ui.Dim("(unset)"))
			}
			ui.KV("default_project_id", orDim(p.DefaultProjectID))
			ui.KV("default_environment", orDim(p.DefaultEnvironment))
			ui.KV("default_folder", orDim(p.DefaultFolder))

			profiles := cfg.Profiles()
			if len(profiles) > 1 {
				ui.Infof("\n%s", ui.Key("Profiles"))
				for _, name := range profiles {
					marker := "  "
					if name == cfg.ProfileName() {
						marker = "* "
					}
					fmt.Fprintf(ui.Out, "%s%s\n", marker, name)
				}
			}

			pf, pfPath, _ := projectfile.Load(".")
			ui.Infof("\n%s", ui.Key("Project pin"))
			if pf == nil {
				ui.Infof("  %s", ui.Dim("(no .mgm.yaml in cwd or ancestors)"))
			} else {
				ui.KV("file", pfPath)
				ui.KV("project_id", orDim(pf.ProjectID))
				ui.KV("environment", orDim(pf.Environment))
				ui.KV("folder", orDim(pf.Folder))
			}

			ui.Infof("\n%s", ui.Key("Connection"))
			if !p.IsConfigured() {
				ui.Warnf("  not configured — run `mgm env configure`")
				return nil
			}
			client, err := newClient(context.Background(), p)
			if err != nil {
				ui.Errorf("  login failed: %v", err)
				return nil
			}
			_ = client
			ui.Successf("  ok — authenticated to %s", p.HostURL)
			return nil
		},
	}
	return c
}

func orDim(s string) string {
	if s == "" {
		return ui.Dim("(unset)")
	}
	return s
}
