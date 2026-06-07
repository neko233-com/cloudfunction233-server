#!/usr/bin/env sh
set -eu

REPO="${CF233_REPO:-neko233/cloudfunction233-server}"
INSTALL_DIR="${CF233_HOME:-/opt/cloudfunction233-server}"
BIN="$INSTALL_DIR/cloudfunction233-server"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

api="https://api.github.com/repos/$REPO/releases/latest"
asset_url="$(curl -fsSL "$api" | grep browser_download_url | grep "${os}_${arch}" | head -n1 | cut -d '"' -f4)"
if [ -z "$asset_url" ]; then
  echo "release asset not found for ${os}_${arch}" >&2
  exit 1
fi

mkdir -p "$INSTALL_DIR"
curl -fsSL "$asset_url" -o "$tmp/pkg"
case "$asset_url" in
  *.tar.gz|*.tgz) tar -xzf "$tmp/pkg" -C "$tmp" ;;
  *.zip) unzip -q "$tmp/pkg" -d "$tmp" ;;
  *) cp "$tmp/pkg" "$tmp/cloudfunction233-server" ;;
esac

install -m 0755 "$tmp/cloudfunction233-server" "$BIN"
"$BIN" init-config >/dev/null

echo "installed: $BIN"
echo "start:     $BIN start"
echo "status:    $BIN status"
echo "autostart: $BIN autostart enable"
