//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"
)

// These smoke tests need no broker and no credentials. They run on every OS in
// CI (Linux, macOS, Windows) and assert the binary's no-auth surfaces behave
// and that error paths fail fast and cleanly (no hang, no panic).

func TestSmoke_Version(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}} {
		res := run(t, nil, args...)
		if res.ExitCode != 0 {
			t.Fatalf("mgm %v exit=%d, want 0\n%s", args, res.ExitCode, res.Combined)
		}
		if !strings.Contains(res.Stdout, "mgm ") {
			t.Errorf("mgm %v stdout missing version line: %q", args, res.Stdout)
		}
	}
}

func TestSmoke_RootHelp(t *testing.T) {
	res := run(t, nil, "--help")
	if res.ExitCode != 0 {
		t.Fatalf("mgm --help exit=%d, want 0\n%s", res.ExitCode, res.Combined)
	}
	for _, want := range []string{"mgm", "megumi", "auth", "whoami", "env", "status"} {
		if !strings.Contains(res.Combined, want) {
			t.Errorf("mgm --help output missing %q\n%s", want, res.Combined)
		}
	}
}

func TestSmoke_MegumiHelpIsBrandedAndNoAuth(t *testing.T) {
	// --help must forward to the embedded agent WITHOUT triggering auth, and be
	// Megumi-branded (no leftover upstream "crush" wording in the usage).
	res := run(t, nil, "megumi", "--help")
	if res.ExitCode != 0 {
		t.Fatalf("mgm megumi --help exit=%d, want 0\n%s", res.ExitCode, res.Combined)
	}
	out := res.Combined
	for _, want := range []string{"Megumi", "-p", "--dangerously-skip-permissions"} {
		if !strings.Contains(out, want) {
			t.Errorf("mgm megumi --help missing %q\n%s", want, out)
		}
	}
	// Usage line must read `megumi`, not `crush`.
	if strings.Contains(out, "Usage:\n  crush") || strings.Contains(out, "crush [flags]") {
		t.Errorf("mgm megumi --help still shows crush usage:\n%s", out)
	}
}

func TestSmoke_UnknownCommandFailsFast(t *testing.T) {
	res := runWithTimeout(t, 20*time.Second, nil, nil, "definitely-not-a-command")
	if res.TimedOut {
		t.Fatal("unknown command hung instead of failing fast")
	}
	if res.ExitCode == 0 {
		t.Errorf("unknown command exit=0, want non-zero\n%s", res.Combined)
	}
}

func TestSmoke_MegumiPrintNoCredsFailsFast(t *testing.T) {
	// Non-interactive `--api-code` with no stored code must error immediately
	// with a helpful message — never hang waiting for input and never panic.
	res := runWithTimeout(t, 30*time.Second, nil, nil, "megumi", "--api-code", "-p", "hello")
	if res.TimedOut {
		t.Fatal("megumi --api-code -p hung with no stored credential; expected a fast, clean failure")
	}
	if res.ExitCode == 0 {
		t.Errorf("expected non-zero exit with no credential, got 0\n%s", res.Combined)
	}
	if strings.Contains(strings.ToLower(res.Combined), "panic") {
		t.Errorf("output contains a panic:\n%s", res.Combined)
	}
	if !strings.Contains(strings.ToLower(res.Combined), "api code") {
		t.Errorf("expected a helpful 'API code' message, got:\n%s", res.Combined)
	}
}
