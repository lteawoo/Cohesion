#!/usr/bin/env bash
set -euo pipefail

# Dialect bounds + deny taxonomy(readonly/permission/boundary) + rollback baseline
go test ./pkg/smbcore -run 'TestHandleConn_EnforcesDialectBoundsAndRejectsSMB1|TestHandleConn_ReadonlyPhaseDenied_EmitsTelemetryReason|TestHandleConn_WriteFull_ManagePermissionDeniedReason|TestHandleConn_WriteFull_ManageOpsAndBoundaryReason|TestHandleConn_RollbackToReadonly_DeniesNewWriteRequests'

# SMB readiness semantics baseline (service/state surface)
go test ./internal/smb -run 'TestService_StartStop_Disabled|TestService_StartFailure_UpdatesReadiness|TestService_StartStop_Enabled'
go test ./internal/status -run 'TestHandleStatus_IncludesSMBProtocol|TestHandleStatus_UsesSMBReadinessProviderState'

echo "[smb-compat-baseline] ok"
