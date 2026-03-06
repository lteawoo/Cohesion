## 1. Backend Self-Profile API

- [x] 1.1 `PATCH /api/auth/me` 라우트와 요청/응답 계약을 추가한다.
- [x] 1.2 현재 비밀번호 검증과 새 비밀번호 8자 정책을 포함한 자기수정 서비스 로직을 구현한다.
- [x] 1.3 `profile.write` 권한 매핑과 denied 감사 액션을 자기수정 경로에 연결한다.
- [x] 1.4 auth/account 권한 및 handler/service 테스트를 추가한다.

## 2. Frontend Profile Settings

- [x] 2.1 프로필 수정 API 래퍼와 auth 컨텍스트 세션 재동기화 경로를 추가한다.
- [x] 2.2 `ProfileSettings`를 읽기 전용 화면에서 닉네임/비밀번호 편집 UI로 확장한다.
- [x] 2.3 현재 비밀번호, 새 비밀번호, 확인 입력 검증과 read-only 분기를 구현한다.

## 3. Regression Coverage

- [x] 3.1 프로필 화면 저장 성공/실패/검증 경로 테스트를 추가한다.
- [x] 3.2 세션 재동기화와 읽기 전용 권한 분기 테스트를 추가한다.

## 4. Verification And Context

- [x] 4.1 `cd apps/backend && go test ./...`, `pnpm --dir apps/frontend typecheck`, `pnpm --dir apps/frontend test`로 검증한다.
- [x] 4.2 작업 내용과 결정 사항을 `docs/ai-context`에 반영한다.
