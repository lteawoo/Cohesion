## MODIFIED Requirements

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
