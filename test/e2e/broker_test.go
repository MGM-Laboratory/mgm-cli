//go:build e2e

package e2e

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// requireBrokerCapable skips on platforms where seeding the file credential
// store for a subprocess is unreliable (Windows: os.Getuid()==-1 and keychain
// quirks). The full pipeline is exercised on Linux and macOS.
func requireBrokerCapable(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("broker round-trip seeding is exercised on Linux/macOS")
	}
}

// brokerEnv wires a seeded credential + the mock broker into the subprocess
// environment, returning the env slice to pass to run().
func brokerEnv(t *testing.T, broker *mockBroker) (megHome string, env []string) {
	t.Helper()
	megHome = filepath.Join(t.TempDir(), "megumi")
	seedAPICode(t, megHome, "mgm_e2e_secret_code")
	env = []string{
		"MEGUMI_HOME=" + megHome,
		"MEGUMI_CRED_STORE=file",
		"MEGUMI_BASE_URL=" + broker.URL,
		"MEGUMI_OIDC_ISSUER=" + broker.URL, // unused on the api-code path; kept off the real IdP
	}
	return megHome, env
}

// TestE2E_Broker_NonInteractiveOutput is the headline end-to-end test: a real
// `mgm megumi -p` run, authenticated with a Megumi API code, brokered through a
// mock server that streams an Anthropic SSE response — asserting the model text
// reaches stdout.
func TestE2E_Broker_NonInteractiveOutput(t *testing.T) {
	requireBrokerCapable(t)

	const reply = "Hello from the mock broker."
	broker := newMockBroker(t, reply)
	_, env := brokerEnv(t, broker)

	res := runWithTimeout(t, 90*time.Second, nil, env, "megumi", "--api-code", "-p", "say hello")
	if res.TimedOut {
		t.Fatalf("one-shot run hung:\n%s", res.Combined)
	}
	if res.ExitCode != 0 {
		t.Fatalf("one-shot run exit=%d, want 0\nstdout:%s\nstderr:%s", res.ExitCode, res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stdout, reply) {
		t.Errorf("model reply missing from stdout.\nwant substring: %q\nstdout: %q", reply, res.Stdout)
	}
	if len(broker.messageRequests()) == 0 {
		t.Error("broker never received a /v1/messages request")
	}
}

// TestE2E_Broker_RequestShape asserts the precise shape of what the CLI sends to
// the broker: the correct path, the Megumi credential injected as x-api-key
// (with Authorization stripped), the effort/project headers, and the user
// prompt carried in the Anthropic request body.
func TestE2E_Broker_RequestShape(t *testing.T) {
	requireBrokerCapable(t)

	broker := newMockBroker(t, "ok")
	_, env := brokerEnv(t, broker)
	env = append(env, "MEGUMI_EFFORT=high")

	const prompt = "uniquePromptMarker12345"
	res := runWithTimeout(t, 90*time.Second, nil, env, "megumi", "--api-code", "-p", prompt)
	if res.TimedOut {
		t.Fatalf("run hung:\n%s", res.Combined)
	}
	if res.ExitCode != 0 {
		t.Fatalf("run exit=%d, want 0\n%s", res.ExitCode, res.Combined)
	}

	reqs := broker.messageRequests()
	if len(reqs) == 0 {
		t.Fatal("no /v1/messages request captured")
	}
	r := reqs[0]

	if r.Method != "POST" {
		t.Errorf("method = %q, want POST", r.Method)
	}
	if r.APIKey != "mgm_e2e_secret_code" {
		t.Errorf("x-api-key = %q, want the injected Megumi API code", r.APIKey)
	}
	if r.Authorization != "" {
		t.Errorf("Authorization header = %q, want empty (the credential hook strips it)", r.Authorization)
	}
	if r.Effort != "high" {
		t.Errorf("x-megumi-effort = %q, want high", r.Effort)
	}
	if r.Project == "" {
		t.Errorf("x-megumi-project header missing")
	}
	if r.AnthropicVer == "" {
		t.Errorf("anthropic-version header missing (SDK should set it)")
	}
	if !strings.Contains(r.UserPromptInBody, prompt) {
		t.Errorf("user prompt %q not found in request body; got %q", prompt, r.UserPromptInBody)
	}
}

// TestE2E_Broker_IdentityReachesModel asserts the Megumi identity makes it into
// the request sent upstream: the system prompt says "Megumi Code" and never
// "You are Crush". This validates both the template rebrand and the locked
// provider's system_prompt_prefix on a real run.
func TestE2E_Broker_IdentityReachesModel(t *testing.T) {
	requireBrokerCapable(t)

	broker := newMockBroker(t, "ok")
	_, env := brokerEnv(t, broker)

	res := runWithTimeout(t, 90*time.Second, nil, env, "megumi", "--api-code", "-p", "who made you?")
	if res.TimedOut {
		t.Fatalf("run hung:\n%s", res.Combined)
	}
	if res.ExitCode != 0 {
		t.Fatalf("run exit=%d, want 0\n%s", res.ExitCode, res.Combined)
	}
	reqs := broker.messageRequests()
	if len(reqs) == 0 {
		t.Fatal("no /v1/messages request captured")
	}
	body := reqs[0].RawBody
	if !strings.Contains(body, "Megumi Code") {
		t.Errorf("request body sent upstream does not contain the Megumi identity:\n%s", body)
	}
	if strings.Contains(body, "You are Crush") {
		t.Errorf("request body still contains upstream 'You are Crush' branding:\n%s", body)
	}
}

// TestE2E_Broker_StdinPipedPrompt verifies the one-shot path also accepts a
// prompt piped on stdin (Claude Code parity for `... | mgm megumi -p`).
func TestE2E_Broker_StdinPipedPrompt(t *testing.T) {
	requireBrokerCapable(t)

	const reply = "stdin reply received"
	broker := newMockBroker(t, reply)
	_, env := brokerEnv(t, broker)

	res := runWithTimeout(t, 90*time.Second, []byte("summarize this piped text"), env, "megumi", "--api-code", "-p", "summarize")
	if res.TimedOut {
		t.Fatalf("piped one-shot run hung:\n%s", res.Combined)
	}
	if res.ExitCode != 0 {
		t.Fatalf("piped run exit=%d, want 0\n%s", res.ExitCode, res.Combined)
	}
	if !strings.Contains(res.Stdout, reply) {
		t.Errorf("reply missing from stdout: %q", res.Stdout)
	}
	reqs := broker.messageRequests()
	if len(reqs) == 0 {
		t.Fatal("broker received no request for piped prompt")
	}
	if !strings.Contains(reqs[0].UserPromptInBody, "piped text") {
		t.Errorf("piped stdin not included in prompt body; got %q", reqs[0].UserPromptInBody)
	}
}
