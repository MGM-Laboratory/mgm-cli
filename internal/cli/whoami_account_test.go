package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/credstore"
	"github.com/MGM-Laboratory/mgm-cli/internal/ui"
)

// runWhoami executes the top-level whoami command with ui output captured.
func runWhoami(t *testing.T, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	oldOut, oldErr := ui.Out, ui.Err
	ui.Out, ui.Err = &buf, &buf
	t.Cleanup(func() { ui.Out, ui.Err = oldOut, oldErr })

	cmd := newAccountWhoamiCommand()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("whoami execute: %v", err)
	}
	return buf.String()
}

func TestWhoamiSignedInRendersIdentity(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MEGUMI_HOME", dir)
	t.Setenv("MEGUMI_CRED_STORE", "file")
	// Point the backend at a closed port so /api/v1/me fails fast and we exercise
	// the offline render path deterministically.
	t.Setenv("MEGUMI_BASE_URL", "http://127.0.0.1:1")

	store, err := credstore.New()
	if err != nil {
		t.Fatalf("credstore.New: %v", err)
	}
	// No refresh token => Current returns the record as-is (no network refresh).
	if err := store.Save(&credstore.Credentials{
		Method:  credstore.MethodAccount,
		Email:   "member@labmgm.org",
		Name:    "Lab Member",
		Subject: "kc-sub-1",
		Role:    "member",
		Issuer:  "https://iam.labmgm.org/realms/megumi",
	}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	out := runWhoami(t)
	for _, want := range []string{"member@labmgm.org", "Lab Member", "kc-sub-1", "account", "file"} {
		if !strings.Contains(out, want) {
			t.Errorf("whoami output missing %q\n--- output ---\n%s", want, out)
		}
	}
	if !strings.Contains(out, "mgm env whoami") {
		t.Errorf("whoami should point to the Infisical identity command\n%s", out)
	}
}

func TestWhoamiSignedOutJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MEGUMI_HOME", dir)
	t.Setenv("MEGUMI_CRED_STORE", "file")

	out := runWhoami(t, "--json")
	if !strings.Contains(out, `"signed_in": false`) {
		t.Errorf("expected signed_in false JSON, got:\n%s", out)
	}
}
