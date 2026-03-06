## Context

현재 감사 로그 기능은 Settings 화면에서 필터 조회와 상세 확인까지만 제공한다. 운영자는 필터 결과를 외부 전달용으로 추출하거나, 누적 로그를 보존 정책 기준으로 정리할 수 없다. 저장소와 조회 API는 이미 있으므로 이번 변경은 기존 감사 로그 흐름을 확장해 `CSV export`, `보존 일수 설정`, `수동 cleanup`을 추가하는 성격이다.

이 변경은 프론트엔드 감사 로그 화면, 서버 설정 화면, 감사 로그 API, 설정 저장 계약을 동시에 건드린다. 특히 cleanup은 파괴적 액션이므로 자동 스케줄러 없이 명시적 실행과 결과 피드백을 우선 제공해야 한다.

## Goals / Non-Goals

**Goals:**
- 현재 감사 로그 필터 조건을 그대로 사용하는 CSV export 계약을 정의한다.
- 시스템 설정에 감사 로그 보존 일수를 저장하고 조회할 수 있게 한다.
- 감사 로그 화면에서 보존 정책 기준 cleanup을 수동 실행할 수 있게 한다.
- cleanup 실행 결과와 denied/success 결과를 운영자가 이해할 수 있는 형태로 안내한다.
- cleanup 실행 자체를 감사 이벤트로 남겨 운영 흔적을 보존한다.

**Non-Goals:**
- JSON export 또는 다중 형식 export 지원
- cron 또는 background worker 기반 자동 삭제
- 외부 SIEM, webhook, streaming 연동
- 감사 로그 스키마 전면 개편 또는 인덱싱/아카이빙 설계

## Decisions

### CSV export는 현재 필터를 재사용하는 GET endpoint로 제공한다
- 결정: 목록 조회와 같은 필터 파라미터를 받는 전용 CSV export endpoint를 추가한다.
- 이유: 사용자는 화면에서 보고 있는 조건 그대로 추출하기를 기대한다. 별도 export 전용 필터 모델을 만들면 UX와 테스트가 중복된다.
- 대안:
  - 목록 API 응답을 프론트에서 CSV로 변환: 페이지네이션 제약 때문에 전체 결과 export와 불일치가 생긴다.
  - JSON/CSV를 함께 지원: 이번 단계에서 요구 대비 복잡도만 늘어난다.

### 보존 일수는 시스템 설정(`/api/config`)에 포함한다
- 결정: 감사 로그 보존 일수는 서버 설정 계약에 숫자 필드로 추가하고, `0` 또는 비어 있는 값은 무기한 보관으로 해석한다.
- 이유: 보존 일수는 화면 단위 옵션이 아니라 시스템 전체 정책이다. 기존 설정 저장 흐름을 재사용하면 별도 저장소나 관리 화면을 만들 필요가 없다.
- 대안:
  - 감사 로그 화면 내부의 로컬 설정: 정책의 중앙 관리가 안 되고 운영자가 값을 찾기 어렵다.
  - 전용 시스템 API 추가: 현재 설정 인프라가 이미 있어 중복이다.

### cleanup은 수동 실행만 허용하고 retention policy를 강제 사용한다
- 결정: cleanup API는 요청자가 임의 cutoff를 넘기지 않고, 서버가 저장된 retention days를 읽어 삭제 대상을 계산한다.
- 이유: 테스트 단계에서는 운영자 수가 적고, 임의 기준 삭제는 실수 가능성이 높다. 중앙 정책 기준으로만 정리하면 예측 가능성이 높아진다.
- 대안:
  - 임의 날짜 직접 입력 cleanup: 더 유연하지만 실수와 감사 추적 리스크가 크다.
  - 자동 스케줄러 도입: 운영 정책이 충분히 검증되기 전에는 과하다.

### 권한은 읽기/운영 액션을 분리한다
- 결정: CSV export는 `account.read`, cleanup은 `account.write`, retention 설정 변경은 `server.config.write`를 요구한다.
- 이유: export는 조회의 연장선이고, cleanup과 정책 변경은 파괴적/운영적 액션이다. 기존 권한 체계와도 잘 맞는다.
- 대안:
  - cleanup도 `account.read` 허용: 파괴적 액션 기준이 너무 낮다.
  - cleanup을 `server.config.write`로 묶기: 감사 로그 운영과 권한 의도가 다소 어긋난다.

### cleanup 실행 자체를 감사 이벤트로 기록한다
- 결정: cleanup 성공/실패/denied를 `audit.logs.cleanup` 계열 액션으로 기록하고, metadata에는 retention days와 deleted count 같은 요약만 남긴다.
- 이유: 감사 로그 정리도 중요한 운영 액션이므로 추적 가능해야 한다. 단, 민감정보 비저장 정책은 그대로 유지한다.
- 대안:
  - cleanup 이벤트 미기록: 가장 중요한 운영 액션 하나가 감사 흔적 없이 사라진다.

## Risks / Trade-offs

- [대량 CSV 응답] → export endpoint는 필터 조건을 그대로 적용하고, 필요 시 서버 스트리밍 방식으로 구현해 메모리 사용을 제한한다.
- [보존 일수 오설정으로 인한 과다 삭제] → cleanup 전에 현재 retention policy와 삭제 기준 시점을 안내하고 확인 모달을 강제한다.
- [cleanup 결과와 목록 화면 불일치] → cleanup 성공 후 목록을 재조회하고 삭제 건수를 별도 메시지로 노출한다.
- [권한 혼동] → export, cleanup, retention 설정을 UI에서 권한별로 명확히 노출/비노출한다.

## Migration Plan

1. 설정 모델과 `/api/config` 응답/저장 계약에 `auditLogRetentionDays`를 추가한다.
2. 감사 로그 서비스/저장소에 export와 cleanup용 조회/삭제 경로를 추가한다.
3. 감사 로그 API에 CSV export endpoint와 cleanup endpoint를 추가한다.
4. 프론트엔드 감사 로그 화면과 서버 설정 화면에 운영 액션을 연결한다.
5. 기존 감사 로그 데이터는 유지하고, retention days가 설정되지 않은 설치는 무기한 보관으로 동작시킨다.
6. rollback 시에는 신규 endpoint와 UI를 제거하되, 이미 삭제된 로그는 복구 대상이 아니므로 cleanup 배포 전 검증을 충분히 수행한다.

## Open Questions

- CSV export가 현재 필터 기준 전체 결과를 내보낼 때 상한을 둘지 여부는 구현 시점에 실제 데이터 규모를 보고 결정해야 한다.
- retention days 최소 허용값을 둘지(`1` 이상, `7` 이상 등)는 운영 경험이 더 필요하다. 1차 구현에서는 `0 = 무기한`, `1 이상 = 활성화` 정도가 현실적이다.
