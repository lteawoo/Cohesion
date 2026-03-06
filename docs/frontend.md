# frontend

## 목적

프론트엔드는 Cohesion의 웹 운영 화면이다. 로그인, 스페이스 브라우징, 검색, 상태 표시, 설정 화면을 제공하며 개발 환경에서는 Vite dev server로 실행된다.

## 기술 스택

- React 19
- TypeScript
- Vite 7
- Ant Design 6
- React Router 7
- Zustand
- i18next
- Vitest + Testing Library

## 현재 구조

```text
apps/frontend/src/
├── api/                  # 백엔드 REST API 래퍼
├── assets/               # 글로벌 CSS 등 정적 자산
├── components/           # 레이아웃/공용 UI 컴포넌트
│   ├── common/
│   └── layout/
├── features/             # 기능별 모듈
│   ├── auth/             # 인증 컨텍스트/가드
│   ├── browse/           # 스페이스 브라우저, 파일 작업, 휴지통
│   ├── search/           # 검색 응답 타입/훅/유틸
│   ├── space/            # 스페이스 타입/공유 상태
│   └── status/           # 서버 상태/버전 표시
├── i18n/                 # 다국어 리소스
├── pages/                # 라우트 단위 화면
│   ├── Login/
│   └── Settings/
├── stores/               # 전역 상태 저장소
├── test/                 # 테스트 부트스트랩
├── theme/                # 테마 설정
├── App.tsx
├── RootProviders.tsx
└── main.tsx
```

## 화면 경계

- `Login`
  - 초기 admin setup 여부 확인
  - 로그인/리다이렉트 처리
- 메인 레이아웃
  - 사이드바 스페이스 목록
  - 헤더 검색
  - 브라우즈/상태 진입점
- `Settings`
  - 내 프로필
  - 일반 설정
  - 화면 설정
  - 서버 설정
  - 스페이스 관리
  - 권한 관리
  - 계정 관리
  - 감사 로그
  - 정보

## 상태 관리 기준

- 인증/세션: `features/auth`
- 스페이스 목록과 선택 상태: `stores/spaceStore.ts`
- 브라우즈 상태와 파일 목록: `stores/browseStore.ts`
- 설정/테마/컨텍스트 메뉴는 별도 store로 분리

새 UI를 추가할 때는 가능한 한 기존 store와 feature 경계를 재사용하고, 화면 전용 상태만 컴포넌트 로컬 state로 유지한다.

## API 연동 기준

- 프론트는 `/api/*` 백엔드 REST 경로를 호출한다.
- 개발 환경에서는 Vite가 `/api`를 백엔드로 프록시한다.
- API 호출 코드는 `src/api/*`에 두고, 화면 컴포넌트에서 직접 `fetch`를 중복 구현하지 않는다.

## 테스트/검증 명령

```bash
# 프론트 개발 서버
pnpm --dir apps/frontend dev

# 타입체크
pnpm --dir apps/frontend typecheck

# 테스트
pnpm --dir apps/frontend test

# 린트
pnpm --dir apps/frontend lint

# 프로덕션 빌드
pnpm --dir apps/frontend build
```

## 구현 규칙 메모

- 타입 정의는 현재 코드 규칙대로 `interface`와 `type`을 구분해 사용한다.
- 공용 API 계약은 `src/api/*`, 기능 로직은 `src/features/*`, 라우트 조합은 `src/pages/*`에 둔다.
- 브라우즈, 검색, 설정처럼 운영 핵심 경로를 바꿀 때는 회귀 테스트를 함께 추가하는 것을 기본으로 본다.
- UI 변경은 실제 렌더 기준으로 확인하고, 기존 Ant Design 패턴과 현재 스타일 변수 체계를 유지한다.

## 문서 유지보수 체크리스트

- `src/features/*`, `src/pages/*`, `src/stores/*`의 안정적인 경계가 바뀌면 `docs/frontend.md`를 같이 갱신한다.
- 로그인, 메인 레이아웃, 설정 화면의 라우트/권한/섹션 구조가 바뀌면 "화면 경계" 설명을 다시 확인한다.
- 프론트 실행, 테스트, 빌드 명령이 바뀌면 이 문서의 "테스트/검증 명령"을 먼저 맞춘다.
