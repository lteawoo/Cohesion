#!/usr/bin/env bash
set -euo pipefail

module_target="./pkg/smbcore"
if go list github.com/lteawoo/smb-core >/dev/null 2>&1; then
  module_target="github.com/lteawoo/smb-core"
fi
imports="$(go list -f '{{ join .Imports "\n" }}' "$module_target")"

# Block direct imports of Cohesion domain packages and their subpackages.
disallowed='^taeu\.kr/cohesion/internal/(account|space|config)(/|$)'
if printf '%s\n' "$imports" | rg -q "$disallowed"; then
  echo "[smbcore-boundary] disallowed Cohesion domain import detected in smbcore module"
  printf '%s\n' "$imports" | rg "$disallowed" || true
  exit 1
fi

echo "[smbcore-boundary] ok"
