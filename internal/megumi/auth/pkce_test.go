package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// driveCallback issues a single GET to the loopback handler and returns the
// captured result.
func driveCallback(t *testing.T, expectedState, rawQuery string) callbackResult {
	t.Helper()
	handler, ch := newCallbackHandler(expectedState)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/callback?" + rawQuery)
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	resp.Body.Close()
	return <-ch
}

func TestCallbackCapturesCode(t *testing.T) {
	res := driveCallback(t, "the-state", "code=auth-code-123&state=the-state")
	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.code != "auth-code-123" {
		t.Fatalf("code = %q, want auth-code-123", res.code)
	}
}

func TestCallbackRejectsStateMismatch(t *testing.T) {
	res := driveCallback(t, "expected", "code=x&state=tampered")
	if res.err == nil {
		t.Fatal("expected state-mismatch error")
	}
	if res.code != "" {
		t.Fatalf("code should be empty on state mismatch, got %q", res.code)
	}
}

func TestCallbackPropagatesIdPError(t *testing.T) {
	res := driveCallback(t, "s", "error=access_denied&error_description=user+said+no&state=s")
	if res.err == nil {
		t.Fatal("expected error to be propagated")
	}
}

func TestCallbackMissingCode(t *testing.T) {
	res := driveCallback(t, "s", "state=s")
	if res.err == nil {
		t.Fatal("expected missing-code error")
	}
}
