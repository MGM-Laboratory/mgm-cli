package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/auth"
	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/credstore"
	"github.com/MGM-Laboratory/mgm-cli/internal/ui"
)

func TestBuildLockedConfig(t *testing.T) {
	cfg := buildLockedConfig("https://broker.example/", "high", "myproj")

	opts := cfg["options"].(map[string]any)
	if opts["disable_default_providers"] != true {
		t.Error("default providers not disabled")
	}
	if opts["disable_metrics"] != true {
		t.Error("metrics not disabled")
	}
	if cp := opts["context_paths"].([]string); cp[0] != "MEGUMI.md" {
		t.Errorf("context_paths[0] = %q, want MEGUMI.md", cp[0])
	}

	prov := cfg["providers"].(map[string]any)[megumiProviderID].(map[string]any)
	if prov["type"] != "anthropic" {
		t.Errorf("provider type = %v, want anthropic", prov["type"])
	}
	if prov["base_url"] != "https://broker.example/" {
		t.Errorf("base_url = %v", prov["base_url"])
	}
	hdr := prov["extra_headers"].(map[string]any)
	if hdr["x-megumi-effort"] != "high" || hdr["x-megumi-project"] != "myproj" {
		t.Errorf("headers wrong: %v", hdr)
	}
	models := prov["models"].([]map[string]any)
	if _, ok := models[0]["reasoning_levels"]; !ok {
		t.Error("models should carry reasoning_levels so the /effort picker appears")
	}
	gotIDs := []string{models[0]["id"].(string), models[1]["id"].(string), models[2]["id"].(string)}
	want := []string{labelMeji, labelGumi, labelMiyu}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Errorf("model[%d] id = %q, want %q", i, gotIDs[i], want[i])
		}
	}
	sel := cfg["models"].(map[string]any)
	if sel["large"].(map[string]any)["model"] != labelGumi {
		t.Error("large model should be gumi")
	}
	if sel["small"].(map[string]any)["model"] != labelMeji {
		t.Error("small model should be meji")
	}
}

func TestMegumiIdentityPrefix(t *testing.T) {
	p := megumiIdentityPrefix("https://example.test")
	// Identity facts the agent must carry (Megumi Code self-awareness).
	for _, want := range []string{
		"Megumi Code",
		"MGM Laboratory",
		"Universitas Brawijaya",
		"Muhammad Idham Ma'arif",
		"Syafa Hadyan Rasendriya",
		"https://example.test",
		"Meji", "Gumi", "Miyu",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("identity prefix missing %q\n---\n%s", want, p)
		}
	}
	// Must instruct against revealing the underlying provider.
	if !strings.Contains(p, "Claude") {
		t.Error("identity prefix should explicitly tell the model not to identify as Claude")
	}
}

func TestBuildLockedConfigCarriesIdentityPrefix(t *testing.T) {
	cfg := buildLockedConfig("https://broker.example/", "medium", "p")
	prov := cfg["providers"].(map[string]any)[megumiProviderID].(map[string]any)
	prefix, ok := prov["system_prompt_prefix"].(string)
	if !ok || prefix == "" {
		t.Fatal("locked provider config is missing a system_prompt_prefix")
	}
	if !strings.Contains(prefix, "Megumi Code") || !strings.Contains(prefix, "MGM Laboratory") {
		t.Errorf("system_prompt_prefix missing identity facts:\n%s", prefix)
	}
}

func TestMegumiSiteURLEnvOverride(t *testing.T) {
	t.Setenv("MEGUMI_SITE_URL", "https://custom.site")
	if got := megumiSiteURL(); got != "https://custom.site" {
		t.Errorf("site = %q, want the env override", got)
	}
	t.Setenv("MEGUMI_SITE_URL", "")
	if got := megumiSiteURL(); got != defaultSiteURL {
		t.Errorf("site = %q, want default %q", got, defaultSiteURL)
	}
}

func TestWriteLockedConfigNoSecretAndValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crush.json")
	cfg := buildLockedConfig("https://broker.example", "medium", "p")
	if err := writeLockedConfig(path, cfg); err != nil {
		t.Fatalf("writeLockedConfig: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// The real credential is injected at runtime via the hook — no token may be
	// written to disk. A non-secret placeholder api_key is allowed.
	for _, bad := range []string{"Bearer ", "mgm_"} {
		if bytes.Contains(raw, []byte(bad)) {
			t.Errorf("config file unexpectedly contains secret-like %q", bad)
		}
	}
	if !bytes.Contains(raw, []byte(apiKeyPlaceholder)) {
		t.Errorf("config should carry the non-secret api_key placeholder %q", apiKeyPlaceholder)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("written config is not valid JSON: %v", err)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o644 {
			t.Errorf("perms = %o, want 644", info.Mode().Perm())
		}
	}
}

func TestSplitMegumiArgs(t *testing.T) {
	cases := []struct {
		in       []string
		wantFwd  []string
		wantPref methodPref
	}{
		{[]string{"--account", "run", "hello"}, []string{"run", "hello"}, prefAccount},
		{[]string{"--api-code"}, []string{}, prefCode},
		{[]string{"--code", "-y"}, []string{"-y"}, prefCode},
		{[]string{"run", "hi"}, []string{"run", "hi"}, prefAuto},
		// -p / --print map onto the embedded `run` one-shot subcommand.
		{[]string{"-p", "hello world"}, []string{"run", "hello world"}, prefAuto},
		{[]string{"--print"}, []string{"run"}, prefAuto},
		// --dangerously-skip-permissions maps onto --yolo.
		{[]string{"--dangerously-skip-permissions"}, []string{"--yolo"}, prefAuto},
		// Combined: print + skip-permissions + auth selection.
		{[]string{"--account", "-p", "do x", "--dangerously-skip-permissions"}, []string{"run", "do x", "--yolo"}, prefAccount},
		// Idempotent: an explicit `run` plus -p must not double the subcommand.
		{[]string{"run", "-p", "x"}, []string{"run", "x"}, prefAuto},
	}
	for _, tc := range cases {
		fwd, pref := splitMegumiArgs(tc.in)
		if pref != tc.wantPref {
			t.Errorf("%v: pref = %d, want %d", tc.in, pref, tc.wantPref)
		}
		if strings.Join(fwd, ",") != strings.Join(tc.wantFwd, ",") {
			t.Errorf("%v: forward = %v, want %v", tc.in, fwd, tc.wantFwd)
		}
	}
}

func TestEffortFromEnv(t *testing.T) {
	for in, want := range map[string]string{"low": "low", "high": "high", "MEDIUM": "medium", "": "medium", "bogus": "medium"} {
		t.Setenv("MEGUMI_EFFORT", in)
		if got := effortFromEnv(); got != want {
			t.Errorf("MEGUMI_EFFORT=%q -> %q, want %q", in, got, want)
		}
	}
}

func TestResolveCredentialReusesStoredAPICode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MEGUMI_HOME", dir)
	t.Setenv("MEGUMI_CRED_STORE", "file")

	// Silence handshake output.
	var buf bytes.Buffer
	oldOut, oldErr := ui.Out, ui.Err
	ui.Out, ui.Err = &buf, &buf
	t.Cleanup(func() { ui.Out, ui.Err = oldOut, oldErr })

	a, err := auth.New()
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	if err := a.Store.Save(&credstore.Credentials{
		Method: credstore.MethodAPICode, APICode: "mgm_abc_secret", Email: "u@x.io",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	credFn, err := resolveCredential(t.Context(), a, prefCode)
	if err != nil {
		t.Fatalf("resolveCredential: %v", err)
	}
	got, err := credFn()
	if err != nil {
		t.Fatalf("credFn: %v", err)
	}
	if got != "mgm_abc_secret" {
		t.Fatalf("credential = %q, want the stored API code", got)
	}
}
