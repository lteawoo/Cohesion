#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$SCRIPT_DIR/cohesion.pid"

if [[ ! -f "$PID_FILE" ]]; then
  echo "cohesion.pid not found. Cohesion may already be stopped."
  exit 0
fi

PID="$(tr -d '[:space:]' < "$PID_FILE")"
if [[ -z "$PID" ]]; then
  echo "cohesion.pid is empty. Removing stale PID file."
  rm -f "$PID_FILE"
  exit 0
fi

if ! kill -0 "$PID" 2>/dev/null; then
  echo "Process $PID is not running. Removing stale PID file."
  rm -f "$PID_FILE"
  exit 0
fi

echo "Stopping Cohesion (PID: $PID)..."
kill "$PID"

for _ in {1..20}; do
  if ! kill -0 "$PID" 2>/dev/null; then
    echo "Cohesion stopped."
    rm -f "$PID_FILE"
    exit 0
  fi
  sleep 0.5
done

echo "Cohesion did not stop gracefully. Sending SIGKILL..."
kill -9 "$PID" || true
rm -f "$PID_FILE"
echo "Cohesion stopped."
