# mgm

MGM internal CLI. Today it manages secrets and environments stored in our self-hosted Infisical at https://secrets.labmgm.org. The `env` namespace is the first feature; future areas (`mgm db`, `mgm deploy`, `mgm cluster`, ...) live alongside it.

```
mgm env configure          # one-time credential setup
mgm env init               # pin this directory to a project/env/folder
mgm env list               # browse projects → folders → secrets
mgm env pull               # write secrets to .env
mgm env push               # push .env to Infisical
mgm env diff               # compare .env vs Infisical
mgm env get FOO            # one secret
mgm env set FOO=bar BAZ=qux
mgm env delete FOO
mgm env run -- ./app       # exec with secrets injected
mgm env export --format json
mgm env projects | environments | folders
mgm env status | whoami
```

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

That installs the latest release into `/usr/local/bin` (Linux/macOS) or `%LOCALAPPDATA%\Programs\mgm` (Windows), updates your `PATH` if needed, and shows you the version. Re-run the same command to upgrade.

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

### Manual download

Grab the archive for your platform from the [releases page](https://github.com/MGM-Laboratory/mgm-cli/releases):

| OS      | Arch  | Asset                          |
| ------- | ----- | ------------------------------ |
| Linux   | amd64 | `mgm-linux-amd64.tar.gz`       |
| Linux   | arm64 | `mgm-linux-arm64.tar.gz`       |
| macOS   | amd64 | `mgm-darwin-amd64.tar.gz`      |
| macOS   | arm64 | `mgm-darwin-arm64.tar.gz`      |
| Windows | amd64 | `mgm-windows-amd64.zip`        |

Extract `mgm` (or `mgm.exe`) onto your `PATH`.

### Arch Linux (AUR)

```sh
cd packaging/arch
makepkg -si
```

A `.deb`, `.rpm`, `.apk`, and Arch package are also produced by `goreleaser` for each release.

### From source

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

---

## First-time setup

```sh
$ mgm env configure
Configure Infisical credentials (profile: default)
Credentials will be saved to /home/you/.mgm/config [default]

Infisical Client ID [None]: ...
Infisical Client Secret [None]: ...
Infisical Host URL [https://secrets.labmgm.org]:

✓ Saved /home/you/.mgm/config [default]
```

Any command that needs credentials will trigger this same prompt if you skip it.

### Profiles

Switch between credential sets with `--profile` (or `MGM_PROFILE`). Each profile is a TOML section in `~/.mgm/config`:

```toml
[default]
host_url      = "https://secrets.labmgm.org"
client_id     = "..."
client_secret = "..."

[ops]
host_url      = "https://secrets.labmgm.org"
client_id     = "..."
client_secret = "..."
```

```sh
mgm --profile ops env list
```

### Pin a working directory

```sh
$ cd ~/code/my-service
$ mgm env init
… interactive picker for project / env / folder …
✓ Wrote .mgm.yaml
```

Commit `.mgm.yaml`. From then on `mgm env pull` etc. skip the picker.

```yaml
# .mgm.yaml
project_id: 7f9e...
environment: dev
folder: /backend
```

---

## Common workflows

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

---

## Configuration reference

| Place                        | Purpose                                       |
| ---------------------------- | --------------------------------------------- |
| `~/.mgm/config`              | Credentials and per-profile defaults (TOML)   |
| `./.mgm.yaml`                | Project pin (project_id, environment, folder) |
| `MGM_PROFILE`                | Active profile name                           |
| `MGM_CONFIG`                 | Override config file path                     |
| `MGM_HOST_URL`               | Override host URL                             |
| `MGM_CLIENT_ID`              | Override client ID                            |
| `MGM_CLIENT_SECRET`          | Override client secret                        |
| `MGM_PROJECT_ID`             | Default project ID                            |
| `MGM_ENVIRONMENT`            | Default environment slug                      |
| `MGM_FOLDER`                 | Default secret folder                         |
| `MGM_NO_TUI=1`               | Disable interactive prompts (CI)              |

CLI flags > project file > profile > env vars > built-in defaults.

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
