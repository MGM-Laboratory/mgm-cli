package oidc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newDiscoveryServer starts a server whose discovery document advertises itself
// as the issuer (resolved after the listener is up via the returned URL).
func newDiscoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	var issuer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":"%s/auth","token_endpoint":"%s/token","device_authorization_endpoint":"%s/auth/device","revocation_endpoint":"%s/revoke"}`,
			issuer, issuer, issuer, issuer, issuer)
	}))
	issuer = srv.URL
	t.Cleanup(srv.Close)
	return srv
}

func TestDiscoverParsesEndpoints(t *testing.T) {
	srv := newDiscoveryServer(t)
	issuer := srv.URL

	ep, err := Discover(context.Background(), issuer)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if ep.AuthorizationEndpoint != issuer+"/auth" || ep.TokenEndpoint != issuer+"/token" {
		t.Fatalf("unexpected endpoints: %+v", ep)
	}
	if ep.DeviceAuthEndpoint != issuer+"/auth/device" {
		t.Fatalf("device endpoint = %q", ep.DeviceAuthEndpoint)
	}
	if got := ep.OAuth2Endpoint(); got.DeviceAuthURL != issuer+"/auth/device" {
		t.Fatalf("OAuth2Endpoint device URL = %q", got.DeviceAuthURL)
	}
}

func TestDiscoverRejectsIssuerMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"issuer":"https://evil.example","authorization_endpoint":"https://evil.example/auth","token_endpoint":"https://evil.example/token"}`)
	}))
	defer srv.Close()
	if _, err := Discover(context.Background(), srv.URL); err == nil {
		t.Fatal("expected issuer mismatch error")
	}
}
