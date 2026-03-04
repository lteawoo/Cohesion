#!/usr/bin/env bash
set -euo pipefail

# Unified publication gate set for smbcore external release readiness.
./scripts/check-smbcore-boundary.sh
./scripts/check-smb-compat-baseline.sh
go test -tags integration ./internal/smb -run TestSMB

echo "[smb-publication-gates] ok"
