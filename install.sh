#!/usr/bin/env sh
# Kilden CLI (kd) installer.
#
#   curl -fsSL https://raw.githubusercontent.com/kildenhq/kilden-cli/main/install.sh | bash
#
# Detects your OS/CPU, downloads the matching release archive, verifies its
# checksum, and installs `kd` into ~/.local/bin (override with env vars below).
#
# Env overrides:
#   KILDEN_INSTALL_DIR   where to put `kd`       (default: ~/.local/bin)
#   KILDEN_VERSION       release tag to install  (default: latest)
#
# Windows: this script doesn't run there — grab kd_windows_amd64.zip from
# https://github.com/kildenhq/kilden-cli/releases/latest

set -eu

REPO="kildenhq/kilden-cli"
INSTALL_DIR="${KILDEN_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${KILDEN_VERSION:-latest}"

info() { printf '\033[36m==>\033[0m %s\n' "$1"; }
err() { printf '\033[31merror:\033[0m %s\n' "$1" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || err "curl is required"
command -v tar >/dev/null 2>&1 || err "tar is required"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux | darwin) ;;
  *) err "unsupported OS '$os' — on Windows download kd_windows_amd64.zip from the releases page" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) err "unsupported architecture '$arch'" ;;
esac

asset="kd_${os}_${arch}.tar.gz"
if [ "$VERSION" = "latest" ]; then
  base="https://github.com/${REPO}/releases/latest/download"
else
  base="https://github.com/${REPO}/releases/download/${VERSION}"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

info "Downloading ${asset} (${VERSION})"
curl -fsSL -o "$tmp/kd.tgz" "${base}/${asset}" || err "download failed: ${base}/${asset}"

# Verify the sha256 against the release's checksums.txt. Every release ships
# one, so fail closed: if we can't fetch it or can't hash locally, abort rather
# than install an unverified binary.
info "Verifying checksum"
checks=$(curl -fsSL "${base}/checksums.txt") || err "could not download checksums.txt to verify the archive"
if command -v sha256sum >/dev/null 2>&1; then
  sum=$(sha256sum "$tmp/kd.tgz" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  sum=$(shasum -a 256 "$tmp/kd.tgz" | awk '{print $1}')
else
  err "need 'sha256sum' or 'shasum' to verify the download"
fi
# checksums.txt lines are exactly "<sha256>  <filename>"; match the whole line.
printf '%s\n' "$checks" | grep -Fqx "$sum  $asset" || err "checksum mismatch for $asset"
info "Checksum verified"

tar -xzf "$tmp/kd.tgz" -C "$tmp" kd || err "could not extract kd from archive"
mkdir -p "$INSTALL_DIR"
mv "$tmp/kd" "$INSTALL_DIR/kd"
chmod +x "$INSTALL_DIR/kd"

info "Installed kd to $INSTALL_DIR/kd"
"$INSTALL_DIR/kd" --version || err "installed kd failed to run"

# Nudge if the install dir isn't on PATH.
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    printf '\n\033[33mnote:\033[0m %s is not on your PATH. Add it to your shell profile:\n' "$INSTALL_DIR"
    # $PATH must stay literal here — it's a snippet for the user to paste.
    # shellcheck disable=SC2016
    printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
    ;;
esac

printf '\nRun \033[36mkd login\033[0m to sign in.\n'
