#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/opt/cohesion"
SYSTEMD_UNIT_DIR="/etc/systemd/system"
SYSTEMD_UNIT_NAME="cohesion.service"
TARGET_USER="${SUDO_USER:-${USER:-}}"
TARGET_GROUP=""
SKIP_SYSTEMD=0
SKIP_START=0

usage() {
  cat <<'EOF'
Usage: sudo ./install.sh [options]

Options:
  --install-dir <path>       Install root (default: /opt/cohesion)
  --systemd-unit-dir <path>  systemd unit directory (default: /etc/systemd/system)
  --user <name>              OS user that runs the service (default: SUDO_USER or USER)
  --group <name>             OS group for the service user (default: primary group of --user)
  --skip-systemd             Install files only, do not install or reload a systemd unit
  --skip-start               Install the systemd unit but do not enable/start or restart it
  -h, --help                 Show this help

Examples:
  sudo ./install.sh --user "$(id -un)"
  sudo ./install.sh --user "$(id -un)" --skip-start
EOF
}

log() {
  printf '[install] %s\n' "$*"
}

fail() {
  printf '[install] %s\n' "$*" >&2
  exit 1
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "required command not found: $1"
  fi
}

escape_sed_replacement() {
  printf '%s' "$1" | sed -e 's/[\/&]/\\&/g'
}

resolve_target_home() {
  local passwd_entry
  passwd_entry="$(getent passwd "$1" || true)"
  if [[ -z "$passwd_entry" ]]; then
    return 1
  fi
  printf '%s' "$passwd_entry" | cut -d: -f6
}

while (($# > 0)); do
  case "$1" in
    --install-dir)
      [[ $# -ge 2 ]] || fail "missing value for --install-dir"
      INSTALL_DIR="$2"
      shift 2
      ;;
    --systemd-unit-dir)
      [[ $# -ge 2 ]] || fail "missing value for --systemd-unit-dir"
      SYSTEMD_UNIT_DIR="$2"
      shift 2
      ;;
    --user)
      [[ $# -ge 2 ]] || fail "missing value for --user"
      TARGET_USER="$2"
      shift 2
      ;;
    --group)
      [[ $# -ge 2 ]] || fail "missing value for --group"
      TARGET_GROUP="$2"
      shift 2
      ;;
    --skip-systemd)
      SKIP_SYSTEMD=1
      shift
      ;;
    --skip-start)
      SKIP_START=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

[[ ${EUID:-$(id -u)} -eq 0 ]] || fail "run this script with sudo or as root"
[[ -n "${TARGET_USER}" ]] || fail "service user is empty; pass --user <name>"

require_command getent
require_command id
require_command install
if [[ "$SKIP_SYSTEMD" -eq 0 ]]; then
  require_command systemctl
fi

if ! id "$TARGET_USER" >/dev/null 2>&1; then
  fail "service user does not exist: $TARGET_USER"
fi

if [[ -z "$TARGET_GROUP" ]]; then
  TARGET_GROUP="$(id -gn "$TARGET_USER")"
fi
TARGET_HOME="$(resolve_target_home "$TARGET_USER" || true)"
[[ -n "$TARGET_HOME" ]] || fail "failed to resolve home directory for user: $TARGET_USER"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY_PATH="$SCRIPT_DIR/cohesion"
UPDATER_PATH="$SCRIPT_DIR/cohesion-updater"
README_PATH="$SCRIPT_DIR/README.md"
CONFIG_TEMPLATE_PATH="$SCRIPT_DIR/config/config.prod.yaml"
SERVICE_TEMPLATE_PATH="$SCRIPT_DIR/cohesion.service"

[[ -f "$BINARY_PATH" ]] || fail "cohesion binary not found next to install.sh"
[[ -f "$UPDATER_PATH" ]] || fail "cohesion-updater binary not found next to install.sh"
[[ -f "$README_PATH" ]] || fail "README.md not found next to install.sh"
[[ -f "$CONFIG_TEMPLATE_PATH" ]] || fail "config/config.prod.yaml not found next to install.sh"
[[ "$SKIP_SYSTEMD" -eq 1 || -f "$SERVICE_TEMPLATE_PATH" ]] || fail "cohesion.service not found next to install.sh"

APP_HOME="$TARGET_HOME/.cohesion"
CONFIG_DIR="$APP_HOME/config"
DATA_DIR="$APP_HOME/data"
SECRETS_DIR="$APP_HOME/secrets"
RUNTIME_ROOT="$INSTALL_DIR/runtime"
SERVICE_PATH="$SYSTEMD_UNIT_DIR/$SYSTEMD_UNIT_NAME"

log "Installing Cohesion into $INSTALL_DIR"
log "Service user: $TARGET_USER"
log "Service group: $TARGET_GROUP"
log "User home: $TARGET_HOME"

install -d -m 0755 "$INSTALL_DIR" "$INSTALL_DIR/bin" "$INSTALL_DIR/share" "$RUNTIME_ROOT" "$RUNTIME_ROOT/data" "$RUNTIME_ROOT/logs"
install -m 0755 "$BINARY_PATH" "$INSTALL_DIR/bin/cohesion"
install -m 0755 "$UPDATER_PATH" "$INSTALL_DIR/bin/cohesion-updater"
install -m 0644 "$README_PATH" "$INSTALL_DIR/share/README.md"
install -m 0644 "$CONFIG_TEMPLATE_PATH" "$INSTALL_DIR/share/config.prod.yaml"

install -d -o "$TARGET_USER" -g "$TARGET_GROUP" -m 0700 "$APP_HOME" "$CONFIG_DIR" "$DATA_DIR" "$SECRETS_DIR"
if [[ ! -f "$CONFIG_DIR/config.prod.yaml" ]]; then
  install -o "$TARGET_USER" -g "$TARGET_GROUP" -m 0644 "$CONFIG_TEMPLATE_PATH" "$CONFIG_DIR/config.prod.yaml"
fi

chown -R "$TARGET_USER:$TARGET_GROUP" "$APP_HOME"
chown -R "$TARGET_USER:$TARGET_GROUP" "$RUNTIME_ROOT"

if [[ "$SKIP_SYSTEMD" -eq 0 ]]; then
  install -d -m 0755 "$SYSTEMD_UNIT_DIR"

  temp_service="$(mktemp)"
  trap 'rm -f "$temp_service"' EXIT

  sed \
    -e "s/__COHESION_USER__/$(escape_sed_replacement "$TARGET_USER")/g" \
    -e "s/__COHESION_GROUP__/$(escape_sed_replacement "$TARGET_GROUP")/g" \
    -e "s/__COHESION_HOME__/$(escape_sed_replacement "$TARGET_HOME")/g" \
    -e "s/__COHESION_INSTALL_DIR__/$(escape_sed_replacement "$INSTALL_DIR")/g" \
    -e "s/__COHESION_RUNTIME_ROOT__/$(escape_sed_replacement "$RUNTIME_ROOT")/g" \
    "$SERVICE_TEMPLATE_PATH" > "$temp_service"

  install -m 0644 "$temp_service" "$SERVICE_PATH"
  systemctl daemon-reload

  if [[ "$SKIP_START" -eq 1 ]]; then
    log "Installed systemd unit at $SERVICE_PATH"
    log "Run 'sudo systemctl enable --now ${SYSTEMD_UNIT_NAME%.service}' after reviewing ~/.cohesion/config/config.prod.yaml"
  elif systemctl is-enabled --quiet "$SYSTEMD_UNIT_NAME" 2>/dev/null; then
    systemctl restart "$SYSTEMD_UNIT_NAME"
    log "Restarted existing service: ${SYSTEMD_UNIT_NAME%.service}"
  else
    systemctl enable --now "$SYSTEMD_UNIT_NAME"
    log "Enabled and started service: ${SYSTEMD_UNIT_NAME%.service}"
  fi
fi

cat <<EOF

Installed Cohesion
- Binary: $INSTALL_DIR/bin/cohesion
- Updater: $INSTALL_DIR/bin/cohesion-updater
- Runtime root: $RUNTIME_ROOT
- Config: $CONFIG_DIR/config.prod.yaml
- Data directory: $DATA_DIR
- Secrets directory: $SECRETS_DIR

Useful commands:
- View service logs: sudo journalctl -u ${SYSTEMD_UNIT_NAME%.service} -f
- Check service status: sudo systemctl status ${SYSTEMD_UNIT_NAME%.service} --no-pager
- Upgrade from a new release archive: sudo ./install.sh --user "$TARGET_USER"
EOF
