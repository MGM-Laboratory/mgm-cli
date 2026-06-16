package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/oauth2"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/oidc"
)

// deviceFlow runs the OAuth 2.0 Device Authorization Grant: it requests a device
// code, shows the user a verification URL + code to enter on another device, and
// polls the token endpoint until they finish. This is the in-terminal fallback
// when no browser can be opened.
func (a *Authenticator) deviceFlow(ctx context.Context, ep *oidc.Endpoints, n Notifier) (*oauth2.Token, error) {
	if ep.DeviceAuthEndpoint == "" {
		return nil, errors.New("identity provider does not advertise a device authorization endpoint")
	}
	oc := a.oauthConfig(ep, "")

	// Bind the device flow to a PKCE verifier: public clients that enforce a
	// code-challenge method (Keycloak "Proof Key for Code Exchange" = S256) also
	// require it on the device grant, so we send the challenge here and the
	// verifier at the token exchange below.
	verifier := oauth2.GenerateVerifier()

	da, err := oc.DeviceAuth(ctx, oauth2.S256ChallengeOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}
	if n.OnDeviceCode != nil {
		n.OnDeviceCode(DeviceInstructions{
			VerificationURI:         da.VerificationURI,
			VerificationURIComplete: da.VerificationURIComplete,
			UserCode:                da.UserCode,
			ExpiresAt:               da.Expiry,
		})
	}

	// DeviceAccessToken polls at the server-specified interval until the user
	// approves, the code expires, or ctx is cancelled.
	tok, err := oc.DeviceAccessToken(ctx, da, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("device authorization: %w", err)
	}
	return tok, nil
}
