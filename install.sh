#!/usr/bin/env sh
# Installs the latest mgm CLI release into /usr/local/bin (or $MGM_INSTALL_DIR).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/mgm/mgm-cli/main/install.sh | sh
#   curl -fsSL .../install.sh | MGM_VERSION=v0.2.0 sh
set -eu

REPO="${MGM_REPO:-mgm/mgm-cli}"
VERSION="${MGM_VERSION:-latest}"
INSTALL_DIR="${MGM_INSTALL_DIR:-/usr/local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  linux)  os=linux ;;
  darwin) os=darwin ;;
  *)      echo "unsupported OS: $os" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  url="https://github.com/${REPO}/releases/latest/download/mgm-${os}-${arch}.tar.gz"
else
  url="https://github.com/${REPO}/releases/download/${VERSION}/mgm-${os}-${arch}.tar.gz"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $url"
curl -fsSL "$url" | tar -xz -C "$tmp"

if [ ! -f "$tmp/mgm" ]; then
  echo "archive did not contain mgm binary" >&2
  exit 1
fi

if [ -w "$INSTALL_DIR" ]; then
  install -m 0755 "$tmp/mgm" "$INSTALL_DIR/mgm"
else
  echo "Installing to $INSTALL_DIR (sudo required)"
  sudo install -m 0755 "$tmp/mgm" "$INSTALL_DIR/mgm"
fi

echo "Installed: $("$INSTALL_DIR/mgm" version | head -n1)"
