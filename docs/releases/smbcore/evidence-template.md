# SMBCore Release Evidence Template

이 문서는 `smbcore` 릴리즈 후보(`rc`/stable)별 게이트 증빙 템플릿이다.

## 1. Release Metadata

- Version:
- Tag:
- Commit SHA:
- Date (UTC):
- Owner:

## 2. Boundary Guard

- Command:
  - `cd apps/backend && ./scripts/check-smbcore-boundary.sh`
- Result:
  - pass/fail:
  - log summary:

## 3. Compatibility Baseline

- Command:
  - `cd apps/backend && ./scripts/check-smb-compat-baseline.sh`
- Scope confirmation:
  - dialect bounds (`2.1`~`3.1.1`, SMB1 reject)
  - rollout phase deny semantics (`readonly_phase_denied`)
  - deny taxonomy (`permission_denied`, `path_boundary_violation`)
  - readiness semantics (`internal/smb`, `internal/status`)
- Result:
  - pass/fail:
  - log summary:

## 4. SMB Integration Smoke

- Command:
  - `cd apps/backend && go test -tags integration ./internal/smb -run TestSMB`
- Result:
  - pass/fail:
  - log summary:

## 5. Unified Publication Gate

- Command:
  - `cd apps/backend && ./scripts/check-smb-publication-gates.sh`
- Result:
  - pass/fail:
  - log summary:

## 6. Migration / Rollback References

- Migration guide link:
- Rollback guide link:
- Parity checklist result link:

## 7. Reviewer Sign-off

- Reviewer:
- Approval date:
- Notes:
