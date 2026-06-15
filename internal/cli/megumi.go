package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	crushagent "github.com/MGM-Laboratory/mgm-cli/internal/megumi/crush/agent"
	crushcmd "github.com/MGM-Laboratory/mgm-cli/internal/megumi/crush/cmd"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/auth"
	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/banner"
	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/credstore"
	"github.com/MGM-Laboratory/mgm-cli/internal/ui"
)

// methodPref selects which auth method `mgm megumi` uses.
type methodPref int

const (
	prefAuto methodPref = iota
	prefAccount
	prefCode
)

// newMegumiCommand builds `mgm megumi` — the embedded, broker-locked agent
// (a forked Charm Crush). Flag parsing is disabled so that everything after
// `megumi` (prompts, --continue, -y, etc.) is forwarded to the embedded agent;
// only `--account` / `--api-code` are intercepted to pick the auth method.
func newMegumiCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "megumi",
		Short: "Start Megumi Code — the lab's AI coding agent",
		Long: "Start Megumi Code, the lab's AI coding agent. Every model request is\n" +
			"brokered through the Megumi backend; tools run locally on your machine.\n\n" +
			"Auth reuses your `mgm auth` session. Pass --account to use your mgm\n" +
			"account or --api-code to use a Megumi API code (otherwise you're asked).\n\n" +
			"Claude-Code-compatible flags are accepted:\n" +
			"  -p, --print \"...\"                one-shot: run a single prompt and exit\n" +
			"  --dangerously-skip-permissions  skip all permission prompts\n\n" +
			"All agent state and MEGUMI.md memory live under ~/.mgm/megumi.",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			forward, pref := splitMegumiArgs(args)
			return runMegumi(cmd, forward, pref)
		},
	}
}

// splitMegumiArgs extracts the auth-method flags, translates the
// Claude-Code-compatible flags Megumi advertises into their embedded-agent
// equivalents, and returns the remaining args to forward to the embedded agent.
//
// Translations (so `mgm megumi` matches Claude Code's surface):
//   - -p / --print                  → the `run` one-shot subcommand
//   - --dangerously-skip-permissions → --yolo (the embedded skip-permissions mode)
func splitMegumiArgs(args []string) ([]string, methodPref) {
	pref := prefAuto
	printMode := false
	forward := make([]string, 0, len(args))
	for _, a := range args {
		switch a {
		case "--account":
			pref = prefAccount
		case "--api-code", "--code":
			pref = prefCode
		case "-p", "--print":
			printMode = true
		case "--dangerously-skip-permissions":
			forward = append(forward, "--yolo")
		default:
			forward = append(forward, a)
		}
	}
	// -p/--print is Claude Code's one-shot/print mode; the embedded agent
	// exposes the same behavior through its `run` subcommand (prompt from args
	// or stdin, streamed to stdout). Prepend it unless the caller already asked
	// for `run` explicitly.
	if printMode && !startsWithRun(forward) {
		forward = append([]string{"run"}, forward...)
	}
	return forward, pref
}

// startsWithRun reports whether the first non-flag token selects the embedded
// `run` (one-shot) subcommand.
func startsWithRun(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a == "run" || a == "r"
	}
	return false
}

func runMegumi(cmd *cobra.Command, forward []string, pref methodPref) error {
	ctx := cmd.Context()

	// Help/version requests forward straight to the embedded agent and never
	// require authentication.
	if isHelpOrVersion(forward) {
		return crushcmd.Run(ctx, forward)
	}

	// Branded welcome: the color-cycling MEGUMI banner before the auth choice,
	// for interactive sessions only (skipped for one-shot `run` / non-TTY).
	if ui.IsInteractive() && startsInteractiveSession(forward) {
		banner.Show(ui.Out)
	}

	a, err := auth.New()
	if err != nil {
		return err
	}
	credFn, err := resolveCredential(ctx, a, pref)
	if err != nil {
		return err
	}
	// Register the per-request credential hook the embedded agent calls, and seed
	// the initial effort tier (the /effort picker updates it live in-session).
	crushagent.CredentialProvider = credFn
	crushagent.SetEffort(effortFromEnv())

	home, err := megumiHome()
	if err != nil {
		return err
	}

	// Relocate all embedded-agent state under ~/.mgm/megumi and lock it down.
	// CRUSH_GLOBAL_CONFIG holds our locked base config; CRUSH_GLOBAL_DATA is a
	// SEPARATE path where the agent persists runtime changes (e.g. model picks)
	// so it never clobbers the locked base.
	setenv("CRUSH_GLOBAL_CONFIG", home)
	setenv("CRUSH_GLOBAL_DATA", filepath.Join(home, "data"))
	setenv("CRUSH_CACHE_DIR", filepath.Join(home, "cache"))
	setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	setenv("DO_NOT_TRACK", "1")
	setenv("MEGUMI_LOCK", "1")

	cfg := buildLockedConfig(a.Cfg.BaseURL, effortFromEnv(), projectName())
	if err := writeLockedConfig(filepath.Join(home, "crush.json"), cfg); err != nil {
		return fmt.Errorf("write Megumi config: %w", err)
	}

	// Use a single global session store/DB under ~/.mgm/megumi.
	launchArgs := append([]string{"--data-dir", home}, forward...)
	return crushcmd.Run(ctx, launchArgs)
}

// resolveCredential runs the auth handshake and returns a function that yields
// the live broker credential (sent as x-api-key). For the account method the
// function refreshes+persists the access token on each call; for an API code it
// returns the stored code.
func resolveCredential(ctx context.Context, a *auth.Authenticator, pref methodPref) (func() (string, error), error) {
	existing, _ := a.Store.Load() // nil when none stored

	if pref == prefAuto {
		pref = chooseMethod(existing)
	}

	switch pref {
	case prefAccount:
		creds, err := a.Current(ctx, true)
		if errors.Is(err, credstore.ErrNotFound) {
			creds, err = accountLogin(ctx, a)
		}
		if err != nil {
			return nil, err
		}
		ts, err := a.TokenSource(ctx, creds)
		if err != nil {
			return nil, err
		}
		ui.Successf("Using mgm account: %s", identityLabel(creds))
		return func() (string, error) {
			tok, err := ts.Token()
			if err != nil {
				return "", err
			}
			return tok.AccessToken, nil
		}, nil

	case prefCode:
		code, err := resolveAPICode(ctx, a, existing)
		if err != nil {
			return nil, err
		}
		return func() (string, error) { return code, nil }, nil

	default:
		return nil, errors.New("no authentication method selected")
	}
}

// chooseMethod asks how to authenticate. Non-interactively it reuses a stored
// method, or errors directing the user to a flag.
func chooseMethod(existing *credstore.Credentials) methodPref {
	if !ui.IsInteractive() {
		if existing != nil && existing.Method == credstore.MethodAPICode {
			return prefCode
		}
		return prefAccount // account creds (or a fresh login) — Current handles missing
	}
	choice, err := ui.SelectOne("How do you want to authenticate Megumi Code?", []ui.Choice{
		{Label: "Use mgm account", Value: "account", Hint: "Keycloak SSO via mgm auth"},
		{Label: "Use a Megumi API code", Value: "code", Hint: "paste an issued code"},
	})
	if err != nil {
		return prefAccount
	}
	if choice == "code" {
		return prefCode
	}
	return prefAccount
}

// accountLogin runs an interactive mgm-account login (same flows as `mgm auth`).
func accountLogin(ctx context.Context, a *auth.Authenticator) (*credstore.Credentials, error) {
	ui.Title("Sign in to Megumi Code")
	return a.Login(ctx, auth.LoginOptions{Notifier: auth.Notifier{
		OnBrowser: func(url string) {
			ui.Infof("Opening your browser to sign in…")
			ui.KV("url", url)
		},
		OnDeviceCode: func(d auth.DeviceInstructions) {
			ui.Infof("To sign in, open this URL and enter the code:")
			ui.KV("url", d.VerificationURI)
			ui.Infof("code: %s", ui.Key(d.UserCode))
		},
	}})
}

// resolveAPICode reuses a stored code or prompts for one, validating it against
// the broker and persisting it to the shared store.
func resolveAPICode(ctx context.Context, a *auth.Authenticator, existing *credstore.Credentials) (string, error) {
	if existing != nil && existing.Method == credstore.MethodAPICode && existing.APICode != "" {
		ui.Successf("Using saved Megumi API code%s", labelSuffix(existing))
		return existing.APICode, nil
	}
	if !ui.IsInteractive() {
		return "", errors.New("no Megumi API code stored; run `mgm megumi` interactively to enter one")
	}
	code, err := ui.PromptSecret("Megumi API code", "")
	if err != nil {
		return "", err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", errors.New("no API code entered")
	}

	rec := &credstore.Credentials{Method: credstore.MethodAPICode, APICode: code, Issuer: a.Cfg.Issuer}
	if bi, verr := auth.FetchBackendIdentity(ctx, a.Cfg.BaseURL, code); verr != nil {
		ui.Warnf("could not verify the code now (%v); it will be checked on first request", verr)
	} else {
		rec.Subject, rec.Email, rec.Role = bi.Subject, bi.Email, bi.Role
		ui.Successf("Verified Megumi API code%s", labelSuffix(rec))
	}
	if err := a.Store.Save(rec); err != nil {
		return "", fmt.Errorf("save API code: %w", err)
	}
	return code, nil
}

func labelSuffix(c *credstore.Credentials) string {
	if l := identityLabel(c); l != "(unknown)" {
		return " (" + l + ")"
	}
	return ""
}

// effortFromEnv resolves the extended-thinking effort tier sent to the broker.
func effortFromEnv() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MEGUMI_EFFORT"))) {
	case "low":
		return "low"
	case "high":
		return "high"
	default:
		return "medium"
	}
}

// projectName tags usage with the current project (the working-dir base name).
func projectName() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "default"
	}
	if b := filepath.Base(cwd); b != "" && b != "." && b != string(filepath.Separator) {
		return b
	}
	return "default"
}

// megumiHome is ~/.mgm/megumi (overridable via MEGUMI_HOME, matching credstore).
func megumiHome() (string, error) {
	if v := strings.TrimSpace(os.Getenv("MEGUMI_HOME")); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".mgm", "megumi"), nil
}

// startsInteractiveSession reports whether the forwarded args launch the
// interactive TUI (vs a one-shot like `run`/`-p`), so the welcome banner only
// shows for real sessions. By the time this runs, splitMegumiArgs has already
// rewritten -p/--print into the `run` subcommand.
func startsInteractiveSession(args []string) bool {
	return !startsWithRun(args)
}

// isHelpOrVersion reports whether the forwarded args are a help/version request
// (which must not trigger the auth handshake).
func isHelpOrVersion(args []string) bool {
	for _, a := range args {
		switch a {
		case "-h", "--help", "help", "-v", "--version", "version":
			return true
		}
	}
	return false
}

func setenv(k, v string) { _ = os.Setenv(k, v) }
