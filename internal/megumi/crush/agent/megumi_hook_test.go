package agent

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// captureRT records the request it receives and returns a canned response.
type captureRT struct{ got *http.Request }

func (c *captureRT) RoundTrip(r *http.Request) (*http.Response, error) {
	c.got = r
	return &http.Response{StatusCode: 200, Body: http.NoBody, Header: make(http.Header)}, nil
}

func TestMegumiCredentialTransportInjectsXAPIKey(t *testing.T) {
	old := CredentialProvider
	t.Cleanup(func() { CredentialProvider = old })
	CredentialProvider = func() (string, error) { return "live-token-123", nil }

	cap := &captureRT{}
	tr := megumiCredentialTransport{base: cap}

	req := httptest.NewRequest(http.MethodPost, "https://broker.example/v1/messages", nil)
	req.Header.Set("Authorization", "Bearer should-be-removed")
	req.Header.Set("x-api-key", "stale")

	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if got := cap.got.Header.Get("x-api-key"); got != "live-token-123" {
		t.Errorf("x-api-key = %q, want live-token-123", got)
	}
	if got := cap.got.Header.Get("Authorization"); got != "" {
		t.Errorf("Authorization should be cleared, got %q", got)
	}
	// Original request must be untouched (we clone).
	if req.Header.Get("x-api-key") != "stale" {
		t.Error("original request was mutated")
	}
}

func TestMegumiCredentialTransportSetsEffort(t *testing.T) {
	oldCred := CredentialProvider
	t.Cleanup(func() { CredentialProvider = oldCred })
	CredentialProvider = func() (string, error) { return "tok", nil }

	// Default tier when unset.
	if got := currentEffort(); got != "medium" {
		t.Fatalf("default effort = %q, want medium", got)
	}

	SetEffort("high")
	t.Cleanup(func() { SetEffort("medium") })

	cap := &captureRT{}
	req := httptest.NewRequest(http.MethodPost, "https://broker.example/v1/messages", nil)
	if _, err := (megumiCredentialTransport{base: cap}).RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if got := cap.got.Header.Get("x-megumi-effort"); got != "high" {
		t.Errorf("x-megumi-effort = %q, want high", got)
	}
}

func TestMegumiCredentialTransportPropagatesError(t *testing.T) {
	old := CredentialProvider
	t.Cleanup(func() { CredentialProvider = old })
	CredentialProvider = func() (string, error) { return "", errors.New("token unavailable") }

	tr := megumiCredentialTransport{base: &captureRT{}}
	req := httptest.NewRequest(http.MethodGet, "https://broker.example/v1/messages", nil)
	if _, err := tr.RoundTrip(req); err == nil {
		t.Fatal("expected error from credential provider to propagate")
	}
}
