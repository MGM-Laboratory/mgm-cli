package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/cli/browser"
	"golang.org/x/oauth2"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/oidc"
)

// errBrowserUnavailable signals that no browser could be opened, so the caller
// should fall back to the device grant.
var errBrowserUnavailable = errors.New("no browser available")

// pkceFlow runs Authorization Code + PKCE: it listens on an ephemeral loopback
// port, opens the browser to the authorization URL, and exchanges the captured
// code (bound to a PKCE verifier and a CSRF state) for tokens.
func (a *Authenticator) pkceFlow(ctx context.Context, ep *oidc.Endpoints, n Notifier) (*oauth2.Token, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start loopback listener: %w", err)
	}
	defer ln.Close()

	redirect := fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)
	oc := a.oauthConfig(ep, redirect)

	verifier := oauth2.GenerateVerifier()
	state, err := randomState()
	if err != nil {
		return nil, err
	}
	authURL := oc.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))

	handler, resCh := newCallbackHandler(state)
	srv := &http.Server{Handler: handler}
	go srv.Serve(ln)
	defer srv.Close()

	if n.OnBrowser != nil {
		n.OnBrowser(authURL)
	}
	if err := browser.OpenURL(authURL); err != nil {
		return nil, errBrowserUnavailable
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resCh:
		if res.err != nil {
			return nil, res.err
		}
		tok, err := oc.Exchange(ctx, res.code, oauth2.VerifierOption(verifier))
		if err != nil {
			return nil, fmt.Errorf("exchange authorization code: %w", err)
		}
		return tok, nil
	}
}

// callbackResult is what the loopback handler captures from the redirect.
type callbackResult struct {
	code string
	err  error
}

// newCallbackHandler returns an http.Handler for the loopback redirect and a
// channel that receives exactly one result. Factored out so the callback
// parsing is testable without a live IdP.
func newCallbackHandler(expectedState string) (http.Handler, <-chan callbackResult) {
	ch := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		res := parseCallback(r, expectedState)
		writeCallbackPage(w, res.err == nil)
		select {
		case ch <- res: // first result wins
		default:
		}
	})
	return mux, ch
}

// parseCallback validates the redirect query and extracts the authorization
// code, rejecting state mismatches and propagating IdP errors.
func parseCallback(r *http.Request, expectedState string) callbackResult {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		if desc := q.Get("error_description"); desc != "" {
			e = e + ": " + desc
		}
		return callbackResult{err: fmt.Errorf("authorization denied (%s)", e)}
	}
	if q.Get("state") != expectedState {
		return callbackResult{err: errors.New("state mismatch (possible CSRF) — login aborted")}
	}
	code := q.Get("code")
	if code == "" {
		return callbackResult{err: errors.New("authorization response missing code")}
	}
	return callbackResult{code: code}
}

func writeCallbackPage(w http.ResponseWriter, ok bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	msg := "Login failed. Return to your terminal."
	if ok {
		msg = "Login complete. You can close this tab and return to your terminal."
	}
	fmt.Fprintf(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>Megumi Code</title></head>"+
		"<body style=\"font-family:sans-serif;text-align:center;margin-top:4rem\"><h2>Megumi Code</h2><p>%s</p></body></html>", msg)
}

func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
