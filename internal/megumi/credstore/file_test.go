package credstore

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// newTestStore forces the file backend in an isolated MEGUMI_HOME.
func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MEGUMI_HOME", dir)
	t.Setenv("MEGUMI_CRED_STORE", "file")
	s, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.Backend() != "file" {
		t.Fatalf("Backend = %q, want file", s.Backend())
	}
	return s, dir
}

func TestFileStoreRoundTrip(t *testing.T) {
	s, _ := newTestStore(t)

	if _, err := s.Load(); err != ErrNotFound {
		t.Fatalf("Load on empty = %v, want ErrNotFound", err)
	}

	in := &Credentials{
		Method:       MethodAccount,
		AccessToken:  "super-secret-access-token-value",
		RefreshToken: "super-secret-refresh-token-value",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).UTC().Round(time.Second),
		Subject:      "abc-123",
		Email:        "user@example.com",
		Name:         "Test User",
		Role:         "operator",
		Issuer:       "https://iam.example.org/realms/megumi",
	}
	if err := s.Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	out, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.AccessToken != in.AccessToken || out.RefreshToken != in.RefreshToken ||
		out.Subject != in.Subject || out.Email != in.Email || out.Role != in.Role {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
	if out.UpdatedAt.IsZero() {
		t.Fatal("Save did not stamp UpdatedAt")
	}

	if err := s.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := s.Load(); err != ErrNotFound {
		t.Fatalf("Load after Clear = %v, want ErrNotFound", err)
	}
	// Clear is idempotent.
	if err := s.Clear(); err != nil {
		t.Fatalf("second Clear: %v", err)
	}
}

func TestFileStorePermsAndNoPlaintext(t *testing.T) {
	s, dir := newTestStore(t)

	secret := "PLAINTEXT-TOKEN-MUST-NOT-APPEAR-ON-DISK"
	if err := s.Save(&Credentials{Method: MethodAccount, AccessToken: secret}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	credPath := filepath.Join(dir, credFileName)
	info, err := os.Stat(credPath)
	if err != nil {
		t.Fatalf("stat cred file: %v", err)
	}
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("cred file perms = %o, want 600", perm)
		}
	}

	raw, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("read cred file: %v", err)
	}
	if bytes.Contains(raw, []byte(secret)) {
		t.Fatal("plaintext secret found in on-disk credentials file")
	}
}
