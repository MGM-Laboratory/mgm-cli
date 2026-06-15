# Release runbook — `mgm` / Megumi Code CLI

Releases are cut by GoReleaser on a pushed `v*` tag (`.github/workflows/release.yml`).

```bash
git tag v0.2.0
git push origin v0.2.0
```

## What a tagged release produces

Always (no external setup required):

- **GitHub Release** with cross-platform archives (`mgm-<os>-<arch>.tar.gz` / `.zip`
  for linux/darwin/windows × amd64/arm64, minus windows/arm64) and `checksums.txt`.
- **Linux packages** attached to the release: `.deb`, `.rpm` (Fedora/RHEL),
  `.apk` (Alpine), and Arch `.pkg.tar.zst` — built by nfpm.

Conditionally (only when the matching secret/repo is configured — otherwise the
publisher is skipped and the core release still succeeds):

- **Homebrew cask** → tap repo `MGM-Laboratory/homebrew-mgm`
- **Scoop manifest** → bucket repo `MGM-Laboratory/scoop-mgm`
- **Nix (NUR)** → `MGM-Laboratory/nur`
- **AUR** (binary package `mgm-bin`) → `ssh://aur@aur.archlinux.org/mgm-bin.git`
- **Docker image** (multi-arch) → Docker Hub

The release workflow builds a `--skip` list from the secrets that are present, so
a missing tap/bucket/AUR key never breaks the binary + package release.

## External repos to create (one-time)

These live in their own repos — the sanctioned exception to the four-repo layout
(prompt-0 §11):

| Target   | Repo                              | Notes |
|----------|-----------------------------------|-------|
| Homebrew | `MGM-Laboratory/homebrew-mgm`     | empty repo; GoReleaser writes `Casks/mgm.rb` |
| Scoop    | `MGM-Laboratory/scoop-mgm`        | empty repo; GoReleaser writes `mgm.json` |
| Nix NUR  | `MGM-Laboratory/nur`              | NUR-style repo |
| AUR      | `aur.archlinux.org/mgm-bin`       | AUR package, pushed over SSH |

## Secrets / variables (repo → Settings → Secrets and variables → Actions)

| Name                          | Type     | Used for | If absent |
|-------------------------------|----------|----------|-----------|
| `GITHUB_TOKEN`                | auto     | GitHub Release | always present |
| `HOMEBREW_TAP_GITHUB_TOKEN`   | secret   | push to the tap | Homebrew skipped |
| `SCOOP_BUCKET_GITHUB_TOKEN`   | secret   | push to the bucket | Scoop skipped |
| `NUR_GITHUB_TOKEN`            | secret   | push to NUR (+ installs Nix for `nix-hash`) | Nix skipped |
| `AUR_KEY`                     | secret   | SSH private key for AUR | AUR skipped |
| `DOCKERHUB_USERNAME`          | secret   | Docker Hub login | Docker image skipped |
| `DOCKERHUB_TOKEN`             | secret   | Docker Hub login | Docker image skipped |
| `DOCKERHUB_IMAGE`             | variable | image name (default `mgmlaboratory/mgm`) | uses default |

The tap/bucket/NUR tokens are GitHub **PATs** with `repo`/`contents:write` scope on
the *target* repo (the default `GITHUB_TOKEN` only has access to this repo, so it
cannot push to the external ones).

## Versioning

Version metadata is injected via `ldflags` into `internal/version`, so after
install `mgm version` reports the tag:

```
mgm v0.2.0
commit: <sha>
built: <timestamp>
```

## Validate config locally

```bash
goreleaser check                                   # config lint
goreleaser release --snapshot --clean --skip=publish   # full dry build into ./dist
```

The CI `package-build` / `package-install` jobs additionally build the packages
and install them in real Ubuntu/Debian/Fedora/Alpine/Arch containers on every PR.
