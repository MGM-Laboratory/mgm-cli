// Package auth implements the Megumi Code mgm-account login against Keycloak.
// It supports both OAuth flows the backend runbook requires — Authorization
// Code + PKCE (auto-open browser, loopback callback) and the Device
// Authorization Grant (in-terminal "open this URL, paste this code") — handles
// transparent token refresh, and persists everything through the shared
// credential store. The backend (via JWKS) remains authoritative for token
// validity and role; the CLI only decodes claims for display.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/credstore"
	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/oidc"
)

// Authenticator ties the resolved OIDC config to the credential store.
type Authenticator struct {
	Cfg   oidc.Config
	Store *credstore.Store
}

// New builds an Authenticator from the environment-resolved config and the
// shared credential store.
func New() (*Authenticator, error) {
	store, err := credstore.New()
	if err != nil {
		return nil, err
	}
	return &Authenticator{Cfg: oidc.LoadConfig(), Store: store}, nil
}

// DeviceInstructions carry what the user must do to complete a device login.
type DeviceInstructions struct {
	VerificationURI         string
	VerificationURIComplete string
	UserCode                string
	ExpiresAt               time.Time
}

// Notifier lets the CLI layer present flow progress with its own styling. All
// fields are optional.
type Notifier struct {
	// OnBrowser is called with the authorization URL just before the browser is
	// opened (PKCE flow).
	OnBrowser func(authURL string)
	// OnDeviceCode is called once a device code is obtained (device flow).
	OnDeviceCode func(DeviceInstructions)
}

// LoginOptions select the flow.
type LoginOptions struct {
	// ForceDevice always uses the device grant.
	ForceDevice bool
	// NoBrowser skips opening a browser; the device grant is used instead.
	NoBrowser bool
	Notifier  Notifier
}

// Login performs an interactive mgm-account login and persists the result.
// Default behavior tries PKCE (opening a browser) and falls back to the device
// grant when no browser can be opened. ForceDevice / NoBrowser go straight to
// the device grant.
func (a *Authenticator) Login(ctx context.Context, opts LoginOptions) (*credstore.Credentials, error) {
	ep, err := oidc.Discover(ctx, a.Cfg.Issuer)
	if err != nil {
		return nil, err
	}

	var tok *oauth2.Token
	if opts.ForceDevice || opts.NoBrowser {
		tok, err = a.deviceFlow(ctx, ep, opts.Notifier)
	} else {
		tok, err = a.pkceFlow(ctx, ep, opts.Notifier)
		if errors.Is(err, errBrowserUnavailable) {
			tok, err = a.deviceFlow(ctx, ep, opts.Notifier)
		}
	}
	if err != nil {
		return nil, err
	}

	creds := tokenToCreds(tok, nil, a.Cfg)
	if err := a.Store.Save(creds); err != nil {
		return nil, fmt.Errorf("persist credentials: %w", err)
	}
	return creds, nil
}

// Current loads the stored credentials. When refresh is true and the record is
// an account login with a refresh token, it transparently refreshes an expired
// access token and persists the result. Network/discovery failures during
// refresh return the stale (still-displayable) credentials rather than erroring,
// so `whoami`/`status` keep working offline; a genuine refresh rejection (e.g.
// an expired refresh token) is returned as an error.
func (a *Authenticator) Current(ctx context.Context, refresh bool) (*credstore.Credentials, error) {
	creds, err := a.Store.Load()
	if err != nil {
		return nil, err
	}
	if !refresh || creds.Method != credstore.MethodAccount || creds.RefreshToken == "" {
		return creds, nil
	}
	ps, err := a.tokenSource(ctx, creds)
	if err != nil {
		return creds, nil // discovery/network down — render what we have
	}
	if _, err := ps.Token(); err != nil {
		return creds, fmt.Errorf("refresh session (try `mgm auth` again): %w", err)
	}
	return ps.current(), nil
}

// TokenSource returns an auto-refreshing oauth2.TokenSource over stored
// credentials that persists rotated tokens back to the store. Intended for
// programmatic callers (e.g. the Phase 2 agent's HTTP client).
func (a *Authenticator) TokenSource(ctx context.Context, creds *credstore.Credentials) (oauth2.TokenSource, error) {
	return a.tokenSource(ctx, creds)
}

// Logout best-effort revokes the refresh token at the IdP, then clears the
// local store. A revocation failure never blocks logout.
func (a *Authenticator) Logout(ctx context.Context) error {
	creds, err := a.Store.Load()
	if errors.Is(err, credstore.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if creds.Method == credstore.MethodAccount && creds.RefreshToken != "" {
		if ep, derr := oidc.Discover(ctx, a.Cfg.Issuer); derr == nil {
			a.revoke(ctx, ep, creds.RefreshToken)
		}
	}
	return a.Store.Clear()
}

func (a *Authenticator) oauthConfig(ep *oidc.Endpoints, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:    a.Cfg.ClientID,
		Endpoint:    ep.OAuth2Endpoint(),
		RedirectURL: redirectURL,
		Scopes:      a.Cfg.Scopes,
	}
}

func (a *Authenticator) tokenSource(ctx context.Context, creds *credstore.Credentials) (*persistingSource, error) {
	ep, err := oidc.Discover(ctx, a.Cfg.Issuer)
	if err != nil {
		return nil, err
	}
	oc := a.oauthConfig(ep, "")
	base := oc.TokenSource(ctx, credsToToken(creds))
	return &persistingSource{base: base, store: a.Store, cfg: a.Cfg, creds: creds}, nil
}

// revoke posts the refresh token to the revocation endpoint (RFC 7009). Best
// effort: errors are ignored by the caller.
func (a *Authenticator) revoke(ctx context.Context, ep *oidc.Endpoints, refreshToken string) {
	if ep.RevocationEndpoint == "" {
		return
	}
	form := url.Values{
		"client_id":       {a.Cfg.ClientID},
		"token":           {refreshToken},
		"token_type_hint": {"refresh_token"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.RevocationEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req); err == nil {
		resp.Body.Close()
	}
}

// BackendIdentity is the backend's authoritative view of the caller, from
// GET /api/v1/me.
type BackendIdentity struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Method  string `json:"method"`
}

// FetchBackendIdentity asks the broker who it thinks the caller is. Used to
// confirm/enrich the locally-decoded identity in `whoami`; callers should treat
// failures as non-fatal (the backend may be unreachable).
func FetchBackendIdentity(ctx context.Context, baseURL, accessToken string) (*BackendIdentity, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v1/me"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend /api/v1/me returned %s", resp.Status)
	}
	var bi BackendIdentity
	if err := json.NewDecoder(resp.Body).Decode(&bi); err != nil {
		return nil, err
	}
	return &bi, nil
}
