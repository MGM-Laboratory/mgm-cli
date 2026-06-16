# mgm

`mgm` is the MGM internal CLI — a single command for everyday MGM ops. It started with secrets and service health and grows from there.

```
mgm env       Pull/push secrets from self-hosted Infisical
mgm status    Check the health of MGM services (Gatus)
```

More namespaces (`mgm db`, `mgm deploy`, `mgm cluster`, ...) are planned. Each one is its own subcommand tree, configured per-profile in `~/.mgm/config`.

---

## Install

One command. Pick the line for your OS — that's it.

### Linux / macOS / WSL / Git Bash

```sh
curl -fsSL https://cli.labmgm.org/install.sh | bash
```

### Windows (PowerShell)

```powershell
irm https://cli.labmgm.org/install.ps1 | iex
```

That installs the latest release into `/usr/local/bin` (Linux/macOS) or `%LOCALAPPDATA%\Programs\mgm` (Windows), updates your `PATH` if needed, and prints the version. Re-run the same command to upgrade. Then `mgm auth` to sign in and `mgm megumi` to start Megumi Code.

### Package managers

```sh
# Homebrew (macOS/Linux)
brew install --cask MGM-Laboratory/mgm/mgm

# Scoop (Windows)
scoop bucket add mgm https://github.com/MGM-Laboratory/scoop-mgm
scoop install mgm

# Debian / Ubuntu
sudo apt install ./mgm_*_linux_amd64.deb        # downloaded from the releases page

# Fedora / RHEL
sudo dnf install ./mgm_*_linux_amd64.rpm

# Alpine
sudo apk add --allow-untrusted ./mgm_*_linux_amd64.apk

# Arch (AUR)
yay -S mgm-bin

# Nix (NUR)
nix-env -iA nur.repos.mgm-laboratory.mgm

# Docker
docker run --rm mgmlaboratory/mgm version
```

<details>
<summary>Pin a version, custom install dir, manual download, source build</summary>

#### Pin a version

```sh
# Linux/macOS
curl -fsSL https://cli.labmgm.org/install.sh | MGM_VERSION=v0.1.0 bash
```

```powershell
# Windows
$env:MGM_VERSION='v0.1.0'; irm https://cli.labmgm.org/install.ps1 | iex
```

#### Custom install location (no sudo)

```sh
curl -fsSL https://cli.labmgm.org/install.sh | MGM_INSTALL_DIR=$HOME/.local/bin bash
```

```powershell
$env:MGM_INSTALL_DIR="$HOME\bin"; irm https://cli.labmgm.org/install.ps1 | iex
```

#### Manual download

Grab the archive for your platform from the [releases page](https://github.com/MGM-Laboratory/mgm-cli/releases):

| OS      | Arch  | Asset                          |
| ------- | ----- | ------------------------------ |
| Linux   | amd64 | `mgm-linux-amd64.tar.gz`       |
| Linux   | arm64 | `mgm-linux-arm64.tar.gz`       |
| macOS   | amd64 | `mgm-darwin-amd64.tar.gz`      |
| macOS   | arm64 | `mgm-darwin-arm64.tar.gz`      |
| Windows | amd64 | `mgm-windows-amd64.zip`        |

Extract `mgm` (or `mgm.exe`) onto your `PATH`.

#### Arch Linux (AUR)

```sh
cd packaging/arch
makepkg -si
```

A `.deb`, `.rpm`, `.apk`, and Arch package are also produced by `goreleaser` for each release.

#### From source

```sh
git clone https://github.com/MGM-Laboratory/mgm-cli
cd mgm-cli
make build               # ./bin/mgm
sudo make install        # /usr/local/bin/mgm
```

Cross-build everything locally:

```sh
make dist                # ./dist/mgm-{os}-{arch}
```

</details>

---

## Configuration

Settings live in `~/.mgm/config` (TOML), with one section per profile. The first time a command needs something it isn't configured for, it'll prompt you and save the answer.

```toml
[default]
# env / Infisical
host_url      = "https://secrets.labmgm.org"
client_id     = "..."
client_secret = "..."

# status / Gatus
gatus_url     = "https://status.labmgm.org"
gatus_token   = ""

[ops]
host_url      = "https://secrets.labmgm.org"
client_id     = "..."
client_secret = "..."
```

Switch profiles with `--profile ops` (or `MGM_PROFILE=ops`).

<details>
<summary>All environment variables and overrides</summary>

| Place                    | Purpose                                       |
| ------------------------ | --------------------------------------------- |
| `~/.mgm/config`          | Credentials and per-profile defaults (TOML)   |
| `./.mgm.yaml`            | Project pin (project_id, environment, folder) |
| `MGM_PROFILE`            | Active profile name                           |
| `MGM_CONFIG`             | Override config file path                     |
| `MGM_HOST_URL`           | Infisical host URL                            |
| `MGM_CLIENT_ID`          | Infisical client ID                           |
| `MGM_CLIENT_SECRET`      | Infisical client secret                       |
| `MGM_PROJECT_ID`         | Default Infisical project ID                  |
| `MGM_ENVIRONMENT`        | Default Infisical environment slug            |
| `MGM_FOLDER`             | Default Infisical secret folder               |
| `MGM_GATUS_URL`          | Gatus base URL                                |
| `MGM_GATUS_TOKEN`        | Gatus bearer token (optional)                 |
| `MGM_NO_TUI=1`           | Disable interactive prompts (CI)              |
| `MEGUMI_OIDC_ISSUER`     | Keycloak realm issuer URL (Megumi auth)       |
| `MEGUMI_OIDC_CLIENT_ID`  | Public CLI client id                          |
| `MEGUMI_OIDC_SCOPES`     | OIDC scopes (default includes `offline_access`) |
| `MEGUMI_BASE_URL`        | Backend broker base URL (`whoami` enrichment) |
| `MEGUMI_ROLE_ADMIN`      | Realm-role/group name mapped to `admin`       |
| `MEGUMI_ROLE_OPERATOR`   | Realm-role/group name mapped to `operator`    |
| `MEGUMI_CRED_STORE`      | Credential backend: `auto` / `keychain` / `file` |
| `MEGUMI_HOME`            | Override `~/.mgm/megumi` (mainly for tests)   |

Resolution order: CLI flags > `.mgm.yaml` > profile in config > env vars > built-in defaults. Megumi auth values are env-only (no profile section); see `.env.example`.

</details>

---

## Namespaces

<details open>
<summary><b><code>mgm megumi</code></b> — Megumi Code, the lab's AI coding agent</summary>

`mgm megumi` starts **Megumi Code**, a forked, rebranded [Charm Crush](https://github.com/charmbracelet/crush)
embedded into this single binary and pointed exclusively at the lab broker. Every
model request is brokered through the backend (which holds the Claude
credentials); the agent loop and all tools run **locally** on your machine.

```sh
mgm megumi                 # start an interactive session (asks how to authenticate)
mgm megumi --account       # use your mgm account (skip the prompt)
mgm megumi --api-code      # use a Megumi API code (skip the prompt)
mgm megumi run "…"         # non-interactive; extra args/flags pass through to the agent
```

- **Auth** reuses your `mgm auth` session. Choosing *mgm account* refreshes the
  saved tokens transparently (no re-login); choosing *Megumi API code* prompts for
  a code, verifies it, and saves it to the shared store under `~/.mgm/megumi`.
- **Models** are the Megumi labels **Meji** (Haiku), **Gumi** (Sonnet),
  **Miyu** (Opus). The broker maps each to an upstream model.
- **Memory** is `MEGUMI.md` — project-level (`./MEGUMI.md`, `./MEGUMI.local.md`)
  and user-level (`~/.mgm/megumi/MEGUMI.md`), hierarchical like Claude Code's
  `CLAUDE.md` (which is still read for interop, along with `AGENTS.md`).
- **State** (sessions, history, config) lives entirely under `~/.mgm/megumi`.
- Only the Megumi broker is reachable — no other model provider is available, and
  telemetry/auto-update are disabled.

Configure via environment (see `.env.example`): `MEGUMI_BASE_URL` (broker),
`MEGUMI_EFFORT` (`low|medium|high`). Auth env (`MEGUMI_OIDC_*`) is shared with
`mgm auth`.

</details>

<details open>
<summary><b><code>mgm auth</code> / <code>mgm whoami</code></b> — Megumi Code mgm-account login</summary>

Sign in to **Megumi Code** with your mgm account (Keycloak at `iam.labmgm.org`).
Credentials are stored under `~/.mgm/megumi` — in the OS keychain when available
(macOS Keychain / Windows Credential Manager / libsecret), otherwise a `0600`
AES-256-GCM encrypted file — and are shared with the `mgm megumi` agent.

```sh
mgm auth                   # sign in (opens a browser for Authorization Code + PKCE)
mgm auth --no-browser      # no browser: show a URL + code to enter elsewhere (device flow)
mgm auth --device          # force the device-code flow
mgm auth status            # are you signed in? (refreshes an expired token)
mgm auth logout            # revoke + clear stored credentials
mgm whoami                 # show the active Megumi identity (subject/email/role/method)
mgm whoami --json          # same, as JSON
```

Login tries the browser PKCE flow first and automatically falls back to the
in-terminal device-code flow when no browser can be opened. Access tokens are
refreshed transparently. Your **role** (`member` / `operator` / `admin`) comes
from Keycloak realm roles or groups; `mgm whoami` confirms it against the backend
when reachable and otherwise shows the locally-decoded value.

The Megumi identity is distinct from the Infisical identity — for the latter use
`mgm env whoami`.

Configure via environment (see `.env.example`): `MEGUMI_OIDC_ISSUER`,
`MEGUMI_OIDC_CLIENT_ID`, `MEGUMI_OIDC_SCOPES`, `MEGUMI_BASE_URL`,
`MEGUMI_ROLE_ADMIN`, `MEGUMI_ROLE_OPERATOR`. The Keycloak realm, CLI client
(`mgm-cli`), redirect URIs, and roles are described in the backend runbook at
`mgm-cli-backend/docs/keycloak.md`.

</details>

<details>
<summary><b><code>mgm env</code></b> — Secrets & .env files via Infisical</summary>

Pull, push, view, and edit secrets stored in self-hosted Infisical at `https://secrets.labmgm.org`.

```sh
mgm env configure          # one-time credential setup (prompts if you skip it)
mgm env init               # pin this directory to a project/env/folder (writes .mgm.yaml)
mgm env list               # browse projects → folders → secrets
mgm env pull               # write secrets to .env
mgm env push               # push .env to Infisical
mgm env diff               # compare .env vs Infisical (CI-friendly: exits 1 on drift)
mgm env get FOO            # one secret
mgm env set FOO=bar BAZ=qux
mgm env delete FOO
mgm env run -- ./app       # exec a process with secrets injected
mgm env export --format json
mgm env projects | environments | folders
mgm env status | whoami
```

#### First-time setup

```sh
$ mgm env configure
Configure Infisical credentials (profile: default)
Credentials will be saved to /home/you/.mgm/config [default]

Infisical Client ID [None]: ...
Infisical Client Secret [None]: ...
Infisical Host URL [https://secrets.labmgm.org]:

Saved /home/you/.mgm/config [default]
```

Any command that needs credentials will trigger this prompt if you skip it.

#### Pin a working directory

```sh
$ cd ~/code/my-service
$ mgm env init
… interactive picker for project / env / folder …
Wrote .mgm.yaml
```

Commit `.mgm.yaml`. From then on `mgm env pull` etc. skip the picker.

```yaml
# .mgm.yaml
project_id: 7f9e...
environment: dev
folder: /backend
```

#### Common workflows

```sh
# Pull dev secrets into .env
mgm env pull

# Pull prod into a custom file
mgm env pull --env prod --file .env.production

# Push current .env to staging, no prompt
mgm env push --env stg --yes

# Push and remove remote keys missing locally (careful)
mgm env push --env dev --delete-orphans --yes

# Inspect what would change without applying
mgm env push --env prod --dry-run

# Run an app with prod secrets injected
mgm env run --env prod -- ./bin/server

# Pipe secrets into another tool
mgm env export --env prod --format json | jq '.DATABASE_URL'
eval "$(mgm env export --env dev --format shell)"

# Compare local .env to remote (exit 1 if drift) — useful in CI
mgm env diff --env prod
```

</details>

<details>
<summary><b><code>mgm status</code></b> — MGM service health (Gatus)</summary>

Reads the MGM Gatus instance at `https://status.labmgm.org` and reports current health, recent checks, and uptime per service.

```sh
mgm status                  # table of every service + current up/down
mgm status SERVICE          # detail view: latest checks, conditions, uptime windows
mgm status list             # every service Gatus knows about (key/name/group/host)
mgm status configure        # set Gatus URL (and optional bearer token) for the profile
mgm status incidents        # show services currently failing (exits 1 if any are down)
mgm status uptime SERVICE   # uptime ratio over a window (--window 1h|24h|7d|30d)
mgm status open [SERVICE]   # open the dashboard, or a specific service, in the browser
mgm status --watch 10s      # repaint every 10 seconds
```

`SERVICE` matches against (in order): exact Gatus key, exact name, `group/name`, then a unique substring. If the match is ambiguous you'll get a picker.

#### First-time setup

The first command that needs Gatus will prompt for the URL (default `https://status.labmgm.org`) and an optional token, then save them under your active profile. To do it explicitly:

```sh
mgm status configure --url https://status.labmgm.org --test
```

#### Examples

```sh
# Quick glance — what's up right now?
mgm status

# Drill into one service
mgm status api
mgm status core/api --json | jq '.results[-1]'

# Watch the dashboard from a terminal during an incident
mgm status --watch 5s

# CI gate that fails the build when production isn't healthy
mgm status incidents

# Daily uptime for a single service
mgm status uptime database --window 24h
```

</details>

---

## Shell completion

```sh
mgm completion bash       > /etc/bash_completion.d/mgm
mgm completion zsh        > "${fpath[1]}/_mgm"
mgm completion fish       > ~/.config/fish/completions/mgm.fish
mgm completion powershell | Out-String | Invoke-Expression
```

---

## Development

```sh
make test
make build
make snapshot           # local goreleaser dry-run
```

Tag a release:

```sh
git tag v0.1.0 && git push --tags
```

GitHub Actions builds and publishes archives + Linux packages automatically.

## Attribution

`mgm megumi` embeds a vendored, modified fork of **Charm Crush**
(<https://github.com/charmbracelet/crush>) under
`internal/megumi/crush/`, licensed under the **Functional Source License
(FSL-1.1-MIT)**. The upstream `LICENSE.md` is preserved verbatim there, alongside
a `NOTICE` recording the upstream commit and the modifications made. Megumi Code
is internal MGM Laboratory tooling.
