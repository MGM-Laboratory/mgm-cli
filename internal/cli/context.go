package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/mgm/mgm-cli/internal/config"
	"github.com/mgm/mgm-cli/internal/infisical"
	"github.com/mgm/mgm-cli/internal/projectfile"
	"github.com/mgm/mgm-cli/internal/ui"
	"github.com/mgm/mgm-cli/internal/version"
)

// runtimeCtx is everything an env subcommand needs: live config, an
// authenticated Infisical client, and the resolved project-file defaults.
type runtimeCtx struct {
	Cfg     *config.Manager
	Profile config.Profile
	Client  *infisical.Client
	Project *projectfile.ProjectFile
	Cwd     string
}

// resolveProfile loads the active profile, prompting the user to fill in any
// missing credentials when stdin is interactive. When non-interactive and
// credentials are missing it returns a hard error.
func resolveProfile(cfg *config.Manager) (config.Profile, error) {
	p := cfg.Load()
	if p.IsConfigured() {
		return p, nil
	}
	if !ui.IsInteractive() {
		return p, fmt.Errorf("no Infisical credentials configured for profile %q. Run `mgm env configure` or set MGM_CLIENT_ID / MGM_CLIENT_SECRET / MGM_HOST_URL", cfg.ProfileName())
	}

	ui.Title(fmt.Sprintf("Configure Infisical credentials (profile: %s)", cfg.ProfileName()))
	ui.Infof("Credentials will be saved to %s [%s]", cfg.Path(), cfg.ProfileName())

	id, err := ui.PromptString("Infisical Client ID", p.ClientID)
	if err != nil {
		return p, err
	}
	secret, err := ui.PromptSecret("Infisical Client Secret", p.ClientSecret)
	if err != nil {
		return p, err
	}
	host := p.HostURL
	if host == "" {
		host = config.DefaultHostURL
	}
	host, err = ui.PromptString("Infisical Host URL", host)
	if err != nil {
		return p, err
	}

	p.ClientID = id
	p.ClientSecret = secret
	p.HostURL = host

	if err := cfg.Save(p); err != nil {
		return p, err
	}
	ui.Successf("Saved %s [%s]", cfg.Path(), cfg.ProfileName())
	return p, nil
}

// newClient builds an authenticated Infisical client for p.
func newClient(ctx context.Context, p config.Profile) (*infisical.Client, error) {
	c := infisical.New(p.HostURL, p.ClientID, p.ClientSecret, "mgm-cli/"+version.Version)
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// resolveRuntime is the standard prelude shared by every Infisical subcommand.
func resolveRuntime(ctx context.Context, profileName string) (*runtimeCtx, error) {
	cfg, err := config.New(profileName)
	if err != nil {
		return nil, err
	}
	p, err := resolveProfile(cfg)
	if err != nil {
		return nil, err
	}
	client, err := newClient(ctx, p)
	if err != nil {
		return nil, err
	}
	cwd, _ := os.Getwd()
	pf, _, _ := projectfile.Load(cwd)

	return &runtimeCtx{
		Cfg:     cfg,
		Profile: p,
		Client:  client,
		Project: pf,
		Cwd:     cwd,
	}, nil
}
