#!/bin/sh
set -eu

SERVICE_USER="cohesion"
SERVICE_GROUP="cohesion"
SERVICE_HOME="/var/lib/cohesion"
APP_HOME="$SERVICE_HOME/.cohesion"
CONFIG_DIR="$APP_HOME/config"
DATA_DIR="$APP_HOME/data"
SECRETS_DIR="$APP_HOME/secrets"
RUNTIME_DIR="$SERVICE_HOME/runtime"
CONFIG_TEMPLATE="/usr/share/cohesion/config.prod.yaml"
TARGET_CONFIG="$CONFIG_DIR/config.prod.yaml"

create_group() {
  if getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
    return 0
  fi
  groupadd --system "$SERVICE_GROUP"
}

resolve_nologin_shell() {
  if [ -x /usr/sbin/nologin ]; then
    printf '%s\n' /usr/sbin/nologin
    return 0
  fi
  if [ -x /sbin/nologin ]; then
    printf '%s\n' /sbin/nologin
    return 0
  fi
  printf '%s\n' /bin/false
}

create_user() {
  if getent passwd "$SERVICE_USER" >/dev/null 2>&1; then
    return 0
  fi
  useradd \
    --system \
    --gid "$SERVICE_GROUP" \
    --home-dir "$SERVICE_HOME" \
    --shell "$(resolve_nologin_shell)" \
    --comment "Cohesion service user" \
    "$SERVICE_USER"
}

prepare_dirs() {
  mkdir -p "$SERVICE_HOME" "$CONFIG_DIR" "$DATA_DIR" "$SECRETS_DIR" "$RUNTIME_DIR" "$RUNTIME_DIR/data" "$RUNTIME_DIR/logs"
  chown -R "$SERVICE_USER:$SERVICE_GROUP" "$SERVICE_HOME"
  chmod 0755 "$SERVICE_HOME" "$RUNTIME_DIR" "$RUNTIME_DIR/data" "$RUNTIME_DIR/logs"
  chmod 0700 "$APP_HOME" "$CONFIG_DIR" "$DATA_DIR" "$SECRETS_DIR"
}

seed_config() {
  if [ -f "$TARGET_CONFIG" ] || [ ! -f "$CONFIG_TEMPLATE" ]; then
    return 0
  fi
  cp "$CONFIG_TEMPLATE" "$TARGET_CONFIG"
  chown "$SERVICE_USER:$SERVICE_GROUP" "$TARGET_CONFIG"
  chmod 0640 "$TARGET_CONFIG"
}

reload_or_restart_service() {
  if ! command -v systemctl >/dev/null 2>&1; then
    return 0
  fi

  systemctl daemon-reload >/dev/null 2>&1 || true

  if systemctl is-enabled --quiet cohesion.service 2>/dev/null || systemctl is-active --quiet cohesion.service 2>/dev/null; then
    systemctl try-restart cohesion.service >/dev/null 2>&1 || true
  fi
}

create_group
create_user
prepare_dirs
seed_config
reload_or_restart_service
