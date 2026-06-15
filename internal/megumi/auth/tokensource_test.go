package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/credstore"
)

// idpServer serves OIDC discovery plus a refresh-token endpoint that mints a
// fresh JWT access token and rotates the refresh token.
func idpServer(t *testing.T, newAccessToken string) *httptest.Server {
	t.Helper()
	var issuer string
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":"%s/auth","token_endpoint":"%s/token","device_authorization_endpoint":"%s/auth/device","revocation_endpoint":"%s/revoke"}`,
			issuer, issuer, issuer, issuer, issuer)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "rt-1" {
			http.Error(w, "bad refresh request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":%q,"token_type":"Bearer","expires_in":3600,"refresh_token":"rt-2"}`, newAccessToken)
	})
	srv := httptest.NewServer(mux)
	issuer = srv.URL
	t.Cleanup(srv.Close)
	return srv
}

func TestCurrentRefreshesAndPersists(t *testing.T) {
	freshToken := makeJWT(t, map[string]any{
		"sub": "u-refresh", "email": "r@x.io", "name": "Refreshed User",
		"realm_access": map[string]any{"roles": []string{"megumi-operator"}},
	})
	srv := idpServer(t, freshToken)

	dir := t.TempDir()
	t.Setenv("MEGUMI_HOME", dir)
	t.Setenv("MEGUMI_CRED_STORE", "file")
	t.Setenv("MEGUMI_OIDC_ISSUER", srv.URL)
	t.Setenv("MEGUMI_OIDC_CLIENT_ID", "megumi-cli")

	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Seed an expired access token with a valid refresh token.
	expired := &credstore.Credentials{
		Method:       credstore.MethodAccount,
		AccessToken:  "old-expired-token",
		RefreshToken: "rt-1",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
		Issuer:       srv.URL,
		Role:         RoleMember,
	}
	if err := a.Store.Save(expired); err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	got, err := a.Current(context.Background(), true)
	if err != nil {
		t.Fatalf("Current(refresh): %v", err)
	}
	if got.AccessToken != freshToken {
		t.Fatalf("access token not refreshed: got %q", got.AccessToken)
	}
	if got.RefreshToken != "rt-2" {
		t.Fatalf("refresh token not rotated: got %q", got.RefreshToken)
	}
	if got.Role != RoleOperator {
		t.Fatalf("identity not re-derived from fresh token: role = %q", got.Role)
	}

	// The rotation must be persisted, not just returned.
	reloaded, err := a.Store.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.AccessToken != freshToken || reloaded.RefreshToken != "rt-2" {
		t.Fatalf("refresh not persisted: %+v", reloaded)
	}
}

func TestCurrentNoRefreshWhenNotRequested(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MEGUMI_HOME", dir)
	t.Setenv("MEGUMI_CRED_STORE", "file")

	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	seed := &credstore.Credentials{Method: credstore.MethodAccount, AccessToken: "tok", RefreshToken: "rt", Role: RoleMember}
	if err := a.Store.Save(seed); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := a.Current(context.Background(), false)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if got.AccessToken != "tok" {
		t.Fatalf("unexpected token: %q", got.AccessToken)
	}
}

func TestCurrentNotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MEGUMI_HOME", dir)
	t.Setenv("MEGUMI_CRED_STORE", "file")
	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := a.Current(context.Background(), true); err != credstore.ErrNotFound {
		t.Fatalf("Current on empty = %v, want ErrNotFound", err)
	}
}
