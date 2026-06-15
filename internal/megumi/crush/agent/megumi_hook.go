package agent

import (
	"net/http"
	"sync/atomic"
)

// CredentialProvider, when set by the embedding mgm CLI, supplies the Megumi
// broker credential (an mgm-account access token or a Megumi API code) for each
// outbound request. It is consulted per request so account access tokens can be
// refreshed transparently mid-session. The returned value is sent as the
// x-api-key header (never Authorization: Bearer) — which is exactly what the
// Megumi broker expects. When nil, the fork behaves like upstream Crush.
//
// This is a one-way seam: mgm sets the variable; the fork only reads it. No
// import of mgm packages occurs here, so there is no dependency cycle.
var CredentialProvider func() (string, error)

// effortTier holds the live extended-thinking effort tier (low|medium|high) sent
// to the broker as the x-megumi-effort header. mgm seeds it at launch and the
// in-session /effort picker updates it; the transport reads it per request.
var effortTier atomic.Value // string

// SetEffort sets the live effort tier (low|medium|high). Called by mgm at launch
// and by the /effort picker.
func SetEffort(tier string) {
	if tier != "" {
		effortTier.Store(tier)
	}
}

func currentEffort() string {
	if v, ok := effortTier.Load().(string); ok && v != "" {
		return v
	}
	return "medium"
}

// megumiCredentialTransport injects the live Megumi credential as the x-api-key
// header on every request, overriding whatever the SDK set.
type megumiCredentialTransport struct {
	base http.RoundTripper
}

func (t megumiCredentialTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cred, err := CredentialProvider()
	if err != nil {
		return nil, err
	}
	// Clone so we never mutate a request the caller may reuse.
	r := req.Clone(req.Context())
	r.Header.Set("x-api-key", cred)
	r.Header.Set("x-megumi-effort", currentEffort())
	r.Header.Del("Authorization")
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r)
}

// megumiCredentialClient builds an *http.Client that injects the live credential.
func megumiCredentialClient() *http.Client {
	return &http.Client{Transport: megumiCredentialTransport{base: http.DefaultTransport}}
}
