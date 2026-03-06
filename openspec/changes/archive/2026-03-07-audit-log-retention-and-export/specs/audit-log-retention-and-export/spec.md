## ADDED Requirements

### Requirement: 감사 로그 CSV 내보내기
시스템은 감사 로그 화면에서 사용한 필터 조건을 그대로 적용해 감사 로그를 CSV 형식으로 내보내기 SHALL 한다.

#### Scenario: 필터 결과를 CSV로 내보낸다
- **WHEN** 관리자가 기간, 사용자, 액션, Space, 결과 필터를 적용한 뒤 CSV 내보내기를 실행한다
- **THEN** 시스템은 동일한 필터 조건을 적용한 감사 로그를 CSV 파일로 반환한다

#### Scenario: 조회 권한 없는 사용자는 CSV를 내보낼 수 없다
- **WHEN** `account.read` 권한이 없는 사용자가 감사 로그 CSV 내보내기 endpoint를 호출한다
- **THEN** 시스템은 권한 오류를 반환하고 export를 수행하지 않는다

### Requirement: 감사 로그 보존 일수 시스템 설정
시스템은 감사 로그 보존 일수를 시스템 설정으로 조회 및 저장 SHALL 하며, `0` 값은 무기한 보관으로 해석 MUST 한다.

#### Scenario: 보존 일수를 조회한다
- **WHEN** 운영자가 시스템 설정을 조회한다
- **THEN** 응답에는 현재 감사 로그 보존 일수 값이 포함된다

#### Scenario: 보존 일수를 무기한 보관으로 설정한다
- **WHEN** 운영자가 감사 로그 보존 일수를 `0`으로 저장한다
- **THEN** 시스템은 감사 로그를 자동 정리 대상으로 간주하지 않는다

### Requirement: 보존 정책 기반 수동 cleanup
시스템은 저장된 감사 로그 보존 일수를 기준으로 수동 cleanup을 실행 SHALL 하며, 실행 결과를 운영자에게 반환 MUST 한다.

#### Scenario: 보존 일수 기준으로 오래된 로그를 정리한다
- **WHEN** `account.write` 권한을 가진 관리자가 cleanup을 실행하고 보존 일수가 30일로 설정되어 있다
- **THEN** 시스템은 30일보다 오래된 감사 로그만 삭제하고 삭제 건수를 응답한다

#### Scenario: 보존 일수가 비활성화된 경우 cleanup을 거절한다
- **WHEN** 관리자가 cleanup을 실행했지만 감사 로그 보존 일수가 `0`이다
- **THEN** 시스템은 cleanup을 수행하지 않고 보존 정책이 비활성화되었다는 오류를 반환한다

#### Scenario: cleanup 실행을 감사 이벤트로 남긴다
- **WHEN** cleanup 요청이 성공하거나 실패한다
- **THEN** 시스템은 `audit.logs.cleanup` 액션과 결과, 보존 일수, 삭제 건수 요약을 포함한 감사 이벤트를 기록한다
