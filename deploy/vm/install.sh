#!/usr/bin/env bash
# deploy/vm/install.sh
# Installs obsidianstack-agent and/or obsidianstack-server on a Linux VM.
#
# Usage:
#   sudo bash install.sh [agent|server|both]          # pull from Docker Hub (default)
#   sudo bash install.sh [agent|server|both] --from-source  # build locally (requires Go 1.24+)
#
# Requirements (default): Docker
# Requirements (--from-source): Go 1.24+, repo checked out on this host
set -euo pipefail

COMPONENT="${1:-both}"
FROM_SOURCE="${2:-}"
DOCKER_IMAGE_PREFIX="marocz/obsidianstack"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/obsidianstack"
DATA_DIR="/var/lib/obsidianstack"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

# ── Create system user ───────────────────────────────────────────────────────
if ! id obsidianstack &>/dev/null; then
  useradd --system --no-create-home --shell /usr/sbin/nologin obsidianstack
  echo "Created system user: obsidianstack"
fi

# ── Create directories ───────────────────────────────────────────────────────
mkdir -p "$CONFIG_DIR" "$DATA_DIR"
chown obsidianstack:obsidianstack "$DATA_DIR"

# ── Binary install helper ────────────────────────────────────────────────────
install_binary() {
  local name="$1"      # e.g. obsidianstack-agent
  local build_path="$2" # e.g. ./agent/cmd/agent

  if [[ "$FROM_SOURCE" == "--from-source" ]]; then
    echo "Building $name from source (requires Go 1.24+)..."
    (cd "$REPO_ROOT" && go build -ldflags="-s -w" -o "$INSTALL_DIR/$name" "$build_path")
  else
    echo "Pulling $name binary from Docker Hub..."
    if ! command -v docker &>/dev/null; then
      echo "ERROR: Docker not found. Install Docker or use --from-source flag." >&2
      exit 1
    fi
    local image="${DOCKER_IMAGE_PREFIX}-${name##*obsidianstack-}:latest"
    docker pull --quiet "$image"
    local cid
    cid=$(docker create "$image")
    docker cp "$cid:/$name" "$INSTALL_DIR/$name"
    docker rm "$cid" >/dev/null
    chmod +x "$INSTALL_DIR/$name"
    echo "Installed $INSTALL_DIR/$name"
  fi
}

# ── Install agent ────────────────────────────────────────────────────────────
if [[ "$COMPONENT" == "agent" || "$COMPONENT" == "both" ]]; then
  echo "--- Installing obsidianstack-agent ---"
  install_binary "obsidianstack-agent" "./agent/cmd/agent"

  if [[ ! -f "$CONFIG_DIR/agent.yaml" ]]; then
    cp "$REPO_ROOT/deploy/vm/agent.yaml" "$CONFIG_DIR/agent.yaml"
    echo "Installed config: $CONFIG_DIR/agent.yaml  ← EDIT THIS"
  else
    echo "Config already exists, skipping: $CONFIG_DIR/agent.yaml"
  fi

  if [[ ! -f "$CONFIG_DIR/agent.env" ]]; then
    cp "$REPO_ROOT/deploy/vm/agent.env" "$CONFIG_DIR/agent.env"
    chmod 600 "$CONFIG_DIR/agent.env"
    chown obsidianstack:obsidianstack "$CONFIG_DIR/agent.env"
    echo "Installed secrets: $CONFIG_DIR/agent.env  ← EDIT THIS (add real passwords)"
  fi

  cp "$REPO_ROOT/deploy/vm/obsidianstack-agent.service" /etc/systemd/system/
  systemctl daemon-reload
  systemctl enable obsidianstack-agent

  echo ""
  echo "Agent installed. Next steps:"
  echo "  1. Edit $CONFIG_DIR/agent.yaml    (sources, server_endpoint)"
  echo "  2. Edit $CONFIG_DIR/agent.env     (passwords / tokens)"
  echo "  3. sudo systemctl start obsidianstack-agent"
  echo "  4. sudo journalctl -fu obsidianstack-agent"
fi

# ── Install server ───────────────────────────────────────────────────────────
if [[ "$COMPONENT" == "server" || "$COMPONENT" == "both" ]]; then
  echo "--- Installing obsidianstack-server ---"
  install_binary "obsidianstack-server" "./server/cmd/server"

  if [[ ! -f "$CONFIG_DIR/server.yaml" ]]; then
    cp "$REPO_ROOT/deploy/vm/server.yaml" "$CONFIG_DIR/server.yaml"
    echo "Installed config: $CONFIG_DIR/server.yaml  ← EDIT THIS"
  else
    echo "Config already exists, skipping: $CONFIG_DIR/server.yaml"
  fi

  if [[ ! -f "$CONFIG_DIR/server.env" ]]; then
    touch "$CONFIG_DIR/server.env"
    chmod 600 "$CONFIG_DIR/server.env"
    chown obsidianstack:obsidianstack "$CONFIG_DIR/server.env"
    echo "Installed secrets: $CONFIG_DIR/server.env  ← Add SLACK_WEBHOOK_URL etc."
  fi

  cp "$REPO_ROOT/deploy/vm/obsidianstack-server.service" /etc/systemd/system/
  systemctl daemon-reload
  systemctl enable obsidianstack-server

  echo ""
  echo "Server installed. Next steps:"
  echo "  1. Edit $CONFIG_DIR/server.yaml   (ports, alert rules)"
  echo "  2. Edit $CONFIG_DIR/server.env    (webhook URLs etc.)"
  echo "  3. sudo systemctl start obsidianstack-server"
  echo "  4. sudo journalctl -fu obsidianstack-server"
fi
