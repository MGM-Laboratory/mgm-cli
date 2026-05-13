package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mgm/mgm-cli/internal/config"
	"github.com/mgm/mgm-cli/internal/infisical"
	"github.com/mgm/mgm-cli/internal/ui"
	"github.com/mgm/mgm-cli/internal/version"
)

func newConfigureCommand() *cobra.Command {
	var (
		clientID, clientSecret, hostURL string
		nonInteractive                  bool
		test                            bool
	)
	c := &cobra.Command{
		Use:   "configure",
		Short: "Set Infisical credentials and defaults for a profile",
		Long: "Prompts for client_id, client_secret, and host_url and writes them to ~/.mgm/config under the active profile. " +
			"Pass --client-id / --client-secret / --host to skip prompts.",
		Example: "  mgm env configure\n  mgm env configure --client-id ... --client-secret ... --host https://secrets.labmgm.org\n  mgm --profile prod env configure",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.New(globalProfile)
			if err != nil {
				return err
			}
			p := cfg.Load()

			ui.Title(fmt.Sprintf("Configure Infisical credentials (profile: %s)", cfg.ProfileName()))
			ui.Infof("Credentials will be saved to %s [%s]", cfg.Path(), cfg.ProfileName())

			if clientID == "" && !nonInteractive {
				clientID, err = ui.PromptString("Infisical Client ID", p.ClientID)
				if err != nil {
					return err
				}
			} else if clientID == "" {
				clientID = p.ClientID
			}

			if clientSecret == "" && !nonInteractive {
				clientSecret, err = ui.PromptSecret("Infisical Client Secret", p.ClientSecret)
				if err != nil {
					return err
				}
			} else if clientSecret == "" {
				clientSecret = p.ClientSecret
			}

			if hostURL == "" && !nonInteractive {
				h := p.HostURL
				if h == "" {
					h = config.DefaultHostURL
				}
				hostURL, err = ui.PromptString("Infisical Host URL", h)
				if err != nil {
					return err
				}
			} else if hostURL == "" {
				hostURL = p.HostURL
				if hostURL == "" {
					hostURL = config.DefaultHostURL
				}
			}

			p.ClientID = clientID
			p.ClientSecret = clientSecret
			p.HostURL = hostURL

			if test {
				ui.Infof("Verifying credentials...")
				cl := infisical.New(p.HostURL, p.ClientID, p.ClientSecret, "mgm-cli/"+version.Version)
				if err := cl.Ping(context.Background()); err != nil {
					ui.Errorf("Verification failed: %v", err)
					return err
				}
				ui.Successf("Credentials verified.")
			}

			if err := cfg.Save(p); err != nil {
				return err
			}
			ui.Successf("Saved %s [%s]", cfg.Path(), cfg.ProfileName())
			return nil
		},
	}
	c.Flags().StringVar(&clientID, "client-id", "", "Infisical universal-auth client ID")
	c.Flags().StringVar(&clientSecret, "client-secret", "", "Infisical universal-auth client secret")
	c.Flags().StringVar(&hostURL, "host", "", "Infisical host URL (default https://secrets.labmgm.org)")
	c.Flags().BoolVar(&nonInteractive, "no-input", false, "Fail instead of prompting for missing values")
	c.Flags().BoolVar(&test, "test", true, "Verify credentials by performing a login after saving")
	return c
}
