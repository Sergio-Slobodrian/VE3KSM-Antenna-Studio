#!/usr/bin/env bash
# Sets up the build environment for Antenna Studio on Ubuntu 24.04 (WSL2).
# Installs: git, make, curl, Go 1.24, Node.js 20 LTS.

set -euo pipefail

GO_VERSION="1.24.0"
GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_TARBALL}"
GO_INSTALL_DIR="/usr/local/go"
NODE_MAJOR="20"

# ── Helpers ───────────────────────────────────────────────────────────────────

step()  { echo ""; echo "==> $*"; }
ok()    { echo "    [OK] $*"; }
warn()  { echo "    [!!] $*"; }

require_sudo() {
    if ! sudo -n true 2>/dev/null; then
        echo "This script needs sudo access. You may be prompted for your password."
    fi
}

# ── System packages ───────────────────────────────────────────────────────────

step "Updating apt package lists"
sudo apt-get update -q

step "Upgrading installed packages"
sudo apt-get upgrade -y -q

step "Installing base tools (git, make, curl)"
sudo apt-get install -y -q git make curl
ok "git $(git --version | awk '{print $3}')"
ok "make $(make --version | head -1)"
ok "curl $(curl --version | head -1 | awk '{print $2}')"

# ── Go ────────────────────────────────────────────────────────────────────────

step "Checking Go installation"

need_go=true
if command -v go &>/dev/null; then
    installed_go=$(go version | awk '{print $3}' | sed 's/go//')
    # Compare major.minor only (bash-friendly)
    installed_major=$(echo "$installed_go" | cut -d. -f1)
    installed_minor=$(echo "$installed_go" | cut -d. -f2)
    if [ "$installed_major" -gt 1 ] || { [ "$installed_major" -eq 1 ] && [ "$installed_minor" -ge 22 ]; }; then
        ok "Go $installed_go already installed — skipping"
        need_go=false
    else
        warn "Go $installed_go is too old (need ≥ 1.22) — replacing"
    fi
fi

if $need_go; then
    step "Downloading Go $GO_VERSION"
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    curl -fsSL "$GO_URL" -o "$TMP_DIR/$GO_TARBALL"
    ok "Downloaded $GO_TARBALL"

    step "Installing Go to $GO_INSTALL_DIR"
    sudo rm -rf "$GO_INSTALL_DIR"
    sudo tar -C /usr/local -xzf "$TMP_DIR/$GO_TARBALL"
    ok "Extracted to $GO_INSTALL_DIR"
fi

# Add Go to PATH in ~/.bashrc (idempotent)
GOPATH_EXPORT='export PATH=$PATH:/usr/local/go/bin'
if ! grep -qF '/usr/local/go/bin' "$HOME/.bashrc"; then
    echo "" >> "$HOME/.bashrc"
    echo "# Go" >> "$HOME/.bashrc"
    echo "$GOPATH_EXPORT" >> "$HOME/.bashrc"
    ok "Added Go to PATH in ~/.bashrc"
fi

# Make Go available for the rest of this script
export PATH="$PATH:/usr/local/go/bin"
ok "go $(/usr/local/go/bin/go version | awk '{print $3}')"

# ── Node.js via NodeSource ────────────────────────────────────────────────────

step "Checking Node.js installation"

need_node=true
if command -v node &>/dev/null; then
    installed_node=$(node --version | sed 's/v//' | cut -d. -f1)
    if [ "$installed_node" -ge 18 ]; then
        ok "Node.js v$(node --version | sed 's/v//') already installed — skipping"
        need_node=false
    else
        warn "Node.js $(node --version) is too old (need ≥ 18) — replacing"
    fi
fi

if $need_node; then
    step "Installing Node.js $NODE_MAJOR LTS via NodeSource"
    curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | sudo -E bash -
    sudo apt-get install -y -q nodejs
    ok "node $(node --version)"
    ok "npm $(npm --version)"
fi

# ── Summary ───────────────────────────────────────────────────────────────────

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Build environment ready!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo " Installed versions:"
echo "   go      $(/usr/local/go/bin/go version | awk '{print $3}')"
echo "   node    $(node --version)"
echo "   npm     $(npm --version)"
echo "   git     $(git --version | awk '{print $3}')"
echo ""
echo " Next steps — clone the repo and build:"
echo ""
echo "   git clone https://github.com/Sergio-Slobodrian/VE3KSM-Antenna-Studio.git"
echo "   cd VE3KSM-Antenna-Studio"
echo "   make deps    # install frontend npm packages (one-time)"
echo "   make build   # compile ./bin/antenna-studio"
echo "   make test    # run unit tests"
echo ""
echo " See doc/INSTALL.md for the full installation guide."
echo ""
