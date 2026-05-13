#!/usr/bin/env sh
# mgm CLI installer — Linux, macOS, WSL, Git Bash, MSYS2, Cygwin.
#
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/MGM-Laboratory/mgm-cli/main/install.sh | bash
#
# Pin a version:
#   curl -fsSL https://raw.githubusercontent.com/MGM-Laboratory/mgm-cli/main/install.sh | MGM_VERSION=v0.1.0 bash
#
# Custom install dir (no sudo):
#   curl -fsSL https://raw.githubusercontent.com/MGM-Laboratory/mgm-cli/main/install.sh | MGM_INSTALL_DIR=$HOME/.local/bin bash
set -eu

REPO="${MGM_REPO:-MGM-Laboratory/mgm-cli}"
VERSION="${MGM_VERSION:-latest}"
INSTALL_DIR="${MGM_INSTALL_DIR:-}"

# ---------- pretty output ----------
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  C_BOLD=$(printf '\033[1m');   C_RESET=$(printf '\033[0m')
  C_GREEN=$(printf '\033[32m'); C_RED=$(printf '\033[31m')
  C_BLUE=$(printf '\033[34m');  C_DIM=$(printf '\033[2m')
else
  C_BOLD=""; C_RESET=""; C_GREEN=""; C_RED=""; C_BLUE=""; C_DIM=""
fi
say()  { printf "%s==>%s %s\n" "$C_BLUE" "$C_RESET" "$*"; }
ok()   { printf "%s✓%s %s\n"  "$C_GREEN" "$C_RESET" "$*"; }
die()  { printf "%serror:%s %s\n" "$C_RED" "$C_RESET" "$*" >&2; exit 1; }

# ---------- detect ----------
need() { command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"; }
need uname
need tar
need mkdir
if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -qO "$2" "$1"; }
else
  die "need either curl or wget"
fi

uname_s=$(uname -s 2>/dev/null || echo unknown)
uname_m=$(uname -m 2>/dev/null || echo unknown)

case "$uname_s" in
  Linux*)            os=linux;   ext=tar.gz; binname=mgm ;;
  Darwin*)           os=darwin;  ext=tar.gz; binname=mgm ;;
  MINGW*|MSYS*|CYGWIN*|Windows_NT)
                     os=windows; ext=zip;    binname=mgm.exe ;;
  *) die "unsupported OS: $uname_s (try install.ps1 on Windows PowerShell)" ;;
esac

case "$uname_m" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) die "unsupported architecture: $uname_m" ;;
esac

# Windows arm64 isn't shipped — guard explicitly.
if [ "$os" = "windows" ] && [ "$arch" = "arm64" ]; then
  die "windows/arm64 builds aren't published — use windows/amd64 under Rosetta-style emulation"
fi

# ---------- pick install dir ----------
if [ -z "$INSTALL_DIR" ]; then
  if [ "$os" = "darwin" ] && [ -d /opt/homebrew/bin ] && [ -w /opt/homebrew/bin ]; then
    INSTALL_DIR=/opt/homebrew/bin
  elif [ -w /usr/local/bin ] 2>/dev/null; then
    INSTALL_DIR=/usr/local/bin
  elif [ "$(id -u 2>/dev/null || echo 1000)" = "0" ]; then
    INSTALL_DIR=/usr/local/bin
  else
    INSTALL_DIR="${HOME:-/tmp}/.local/bin"
  fi
fi
mkdir -p "$INSTALL_DIR" 2>/dev/null || true

# ---------- resolve URL ----------
asset="mgm-${os}-${arch}.${ext}"
if [ "$VERSION" = "latest" ]; then
  url="https://github.com/${REPO}/releases/latest/download/${asset}"
else
  url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
fi

# ---------- download ----------
tmp=$(mktemp -d 2>/dev/null || mktemp -d -t mgm)
trap 'rm -rf "$tmp"' EXIT INT HUP TERM

say "Detected ${C_BOLD}${os}/${arch}${C_RESET}"
say "Downloading ${C_DIM}${url}${C_RESET}"
fetch "$url" "$tmp/$asset" || die "download failed: $url"

# ---------- extract ----------
case "$ext" in
  tar.gz) tar -xzf "$tmp/$asset" -C "$tmp" ;;
  zip)
    if command -v unzip >/dev/null 2>&1; then
      unzip -q "$tmp/$asset" -d "$tmp"
    elif command -v 7z >/dev/null 2>&1; then
      7z x -o"$tmp" "$tmp/$asset" >/dev/null
    else
      die "need unzip (apt install unzip / brew install unzip) to extract zip archives"
    fi
    ;;
esac

[ -f "$tmp/$binname" ] || die "archive did not contain $binname"

# ---------- install ----------
target="$INSTALL_DIR/$binname"
if [ -w "$INSTALL_DIR" ] || [ ! -e "$INSTALL_DIR" ]; then
  install -m 0755 "$tmp/$binname" "$target" 2>/dev/null \
    || { cp "$tmp/$binname" "$target" && chmod 0755 "$target"; }
elif command -v sudo >/dev/null 2>&1; then
  say "Installing to ${INSTALL_DIR} ${C_DIM}(sudo required)${C_RESET}"
  sudo install -m 0755 "$tmp/$binname" "$target"
else
  die "$INSTALL_DIR not writable and sudo unavailable. Re-run with MGM_INSTALL_DIR=\$HOME/.local/bin"
fi

ok "installed: $target"

# ---------- PATH guidance ----------
case ":${PATH:-}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    printf "\n%s%s is not on your PATH%s\n" "$C_BOLD" "$INSTALL_DIR" "$C_RESET"
    printf "Add this line to your shell profile (~/.bashrc, ~/.zshrc, etc.):\n"
    printf "  %sexport PATH=\"%s:\$PATH\"%s\n\n" "$C_BOLD" "$INSTALL_DIR" "$C_RESET"
    ;;
esac

# ---------- post-install hint ----------
"$target" version 2>/dev/null | head -n1 || true
printf "\nNext: %smgm env configure%s to set Infisical credentials.\n" "$C_BOLD" "$C_RESET"
