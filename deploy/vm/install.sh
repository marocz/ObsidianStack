#!/usr/bin/env bash
# deploy/vm/install.sh
# Installs obsidianstack-agent and/or obsidianstack-server on a Linux VM.
# Run as root: sudo bash install.sh [agent|server|both]
set -euo pipefail

COMPONENT="${1:-both}"
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

# ── Install agent ────────────────────────────────────────────────────────────
if [[ "$COMPONENT" == "agent" || "$COMPONENT" == "both" ]]; then
  echo "Installing obsidianstack-agent..."

  # Build binary (requires Go 1.24+)
  (cd "$REPO_ROOT" && go build -o "$INSTALL_DIR/obsidianstack-agent" ./agent/cmd/agent)

  # Install config
  if [[ ! -f "$CONFIG_DIR/agent.yaml" ]]; then
    cp "$REPO_ROOT/deploy/vm/agent.yaml" "$CONFIG_DIR/agent.yaml"
    echo "Installed config: $CONFIG_DIR/agent.yaml  ← EDIT THIS"
  fi

  # Install secrets file (only if it doesn't exist — never overwrite)
  if [[ ! -f "$CONFIG_DIR/agent.env" ]]; then
    cp "$REPO_ROOT/deploy/vm/agent.env" "$CONFIG_DIR/agent.env"
    chmod 600 "$CONFIG_DIR/agent.env"
    chown obsidianstack:obsidianstack "$CONFIG_DIR/agent.env"
    echo "Installed secrets: $CONFIG_DIR/agent.env  ← EDIT THIS (add real passwords)"
  fi

  # Install and enable systemd unit
  cp "$REPO_ROOT/deploy/vm/obsidianstack-agent.service" /etc/systemd/system/
  systemctl daemon-reload
  systemctl enable obsidianstack-agent
  echo ""
  echo "Agent installed. Next steps:"
  echo "  1. Edit $CONFIG_DIR/agent.yaml   (sources, server endpoint)"
  echo "  2. Edit $CONFIG_DIR/agent.env    (passwords / tokens)"
  echo "  3. systemctl start obsidianstack-agent"
  echo "  4. journalctl -fu obsidianstack-agent"
fi

# ── Install server ───────────────────────────────────────────────────────────
if [[ "$COMPONENT" == "server" || "$COMPONENT" == "both" ]]; then
  echo "Installing obsidianstack-server..."

  (cd "$REPO_ROOT" && go build -o "$INSTALL_DIR/obsidianstack-server" ./server/cmd/server)

  if [[ ! -f "$CONFIG_DIR/server.yaml" ]]; then
    cp "$REPO_ROOT/deploy/vm/server.yaml" "$CONFIG_DIR/server.yaml"
    echo "Installed config: $CONFIG_DIR/server.yaml  ← EDIT THIS"
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
  echo "  3. systemctl start obsidianstack-server"
  echo "  4. journalctl -fu obsidianstack-server"
fi
