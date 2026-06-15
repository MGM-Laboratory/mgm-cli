package cli

import (
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/auth"
	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/credstore"
	"github.com/MGM-Laboratory/mgm-cli/internal/ui"
)

// identityView is the rendered shape of `mgm whoami` (and the JSON form).
type identityView struct {
	SignedIn         bool   `json:"signed_in"`
	Method           string `json:"method,omitempty"`
	Subject          string `json:"subject,omitempty"`
	Email            string `json:"email,omitempty"`
	Name             string `json:"name,omitempty"`
	Role             string `json:"role,omitempty"`
	Issuer           string `json:"issuer,omitempty"`
	Expiry           string `json:"expiry,omitempty"`
	BackendConfirmed bool   `json:"backend_confirmed"`
	Store            string `json:"store,omitempty"`
}

// newAccountWhoamiCommand builds the top-level `mgm whoami`, which reports the
// active Megumi/mgm-account identity. The Infisical identity remains reachable
// via `mgm env whoami`.
func newAccountWhoamiCommand() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "whoami",
		Short: "Show the active Megumi (mgm-account) identity",
		Long: "Reports the authenticated Megumi identity (subject, email, role, and " +
			"auth method) from `mgm auth`. For the Infisical identity, use `mgm env whoami`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := auth.New()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			creds, err := a.Current(ctx, true)
			if errors.Is(err, credstore.ErrNotFound) {
				return renderSignedOut(asJSON)
			}
			expiredSession := err != nil // refresh rejected; show stale identity + note

			view := identityView{
				SignedIn: true,
				Method:   string(creds.Method),
				Subject:  creds.Subject,
				Email:    creds.Email,
				Name:     creds.Name,
				Role:     creds.Role,
				Issuer:   creds.Issuer,
				Store:    a.Store.Backend(),
			}
			if !creds.Expiry.IsZero() {
				view.Expiry = creds.Expiry.UTC().Format(time.RFC3339)
			}

			// Confirm/enrich the role with the broker's authoritative view. Never
			// fatal: the backend may be unreachable.
			if !expiredSession && creds.Method == credstore.MethodAccount && creds.AccessToken != "" {
				if bi, e := auth.FetchBackendIdentity(ctx, a.Cfg.BaseURL, creds.AccessToken); e == nil {
					view.BackendConfirmed = true
					if bi.Role != "" {
						view.Role = bi.Role
					}
					if bi.Email != "" {
						view.Email = bi.Email
					}
				}
			}

			if asJSON {
				return writeJSON(view)
			}
			renderIdentity(view)
			if expiredSession {
				ui.Warnf("Session expired — run `mgm auth` to sign in again.")
			}
			ui.Infof("%s", ui.Dim("Infisical identity: run `mgm env whoami`"))
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "Output identity as JSON")
	return c
}

func renderSignedOut(asJSON bool) error {
	if asJSON {
		return writeJSON(identityView{SignedIn: false})
	}
	ui.Warnf("Not signed in. Run `mgm auth` to sign in.")
	return nil
}

// renderIdentity prints the human-readable identity block.
func renderIdentity(v identityView) {
	ui.KV("method", v.Method)
	if v.Name != "" {
		ui.KV("name", v.Name)
	}
	if v.Email != "" {
		ui.KV("email", v.Email)
	}
	if v.Subject != "" {
		ui.KV("subject", v.Subject)
	}
	role := v.Role
	if v.BackendConfirmed {
		role += ui.Dim(" (confirmed by backend)")
	} else if v.Method == string(credstore.MethodAccount) {
		role += ui.Dim(" (local; backend not reached)")
	}
	ui.KV("role", role)
	if v.Issuer != "" {
		ui.KV("issuer", v.Issuer)
	}
	if v.Expiry != "" {
		ui.KV("expires", v.Expiry)
	}
	ui.KV("store", v.Store)
}
