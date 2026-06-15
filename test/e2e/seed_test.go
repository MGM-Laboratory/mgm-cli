//go:build e2e

package e2e

import (
	"os"
	"testing"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/credstore"
)

// seedAPICode writes a Megumi API code into the file credential store under
// megHome, so a subsequent non-interactive `mgm megumi --api-code` reuses it
// without prompting. The file backend derives its key from host/OS/uid plus a
// per-install salt stored under megHome, so a credential seeded here decrypts
// in the binary subprocess as long as it runs with the same MEGUMI_HOME and
// MEGUMI_CRED_STORE=file on the same host.
func seedAPICode(t *testing.T, megHome, code string) {
	t.Helper()
	if err := os.MkdirAll(megHome, 0o700); err != nil {
		t.Fatalf("mkdir megumi home: %v", err)
	}
	// Point the store at our temp home for the duration of the seed.
	t.Setenv("MEGUMI_HOME", megHome)
	t.Setenv("MEGUMI_CRED_STORE", "file")

	store, err := credstore.New()
	if err != nil {
		t.Fatalf("credstore.New: %v", err)
	}
	if store.Backend() != "file" {
		t.Fatalf("expected file credential backend, got %q", store.Backend())
	}
	if err := store.Save(&credstore.Credentials{
		Method:  credstore.MethodAPICode,
		APICode: code,
		Email:   "e2e@labmgm.org",
	}); err != nil {
		t.Fatalf("seed API code: %v", err)
	}

	// Sanity check the round-trip locally before the subprocess relies on it.
	got, err := store.Load()
	if err != nil || got == nil || got.APICode != code {
		t.Fatalf("seed verify failed: got=%+v err=%v", got, err)
	}
}
