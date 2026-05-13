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
curl -fsSL https://raw.githubusercontent.com/MGM-Laboratory/mgm-cli/main/install.sh | bash
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/MGM-Laboratory/mgm-cli/main/install.ps1 | iex
```

That installs the latest release into `/usr/local/bin` (Linux/macOS) or `%LOCALAPPDATA%\Programs\mgm` (Windows), updates your `PATH` if needed, and prints the version. Re-run the same command to upgrade.

<details>
<summary>Pin a version, custom install dir, manual download, source build</summary>

#### Pin a version

```sh
# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/MGM-Laboratory/mgm-cli/main/install.sh | MGM_VERSION=v0.1.0 bash
```

```powershell
# Windows
$env:MGM_VERSION='v0.1.0'; irm https://raw.githubusercontent.com/MGM-Laboratory/mgm-cli/main/install.ps1 | iex
```

#### Custom install location (no sudo)

```sh
curl -fsSL https://raw.githubusercontent.com/MGM-Laboratory/mgm-cli/main/install.sh | MGM_INSTALL_DIR=$HOME/.local/bin bash
```

```powershell
$env:MGM_INSTALL_DIR="$HOME\bin"; irm https://raw.githubusercontent.com/MGM-Laboratory/mgm-cli/main/install.ps1 | iex
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

Resolution order: CLI flags > `.mgm.yaml` > profile in config > env vars > built-in defaults.

</details>

---

## Namespaces

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
