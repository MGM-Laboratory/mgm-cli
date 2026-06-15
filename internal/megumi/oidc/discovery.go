package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Endpoints are the subset of the OIDC discovery document the CLI uses.
type Endpoints struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	DeviceAuthEndpoint    string `json:"device_authorization_endpoint"`
	RevocationEndpoint    string `json:"revocation_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
}

// Discover fetches and validates the issuer's OIDC discovery document. The
// endpoint paths are never hardcoded — they come from this document.
func Discover(ctx context.Context, issuer string) (*Endpoints, error) {
	url := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc discovery: %s returned %s", url, resp.Status)
	}

	var ep Endpoints
	if err := json.NewDecoder(resp.Body).Decode(&ep); err != nil {
		return nil, fmt.Errorf("oidc discovery: decode %s: %w", url, err)
	}
	if ep.Issuer != "" && strings.TrimRight(ep.Issuer, "/") != strings.TrimRight(issuer, "/") {
		return nil, fmt.Errorf("oidc discovery: issuer mismatch (configured %q, document %q)", issuer, ep.Issuer)
	}
	if ep.AuthorizationEndpoint == "" || ep.TokenEndpoint == "" {
		return nil, fmt.Errorf("oidc discovery: %s is missing required endpoints", url)
	}
	return &ep, nil
}

// OAuth2Endpoint adapts the discovered endpoints to the oauth2 package's type,
// including the device authorization URL when advertised.
func (e *Endpoints) OAuth2Endpoint() oauth2.Endpoint {
	return oauth2.Endpoint{
		AuthURL:       e.AuthorizationEndpoint,
		TokenURL:      e.TokenEndpoint,
		DeviceAuthURL: e.DeviceAuthEndpoint,
	}
}
