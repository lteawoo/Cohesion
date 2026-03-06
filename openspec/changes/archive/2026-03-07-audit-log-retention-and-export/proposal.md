## Why

현재 감사 로그 화면은 조회와 상세 확인까지만 제공해 운영자가 필터 결과를 외부로 전달하거나, 누적 로그를 보존 정책 기준으로 정리할 수 없다. 감사 로그 저장과 필터링 기반은 이미 있으므로, 1차로 `CSV 내보내기 + 시스템 설정의 보존 일수 + 수동 정리 실행`을 추가해 운영 경로를 완성할 시점이다.

## What Changes

- 감사 로그 화면에서 현재 필터 기준 결과를 CSV로 내보내는 기능을 추가한다.
- 시스템 설정에 감사 로그 보존 일수 설정을 추가한다.
- 감사 로그 화면에서 현재 보존 정책을 기준으로 수동 정리 실행 기능을 추가한다.
- 정리 실행은 확인 절차와 결과 안내를 포함하고, 관련 운영 액션을 감사 이벤트로 남긴다.
- 자동 삭제 스케줄링과 JSON export는 이번 변경에서 제외한다.

## Capabilities

### New Capabilities
- `audit-log-retention-and-export`: 감사 로그 CSV 내보내기와 보존 정책 기반 수동 정리 계약을 정의한다.

### Modified Capabilities
- `audit-log-management`: 감사 로그 Settings 화면 요구사항에 export/cleanup 운영 액션을 추가한다.

## Impact

- Frontend: `apps/frontend/src/pages/Settings/sections/AuditLogsSettings.tsx`, `apps/frontend/src/api/audit.ts`, 필요 시 서버 설정 화면
- Backend: `apps/backend/internal/audit/handler.go`, `apps/backend/internal/audit/service.go`, `apps/backend/internal/audit/store/store.go`, 설정 저장 경로
- API: 감사 로그 CSV export endpoint, cleanup endpoint, 설정 응답/저장 계약 변경
- Tests: 감사 로그 handler/service/store, settings UI, 서버 설정/감사 로그 운영 경로 테스트
- OpenSpec/docs: 감사 로그 운영 capability spec, 기존 audit log management spec delta, AI context 문서
