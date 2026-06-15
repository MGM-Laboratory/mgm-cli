# Megumi Code — CLI Phase 4 status (Claude Code feature parity)

This phase brought `mgm megumi` to Claude-Code-equivalent behavior by auditing
the embedded Crush fork (`internal/megumi/crush`), rebranding it to Megumi
terminology, wiring the remaining gaps, and explicitly deferring what is not
v1-ready. Phases 1–3 (Crush embedded, branded, broker-wired + auth) are intact
and were not regressed.

## Invariants confirmed (do not regress)

- **All model traffic goes through the broker.** A single locked `megumi`
  Anthropic-type provider points at the broker base URL; `MEGUMI_LOCK=1` strips
  every other provider at config load, and the credential is injected per
  request as `x-api-key` (see `internal/cli/megumi.go`,
  `internal/megumi/crush/agent/megumi_hook.go`,
  `internal/megumi/crush/config/load.go`). No code path reaches a model provider
  directly.
- **All tools run locally** (bash, edit/write/multiedit, ls/grep/glob, fetch,
  MCP, etc.). The backend is a model broker, not a tool sandbox.

## Feature parity matrix

| Feature | Status | Notes / location |
|---|---|---|
| Interactive REPL | ✅ working | Crush TUI via `crushcmd.Run` (`internal/cli/megumi.go`) |
| One-shot / print `-p` | ✅ wired (Phase 4) | `-p` / `--print` rewritten to the embedded `run` subcommand; prompt from args or stdin, streamed to stdout |
| `/model` | ✅ working | Command palette → "Switch Model" (alias `model`). Picker lists the locked provider's models in config order **Meji → Gumi → Miyu** (top→bottom spells MeGuMi) |
| `/clear` | ✅ working | Alias on "New Session" (`clear`, `new`) — starts a fresh conversation, matching Claude Code's `/clear` |
| `/help` | ✅ working | Command palette → "Toggle Help" (alias `help`, `ctrl+g`) |
| Slash command entry | ✅ working | Typing `/` on an empty prompt opens the filterable command palette (also `ctrl+p`); aliases resolve Claude-Code names |
| `/effort` (extended thinking) | ✅ working | Reasoning picker; tiers map to the broker's thinking budget via `x-megumi-effort` |
| Normal mode | ✅ working | Per-tool permission prompts |
| Auto-accept-edits mode | ✅ wired (Phase 4) | Auto-approves `edit`/`write`/`multiedit`; still prompts for bash/downloads |
| Plan mode | ✅ wired (Phase 4) | Read-only: blocks `edit`/`write`/`multiedit`/`bash`/`download` |
| Mode switching | ✅ wired (Phase 4) | `shift+tab` cycles normal → accept-edits → plan; palette entries jump directly; editor placeholder + info report show the active mode |
| `--dangerously-skip-permissions` | ✅ wired (Phase 4) | Mapped onto the embedded yolo mode (`--yolo`), made a persistent flag so `-p` runs honor it |
| `@file` mentions | ✅ working | `@` opens file/MCP-resource completion; selection attaches file content |
| MCP servers | ✅ working | stdio / sse / http transports; tools, prompts, and resources flow into the agent |
| Custom commands | ✅ working + rebranded (Phase 4) | Markdown prompt templates with `$ARG` substitution. Discovery now includes Megumi paths (`~/.mgm/megumi/commands`, project-local `.megumi/commands`) with `.crush` kept as fallback; sources de-duplicated |
| Hooks | ✅ working + rebranded (Phase 4) | `PreToolUse` shell hooks, Claude-Code-envelope compatible. Hook env vars now exported under both `MEGUMI_*` (preferred) and `CRUSH_*` (compat) prefixes |
| Subagents | ✅ working | `agent` tool spawns parallel sub-task sessions with cost roll-up |
| Skills | ✅ working + rebranded (Phase 4) | Project skill discovery now includes `.megumi/skills` (ahead of `.crush/.claude/.cursor`) |
| `MEGUMI.md` memory | ✅ working | Project + global hierarchy (`MEGUMI.md`, `MEGUMI.local.md`), with `CLAUDE.md`/`AGENTS.md` fallback for interop |
| Checkpoint / rewind | ⏸️ **deferred** | See below |

## Editing modes — how enforcement works

Modes live in the permission service (`internal/megumi/crush/permission`) as a
session-wide `Mode` (`normal` / `accept-edits` / `plan`), evaluated in
`Request` before the allowlist so plan mode is authoritative:

- **accept-edits** auto-approves the in-workspace edit tools and prompts for
  everything else.
- **plan** blocks every workspace-mutating tool (edits + `bash` + `download`);
  the tool returns the standard "permission denied" result (`StopTurn`), so the
  agent stops mutating and reports its findings instead of changing anything.
- The global skip ("yolo" / `--dangerously-skip-permissions`) overrides all
  modes.

The mode is threaded through the `Workspace` seam
(`PermissionMode`/`PermissionSetMode`) and is fully wired for the default
in-process path (`AppWorkspace`).

## Deferred (with rationale)

- **Checkpoint / rewind.** Deferred to a later phase. The fork already persists
  **file-version history** (`files` table + `history.Service`) and
  **read tracking** (`read_files` + `filetracker.Service`), but there is no
  conversation+file snapshot/restore. A faithful Claude-Code rewind (snapshot
  the message thread and tracked file versions, then restore both, with TUI
  affordances) is disproportionately large for v1 parity. The existing
  file-version history is the intended foundation for it.
- **Plan-mode model system-prompt.** Plan mode is enforced at the permission
  layer (the hard safety guarantee). A model-facing "you are in plan mode,
  present a plan" instruction would make the agent propose rather than attempt
  and get denied; it needs per-turn prompt plumbing (the system prompt is built
  once per agent, while mode is live) and is a planned refinement.
- **Client/server mode sync.** The client/server architecture is opt-in via
  `CRUSH_CLIENT_SERVER` and Megumi never enables it. Editing-mode state on the
  `ClientWorkspace` is therefore kept client-side only; server-side enforcement
  for that path is not wired. The default in-process path is fully wired.

## Quick reference

```
mgm megumi                                   # interactive TUI
mgm megumi -p "explain internal/cli"         # one-shot, prints to stdout
cat err.log | mgm megumi -p "diagnose this"  # one-shot with piped stdin
mgm megumi --dangerously-skip-permissions    # skip all permission prompts
mgm megumi --account                         # force mgm-account auth
mgm megumi --api-code                         # force Megumi API code auth
```

In-session: `shift+tab` cycles editing mode; `/` (or `ctrl+p`) opens the
command palette; type `model`, `clear`, `help`, `plan`, `accept edits`, etc.

## CI coverage (`.github/workflows/ci.yml`)

The CI exercises the program across platforms, distros, and run modes:

- **lint** — blocking gofmt on the packages we own (the upstream fork is
  advisory), `go vet ./...`, vet of the e2e-tagged tests, and a `go mod tidy`
  cleanliness check.
- **unit** — `go test` on Ubuntu, macOS, and Windows (race detector on
  Linux/macOS), with coverage uploaded.
- **build-matrix** — cross-compiles every shipped target
  (linux/darwin/windows × amd64/arm64, minus windows/arm64) and uploads each
  binary.
- **e2e** — the binary-driven suite under `test/e2e` (build tag `e2e`) on all
  three OSes: smoke commands everywhere; the mock-broker one-shot round-trip and
  the interactive PTY startup/quit test on Linux/macOS.
- **distro-smoke** — runs the static (CGO-free) linux/amd64 binary inside
  Ubuntu 22.04/24.04, Debian 11/12, Arch, Alpine (musl), and Fedora containers.
- **package-build / package-install** — builds deb/rpm/apk/Arch packages with a
  GoReleaser snapshot, then installs and runs each in its matching distro.
- **install-scripts** — syntax-checks `install.sh` (bash) and `install.ps1`
  (PowerShell parser).
- **govulncheck** — advisory vulnerability scan.
- **ci-success** — single aggregate gate suitable for branch protection.

### `test/e2e` design

The e2e tests build a real `mgm` binary and drive it as a subprocess. A mock
broker (`httptest`) emulates the backend: `/api/v1/me` for the API-code
handshake and an Anthropic-compatible `/v1/messages` SSE stream. A test seeds a
Megumi API code into the file credential store (same `MEGUMI_HOME`/host/uid, so
the subprocess decrypts it), points `MEGUMI_BASE_URL` at the mock, and asserts:

- the model reply reaches stdout for `mgm megumi -p` (and via piped stdin);
- the broker request shape is correct — `x-api-key` carries the injected
  credential, `Authorization` is stripped, `x-megumi-effort`/`-project` and
  `anthropic-version` are set, and the user prompt is in the body;
- non-interactive `--api-code` with no stored credential fails fast and cleanly
  (no hang, no panic);
- the interactive TUI starts (branded output under a PTY) and quits cleanly.

Run locally with: `go test -tags e2e ./test/e2e/...`
