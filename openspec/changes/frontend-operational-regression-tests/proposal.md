## Why

프론트엔드 핵심 운영 경로의 테스트 안전망이 아직 조각 단위에 치우쳐 있어 로그인, 설정 권한 분기, 헤더 검색, 서버 설정 같은 실제 운영 동선의 회귀를 조기에 잡기 어렵다. 최근 `#197`로 설정 경로가 확장된 직후라, 다음 기능 작업 전에 프론트 회귀 감지력을 먼저 올릴 시점이다.

## What Changes

- 로그인 화면에 대해 로딩, 초기 setup, 로그인 성공/실패, 리다이렉트 분기 테스트를 추가한다.
- 설정 페이지에 대해 권한별 섹션 노출과 핵심 섹션 전환 테스트를 추가한다.
- 메인 레이아웃 헤더 검색에 대해 debounce, 결과 선택, 검색 페이지 이동, 빈 상태/오류 상태 테스트를 추가한다.
- 서버 설정 화면에 대해 로드 실패, 검증 오류, 저장, 재시작 확인 흐름의 핵심 분기 테스트를 추가한다.
- 기존 테스트 스타일과 모킹 패턴을 유지하면서 flaky 가능성이 큰 경로는 최소 핵심 분기까지만 고정한다.

## Capabilities

### New Capabilities
- `frontend-operational-regression-coverage`: 로그인, 설정, 헤더 검색, 서버 설정의 핵심 운영 경로에 대한 프론트 회귀 테스트 계약

### Modified Capabilities
- None.

## Impact

- 프론트 테스트 스위트와 Vitest 모킹 패턴
- `apps/frontend/src/pages/Login/index.tsx`
- `apps/frontend/src/pages/Settings/index.tsx`
- `apps/frontend/src/pages/Settings/sections/ServerSettings.tsx`
- `apps/frontend/src/components/layout/MainLayout/index.tsx`
- CI에서 실행하는 `pnpm -C apps/frontend test`, `pnpm -C apps/frontend typecheck`
