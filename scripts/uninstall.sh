#!/bin/sh
# SIIG Uninstall Script (macOS / Linux)
# Usage: curl -fsSL https://get.siig.dev/uninstall.sh | sh

set -e

BINARY_NAME="siig"

# install.sh picks the first writable directory already on PATH, so uninstall
# has to search the same places instead of assuming /usr/local/bin.
if [ -n "$INSTALL_DIR" ]; then
  candidates="$INSTALL_DIR"
else
  candidates="$(echo "$PATH" | tr ':' ' ') $HOME/.siig/bin /usr/local/bin /opt/homebrew/bin $HOME/.local/bin"
fi

removed=0
for dir in $candidates; do
  [ -n "$dir" ] || continue
  binary="$dir/$BINARY_NAME"
  [ -f "$binary" ] || continue

  echo "→ Removing ${binary}..."
  if [ -w "$dir" ]; then
    rm -f "$binary"
  else
    echo "→ Requires sudo to remove from ${dir}"
    sudo rm -f "$binary"
  fi
  removed=1
done

if [ "$removed" -eq 0 ]; then
  echo "SIIG binary not found in the searched directories."
fi

# The rules cache lives under the user's home regardless of where the binary went.
if [ -d "$HOME/.siig" ]; then
  echo "→ Removing cached rules from ${HOME}/.siig..."
  rm -rf "$HOME/.siig"
  removed=1
fi

if [ "$removed" -eq 0 ]; then
  echo "Nothing to uninstall."
  exit 0
fi

echo ""
echo "✓ SIIG uninstalled successfully."
