## Why

현재 `Settings > 내 프로필`은 로그인한 사용자의 계정 정보를 읽기 전용으로만 보여주고 있어, 닉네임 정정이나 비밀번호 변경을 위해 관리자용 계정 관리 경로에 의존해야 한다. `profile.read/profile.write` 권한은 이미 정의돼 있지만 실제 API와 UI가 연결돼 있지 않아, 자기관리 경로를 지금 완성하는 편이 제품 완성도와 권한 경계를 함께 정리하기에 적절하다.

## What Changes

- `Settings > 내 프로필`에서 닉네임을 수정할 수 있는 편집 UI를 추가한다.
- 로그인한 사용자가 자신의 비밀번호를 변경할 수 있도록 자기수정 API를 추가한다.
- 비밀번호 변경 시 `현재 비밀번호` 입력을 필수로 검증하고, 초기 admin setup 흐름은 그대로 유지한다.
- 자기수정 성공 후 세션 정보를 다시 읽어 프로필 화면과 전역 사용자 상태를 최신 값으로 동기화한다.
- 자기수정 경로에 `profile.write` 권한과 대응 감사 액션을 연결하고, 성공/실패 테스트를 추가한다.

## Capabilities

### New Capabilities
- `self-profile-management`: 로그인한 사용자의 자기 프로필 조회/수정과 현재 비밀번호 확인 기반 비밀번호 변경 계약을 정의한다.

### Modified Capabilities
- `frontend-operational-regression-coverage`: 프로필 화면의 편집/검증 경로를 프론트 회귀 테스트 범위에 포함한다.

## Impact

- Frontend: `apps/frontend/src/pages/Settings/sections/ProfileSettings.tsx`, `apps/frontend/src/features/auth/AuthContext.tsx`, `apps/frontend/src/api/auth.ts`
- Backend: `apps/backend/internal/auth/handler.go`, `apps/backend/internal/auth/service.go`, `apps/backend/internal/auth/permissions.go`, `apps/backend/internal/account/service.go`
- Tests: 프로필 화면, 인증 컨텍스트, auth/account handler/service, 권한 매핑 테스트
- OpenSpec/docs: 자기 프로필 관리 capability spec, 회귀 테스트 capability delta, AI context 문서
