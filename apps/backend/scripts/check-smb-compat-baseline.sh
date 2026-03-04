#!/usr/bin/env bash
set -euo pipefail

# Dialect bounds + deny taxonomy(readonly/permission/boundary) + rollback baseline
module_target="./pkg/smbcore"
if go list github.com/lteawoo/smb-core >/dev/null 2>&1; then
  module_target="github.com/lteawoo/smb-core"
fi
go test "$module_target" -run 'TestHandleConn_EnforcesDialectBoundsAndRejectsSMB1|TestHandleConn_ReadonlyPhaseDenied_EmitsTelemetryReason|TestHandleConn_WriteFull_ManagePermissionDeniedReason|TestHandleConn_WriteFull_ManageOpsAndBoundaryReason|TestHandleConn_RollbackToReadonly_DeniesNewWriteRequests|TestEngineCheckUsability'

# SMB readiness semantics baseline (service/state surface)
go test ./internal/smb -run 'TestService_StartStop_Disabled|TestService_StartFailure_UpdatesReadiness|TestService_StartStop_Enabled'
go test ./internal/status -run 'TestHandleStatus_IncludesSMBProtocol|TestHandleStatus_UsesSMBReadinessProviderState'

echo "[smb-compat-baseline] ok"
