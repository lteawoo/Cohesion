## 1. Config Contract

- [x] 1.1 백엔드 설정 모델과 `/api/config` 응답/저장 계약에 `auditLogRetentionDays` 필드를 추가한다.
- [x] 1.2 서버 설정 화면에 감사 로그 보존 일수 입력과 validation/안내 문구를 추가한다.
- [x] 1.3 보존 일수 `0 = 무기한 보관` 규칙을 프론트/백엔드 모두에서 일관되게 검증한다.

## 2. Audit Backend

- [x] 2.1 감사 로그 export용 필터 재사용 경로와 CSV 응답 formatter를 구현한다.
- [x] 2.2 감사 로그 cleanup service/store 경로를 추가해 retention policy 기준 삭제 건수를 반환한다.
- [x] 2.3 감사 로그 export/cleanup API endpoint와 권한 매핑을 추가한다.
- [x] 2.4 cleanup 성공/실패/denied를 `audit.logs.cleanup` 이벤트로 기록한다.

## 3. Audit Frontend

- [x] 3.1 감사 로그 화면에 현재 필터 기준 CSV 내보내기 액션을 추가한다.
- [x] 3.2 감사 로그 화면에 현재 retention policy 표시와 cleanup 확인 모달을 추가한다.
- [x] 3.3 cleanup 성공 후 목록 재조회와 삭제 건수 피드백을 연결한다.
- [x] 3.4 권한에 따라 export/cleanup 액션 노출을 분기한다.

## 4. Verification

- [x] 4.1 백엔드 export/cleanup/config 테스트를 추가한다.
- [x] 4.2 프론트엔드 감사 로그/서버 설정 운영 경로 테스트를 추가한다.
- [x] 4.3 OpenSpec 및 `docs/ai-context` 문서를 현재 결정사항으로 갱신한다.
