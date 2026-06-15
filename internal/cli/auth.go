package cli

import (
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/auth"
	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/credstore"
	"github.com/MGM-Laboratory/mgm-cli/internal/ui"
)

// newAuthCommand builds `mgm auth` (mgm-account login) with `status` and
// `logout` subcommands. Running `mgm auth` with no subcommand logs in.
func newAuthCommand() *cobra.Command {
	var (
		device    bool
		noBrowser bool
	)
	c := &cobra.Command{
		Use:     "auth",
		Aliases: []string{"login"},
		Short:   "Sign in to your mgm account (Megumi Code)",
		Long: "Sign in to Megumi Code with your mgm account (Keycloak). By default a " +
			"browser opens for Authorization Code + PKCE; with no usable browser, or " +
			"with --device/--no-browser, an in-terminal device code is shown instead. " +
			"Credentials are stored under ~/.mgm/megumi and shared with `mgm megumi`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd, device, noBrowser)
		},
	}
	c.Flags().BoolVar(&device, "device", false, "Use the device-code flow (paste a code) instead of opening a browser")
	c.Flags().BoolVar(&noBrowser, "no-browser", false, "Do not open a browser; use the device-code flow")

	c.AddCommand(newAuthStatusCommand())
	c.AddCommand(newAuthLogoutCommand())
	return c
}

func runLogin(cmd *cobra.Command, device, noBrowser bool) error {
	a, err := auth.New()
	if err != nil {
		return err
	}
	ui.Title("Sign in to Megumi Code")

	notifier := auth.Notifier{
		OnBrowser: func(url string) {
			ui.Infof("Opening your browser to sign in…")
			ui.KV("url", url)
			ui.Infof("%s", ui.Dim("(if it doesn't open, re-run with --no-browser to paste a code)"))
		},
		OnDeviceCode: func(d auth.DeviceInstructions) {
			ui.Infof("To sign in, open this URL and enter the code:")
			ui.KV("url", d.VerificationURI)
			ui.Infof("code: %s", ui.Key(d.UserCode))
			if d.VerificationURIComplete != "" {
				ui.Infof("%s", ui.Dim("(or open "+d.VerificationURIComplete+" to skip typing the code)"))
			}
			ui.Infof("%s", ui.Dim("Waiting for authorization…"))
		},
	}

	creds, err := a.Login(cmd.Context(), auth.LoginOptions{
		ForceDevice: device,
		NoBrowser:   noBrowser,
		Notifier:    notifier,
	})
	if err != nil {
		return err
	}
	ui.Successf("Signed in as %s", identityLabel(creds))
	ui.KV("role", creds.Role)
	ui.KV("store", a.Store.Backend())
	return nil
}

func newAuthStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether you are signed in",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := auth.New()
			if err != nil {
				return err
			}
			creds, err := a.Current(cmd.Context(), true)
			if errors.Is(err, credstore.ErrNotFound) {
				ui.Warnf("Not signed in. Run `mgm auth` to sign in.")
				return nil
			}
			expiredSession := err != nil

			ui.KV("status", ui.SuccessText("signed in"))
			ui.KV("method", string(creds.Method))
			ui.KV("identity", identityLabel(creds))
			ui.KV("role", creds.Role)
			if !creds.Expiry.IsZero() {
				ui.KV("expires", creds.Expiry.UTC().Format(time.RFC3339))
			}
			ui.KV("store", a.Store.Backend())
			if expiredSession {
				ui.Warnf("Session expired — run `mgm auth` to sign in again.")
			}
			return nil
		},
	}
}

func newAuthLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Sign out and clear stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := auth.New()
			if err != nil {
				return err
			}
			if err := a.Logout(cmd.Context()); err != nil {
				return err
			}
			ui.Successf("Signed out.")
			return nil
		},
	}
}

// identityLabel picks the friendliest available label for a credential.
func identityLabel(c *credstore.Credentials) string {
	switch {
	case c.Email != "":
		return c.Email
	case c.Name != "":
		return c.Name
	case c.Subject != "":
		return c.Subject
	default:
		return "(unknown)"
	}
}
