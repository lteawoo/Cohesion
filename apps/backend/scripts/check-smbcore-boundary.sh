#!/usr/bin/env bash
set -euo pipefail

imports="$(go list -f '{{ join .Imports "\n" }}' ./pkg/smbcore)"

disallowed='^taeu\.kr/cohesion/internal/(account|space|config)$'
if printf '%s\n' "$imports" | rg -q "$disallowed"; then
  echo "[smbcore-boundary] disallowed Cohesion domain import detected in pkg/smbcore"
  printf '%s\n' "$imports" | rg "$disallowed" || true
  exit 1
fi

echo "[smbcore-boundary] ok"
