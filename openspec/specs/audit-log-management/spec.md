# audit-log-management Specification

## Purpose
TBD - created by archiving change add-audit-log-review-mvp. Update Purpose after archive.
## Requirements
### Requirement: 감사 이벤트 표준 스키마 저장
시스템은 주요 변경 액션 처리 시 감사 이벤트를 표준 스키마로 저장 SHALL 한다. 이벤트는 최소한 `occurredAt`, `actor`, `action`, `result`, `target`, `requestId`를 포함 MUST 하며, Space 맥락이 있는 액션은 `spaceId`를 포함 MUST 한다.

#### Scenario: 파일 업로드 성공 이벤트 저장
- **WHEN** 인증된 사용자가 Space 내부 파일 업로드 요청을 성공적으로 완료한다
- **THEN** 시스템은 `action=file.upload`, `result=success`, `actor=<username>`, `spaceId=<id>`, `target=<space-relative-path>`를 포함한 감사 이벤트를 저장한다

#### Scenario: 권한 변경 실패 이벤트 저장
- **WHEN** 관리자 사용자가 역할 권한 변경 요청을 수행했지만 검증 실패로 요청이 실패한다
- **THEN** 시스템은 `action=role.permissions.replace`, `result=failure`, 실패 사유 코드가 포함된 감사 이벤트를 저장한다

### Requirement: 감사로그 기록은 서비스 가용성을 차단하지 않음
시스템은 감사로그 기록 실패가 발생해도 비즈니스 요청의 본래 성공/실패 결과를 변경 MUST NOT 한다.

#### Scenario: 감사 저장소 장애 중 파일 삭제 성공
- **WHEN** 파일 삭제 요청은 정상 처리되었으나 감사 저장소 쓰기에서 오류가 발생한다
- **THEN** API 응답은 파일 삭제 성공 결과를 유지하고, 시스템은 감사 기록 실패 경고를 운영 로그에 남긴다

### Requirement: 민감정보 비저장 정책
시스템은 감사 이벤트 생성 시 민감정보를 저장 MUST NOT 한다. 금지 대상에는 `password`, `token`, `authorization`, 인증 쿠키, 절대경로가 포함 MUST 한다.

#### Scenario: 계정 수정 요청에 비밀번호 필드가 포함된 경우
- **WHEN** 계정 수정 요청 payload에 `password` 값이 포함된다
- **THEN** 감사 이벤트 metadata에는 `password` 값이 저장되지 않고, 변경 필드명만 허용된 형태로 기록된다

#### Scenario: 파일 작업 target 경로 기록
- **WHEN** Space 내부 파일 이동/복사/삭제 이벤트를 기록한다
- **THEN** 감사 이벤트의 target은 Space 상대경로로 기록되고 서버 절대경로는 기록되지 않는다

### Requirement: 감사로그 조회 API 필터/페이지네이션
시스템은 감사로그 조회 API에서 기간, 사용자, 액션, Space, 결과 필터와 페이지네이션을 지원 SHALL 한다.

#### Scenario: 필터 조건 기반 조회
- **WHEN** 관리자가 `from`, `to`, `user`, `action`, `spaceId`, `result` 쿼리로 감사로그를 조회한다
- **THEN** 응답은 모든 조건을 만족하는 이벤트만 포함한다

#### Scenario: 페이지 이동 조회
- **WHEN** 관리자가 `page`와 `pageSize`를 지정해 연속 페이지를 조회한다
- **THEN** 응답은 중복 없이 안정적인 순서(`occurredAt desc`, tie-breaker 포함)로 이벤트를 반환한다

### Requirement: Settings 감사로그 운영 점검 화면
시스템은 Settings 내 감사로그 섹션에서 필터 입력, 목록 확인, 행 상세 확인, 현재 필터 기준 CSV 내보내기, 보존 정책 기반 cleanup 실행을 제공 SHALL 한다.

#### Scenario: 운영자가 실패 이벤트를 점검
- **WHEN** 운영자가 결과 필터를 `failure`로 설정하고 조회를 실행한다
- **THEN** 화면은 실패 이벤트 목록을 표시하고 선택한 행의 상세 metadata를 별도 패널에서 확인할 수 있다

#### Scenario: 운영자가 현재 필터 결과를 CSV로 내보낸다
- **WHEN** 운영자가 감사 로그 화면에서 필터를 적용한 뒤 CSV 내보내기 버튼을 실행한다
- **THEN** 화면은 동일한 필터 조건으로 생성된 CSV 다운로드를 시작한다

#### Scenario: 운영자가 보존 정책으로 cleanup을 실행한다
- **WHEN** 운영자가 감사 로그 화면에서 현재 보존 일수와 삭제 기준 안내를 확인한 뒤 cleanup을 확정한다
- **THEN** 화면은 cleanup 결과 삭제 건수를 안내하고 최신 목록으로 갱신한다

#### Scenario: 운영 권한이 없는 사용자는 cleanup 액션을 사용할 수 없다
- **WHEN** `account.write` 권한이 없는 사용자가 감사 로그 화면에 접근한다
- **THEN** 화면은 조회 기능만 제공하고 cleanup 실행 액션은 노출하지 않는다
