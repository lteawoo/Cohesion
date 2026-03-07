# 의사결정 로그 (Decision Log)

## 아키텍처 (Architecture)

### self-update는 현재 launch mode를 전달해 interactive 실행을 유지한다 (2026-03-07)
- 상황:
  - 일반 재시작은 같은 프로세스 루프 안에서 처리돼 처음 터미널/콘솔 맥락이 유지되지만, self-update는 updater가 새 앱을 `app.log` 기반 백그라운드 성격으로 다시 띄웠다.
  - 사용자는 새 창/새 콘솔은 허용하지만, 수동 실행한 앱이 조용히 백그라운드 앱처럼 바뀌는 경험을 원하지 않았다.
- 결정:
  - self-update 시작 시 현재 프로세스의 launch mode를 terminal attachment 기준으로 판별한다.
  - updater에는 `launch-mode` 인자를 넘기고, interactive 모드에서는 새 앱과 rollback 앱을 현재 `stdout/stderr` 상속으로 재기동한다.
  - background 모드에서는 기존 `logs/app.log` 리다이렉트 방식을 유지한다.
- 검증 메모:
  - updater handoff를 직접 실행한 macOS 수동 smoke test에서 `launch-mode=interactive`일 때 updater 터미널에 새 앱의 부팅/종료 로그가 출력되는 것을 확인했다.
- 이유:
  - self-update 이후에도 사용자가 인지하는 실행 모델을 유지해야 일반 재시작과의 차이를 줄이고, 포터블 수동 실행 시 UX 일관성을 확보할 수 있기 때문이다.

### self-update 전환 성공은 새 바이너리 health/version probe 통과 후에만 인정한다 (2026-03-07)
- 상황:
  - 현재 self-update는 업데이터가 replacement 바이너리를 `cmd.Start()`만 하면 성공 경로로 넘어갔다.
  - 실제 새 프로세스가 포트를 열지 못하거나 즉시 종료되면 서비스가 중단된 채 남을 수 있었다.
- 결정:
  - 업데이터는 새 바이너리 재기동 후 loopback `/api/health`와 `/api/system/version` probe가 모두 통과해야만 전환 성공으로 간주한다.
  - probe 실패 시 replacement 프로세스를 중단하고 `.bak`로 롤백한 뒤 이전 바이너리 재기동까지 검증한다.
- 이유:
  - 바이너리 교체 성공과 서비스 재기동 성공을 분리해 판단해야 운영 안정성을 확보할 수 있기 때문이다.

### update/restart lifecycle 상태는 실행 파일 기준 data 디렉터리에 저장한다 (2026-03-07)
- 상황:
  - update status는 프로세스 메모리에만 있어 self-update나 restart 이후 이력이 사라졌다.
  - config 디렉터리에 runtime 상태를 쓰면 개발 환경에서 저장소 추적 파일과 섞일 수 있었다.
- 결정:
  - lifecycle 상태는 실행 파일 기준 `data/system-status.json`에 저장한다.
  - status 표면은 마지막 전환 상태와 현재 runtime 상태를 함께 노출한다.
- 이유:
  - 프로세스 교체 뒤에도 상태를 복구할 수 있어야 하고, runtime 산출물은 설정 파일과 분리하는 편이 운영/개발 모두에서 더 안전하기 때문이다.

### restart API는 완료가 아니라 accepted semantics를 기준으로 노출한다 (2026-03-07)
- 상황:
  - 기존 `/api/system/restart`는 실제 재기동 완료 전에도 성공 응답과 success audit을 먼저 남겼다.
- 결정:
  - `/api/system/restart`는 `202 Accepted`와 `accepted` 상태를 반환한다.
  - 프론트는 `요청 수락 -> 재연결 대기 -> 재연결 확인` 흐름으로 UX를 정리한다.
  - 감사 이벤트는 `system.restart.accepted`, `system.restart.completed`, `system.restart.failed`로 구분한다.
- 이유:
  - API 의미와 실제 시스템 상태, 운영 로그 해석을 일치시켜 오해를 줄이기 위함이다.

### 로컬 UI 시각 검증은 루트 Playwright + js_repl 조합을 기준으로 사용한다 (2026-03-07)
- 상황:
  - 프로젝트 작업 규칙상 UI 변경 검증에는 `playwright-interaction` 기반 실제 렌더 확인이 필요하다.
  - 현재 루트 워크스페이스에는 `playwright` 패키지가 없어 스킬을 바로 실행할 수 없었다.
- 결정:
  - 모노레포 루트에 `playwright`를 devDependency로 추가한다.
  - `js_repl`에서 `import('playwright')`와 headed Chromium launch를 기준으로 환경 준비 여부를 확인한다.
- 이유:
  - 프론트엔드 단일 앱 디렉터리에만 의존성을 두면 루트 기준 Codex 세션과 스킬 실행 경로가 어긋날 수 있다.
  - 루트 기준으로 설치해야 `playwright-interactive` 스킬을 바로 재사용하면서 UI 회귀 검증을 일관되게 수행할 수 있기 때문이다.

### 네트워크 파일 공유 범위 축소 및 코드 경계 단순화 (2026-03-06)
- 상황:
  - 제품 범위를 현재 운영 대상에 맞게 단순화할 필요가 있었다.
- 결정:
  - 미사용 공유 경로와 관련 운영 자산을 정리한다.
  - 상태/설정/런타임 표면을 현재 지원 범위 중심으로 유지한다.
- 이유:
  - 유지보수 비용과 회귀 위험을 줄이고 배포 안정성을 높이기 위함이다.

### 배포 파이프라인은 현재 코드베이스와 동일한 검증 경로만 실행 (2026-03-06)
- 상황:
  - 워크플로우 단계와 실제 코드베이스 사이에 불일치가 있었다.
- 결정:
  - CI/Release에서 현재 저장소에 존재하는 검증 단계만 유지한다.
- 이유:
  - 파이프라인 신뢰성과 운영 가시성을 확보하기 위함이다.

### 스페이스 관리는 Settings에서 운영하고 생성은 사이드바 빠른 진입으로 유지 (2026-03-06)
- 상황:
  - 스페이스 이름 변경과 쿼터 관리를 추가하면서 생성 진입점까지 Settings로 옮길지 결정이 필요했다.
- 결정:
  - `Settings > Spaces`는 이름 변경과 쿼터 관리 중심의 운영 화면으로 확장한다.
  - 스페이스 생성은 기존 사이드바 `+` 진입을 유지한다.
  - `space.write` 권한은 생성/삭제/이름 변경/쿼터 수정에 그대로 사용한다.
- 이유:
  - 빠른 생성 동선을 유지하면서도 운영 관리 맥락은 Settings에 모아 복잡도를 낮추기 위함이다.

### 스페이스 이름 변경 후 shared state는 Space ID 기준으로 재동기화 (2026-03-06)
- 상황:
  - rename 후 사이드바 목록과 browse 선택 상태가 서로 다른 Space 객체를 참조할 수 있었다.
- 결정:
  - Space list refresh 이후 browse `selectedSpace`를 Space ID 기준으로 재결합한다.
  - Settings의 quota 변경도 동일하게 shared Space list refresh를 수행한다.
- 이유:
  - rename/quota 변경 직후에도 사이드바, 브라우즈, 설정 화면이 같은 Space 스냅샷을 보도록 보장하기 위함이다.

### 스페이스 설정은 단일 테이블에서 행별 저장/삭제로 관리 (2026-03-06)
- 상황:
  - `Settings > Spaces`에서 이름 수정과 쿼터 수정, 삭제를 한 화면에서 더 빠르게 처리할 필요가 있었다.
- 결정:
  - Space 이름과 쿼터는 같은 테이블 행에서 편집한다.
  - 각 행의 `작업` 컬럼에 `저장`, `삭제`를 둔다.
  - 삭제는 같은 행에 두되 확인 모달을 반드시 거친다.
  - `space_desc`는 프론트/백엔드/DB 마이그레이션까지 포함해 완전히 제거한다.
- 이유:
  - 실제 사용자가 한 명인 테스트 단계에서는 일괄 저장보다 행별 저장이 단순하고, 여러 Space를 한 번에 비교/수정하는 운영 효율도 높기 때문이다.

### 완료된 OpenSpec change는 메인 spec 동기화 후 archive한다 (2026-03-06)
- 상황:
  - `manage-space-settings` 구현과 머지가 끝나 OpenSpec change를 닫을 시점이 되었다.
- 결정:
  - change의 delta spec를 `openspec/specs/space-settings-management/spec.md`로 동기화한 뒤 archive로 이동한다.
  - 릴리즈 전에는 `pnpm release:check`로 GoReleaser 구성을 다시 검증한다.
- 이유:
  - 구현 이력이 메인 spec에 남아야 후속 변경의 기준선이 일관되고, 태그 릴리즈 전에 배포 경로의 설정 오류를 미리 차단할 수 있기 때문이다.

### 프론트 회귀 테스트는 화면별 file-local mock으로 고정한다 (2026-03-06)
- 상황:
  - `#201`에서 로그인, 설정 페이지, 메인 레이아웃, 서버 설정 테스트를 한 번에 보강해야 했다.
- 결정:
  - 공용 거대 render helper를 도입하지 않고 각 테스트 파일에서 `vi.mock`과 작은 render helper만 사용한다.
  - `MainLayout`은 내부 `HeaderSearch`를 export하지 않고 상위 렌더로 테스트한다.
  - `ServerSettings`는 전체 재시작 시뮬레이션 대신 로드/검증/저장/confirm 기반 핵심 분기만 고정한다.
- 이유:
  - 현재 테스트 스타일과 가장 잘 맞고, 테스트 인프라 리팩터링 없이 핵심 운영 경로의 회귀 감지력을 빠르게 높일 수 있기 때문이다.

### 자기 프로필 수정은 `PATCH /api/auth/me`와 현재 비밀번호 확인으로 분리한다 (2026-03-07)
- 상황:
  - `profile.read/profile.write` 권한은 이미 존재하지만 실제 자기수정 API와 UI는 비어 있었다.
  - 관리자용 `PATCH /api/accounts/{id}`를 일반 사용자 자기수정에 재사용하면 `account.write` 경계가 흐려진다.
- 결정:
  - 자기 프로필 수정은 `PATCH /api/auth/me`로 분리한다.
  - 닉네임 수정은 현재 비밀번호 없이 허용한다.
  - 비밀번호 변경은 `currentPassword`를 필수로 요구하고 새 비밀번호는 8자 이상으로 검증한다.
  - 비밀번호 변경 입력은 메인 프로필 카드에 상시 노출하지 않고, 별도 `비밀번호 변경` 버튼으로 여는 모달에 둔다.
  - 초기 admin setup 흐름은 그대로 유지한다.
  - 저장 성공 후 프론트는 `refreshSession()`으로 전역 사용자 상태를 다시 읽는다.
- 이유:
  - 자기수정과 관리자 계정 관리를 분리해 권한 경계를 명확히 하고, 현재 로그인 사용자에 대한 세션/프로필 상태를 즉시 일치시키기 위함이다.

### 파일 검색 1차 개선은 `total` 대신 `hasMore + 더 보기`로 간다 (2026-03-07)
- 상황:
  - `#198`에서 검색 결과 수와 추가 탐색 경로를 제공해야 했지만, 현재 백엔드는 요청마다 각 space를 `WalkDir`로 순회한다.
- 결정:
  - `/api/search/files`는 raw 배열 대신 `items`, `limit`, `hasMore` 객체를 반환한다.
  - 절대 총 개수는 계산하지 않고, `limit + 1`개째 일치 항목 감지로 `hasMore`만 제공한다.
  - 검색 페이지는 `더 보기`로 limit를 단계적으로 늘려 재조회한다.
  - 검색 페이지 상단 summary는 설명형 Alert 대신 count-only 텍스트로 유지한다.
  - 검색 grid view 메타는 날짜를 제외하고 size/space/path 중심으로 유지한다.
  - 헤더 검색 dropdown은 suggestion popover로 간주하고, outside click과 `Escape`에서 닫힌다.
- 이유:
  - 총 개수 계산 비용을 피하면서도 truncation 여부와 추가 탐색 경로를 사용자에게 제공할 수 있기 때문이다.

### 감사 로그 운영 기능 1차는 `CSV export + 시스템 retention + 수동 cleanup`으로 자른다 (2026-03-07)
- 상황:
  - `#199`에서 감사 로그 운영 기능을 보강해야 했지만, JSON export나 자동 삭제까지 포함하면 범위가 빠르게 커질 수 있었다.
- 결정:
  - 감사 로그 export는 CSV만 지원한다.
  - 감사 로그 보존 일수는 `/api/config`의 `auditLogRetentionDays`로 관리한다.
  - 감사 로그 화면은 목록 응답의 `retentionDays`를 사용해 현재 정책을 표시한다.
  - 오래된 로그 정리는 `POST /api/audit/logs/cleanup`으로 수동 실행만 허용한다.
  - cleanup 액션은 `account.write` 권한에만 노출하고, 실행 자체를 `audit.logs.cleanup` 이벤트로 기록한다.
- 이유:
  - 운영자가 바로 쓸 수 있는 경로를 빠르게 제공하면서도, 자동 삭제 같은 회복 불가능한 동작은 뒤로 미뤄 리스크를 줄이기 위함이다.

### 서버 설정 저장은 신규 필드를 부분 업데이트 호환적으로 유지한다 (2026-03-07)
- 상황:
  - `auditLogRetentionDays`를 `/api/config`에 추가한 뒤, 해당 필드를 모르는 기존 클라이언트가 설정 저장 시 보존 정책을 `0`으로 덮어쓸 수 있었다.
- 결정:
  - `PUT /api/config`에서 `auditLogRetentionDays`는 선택 필드로 취급하고, 요청에 없으면 기존 값을 유지한다.
  - 감사 로그 CSV export는 전체 로그를 메모리에 모으지 않고 row streaming으로 응답한다.
  - `server.config.read` 전용 사용자는 서버 설정을 read-only로만 본다.
- 이유:
  - 신규 설정 필드 도입 이후에도 기존 저장 경로와 운영 스크립트가 안전하게 동작해야 하고, 감사 로그 운영 기능이 로그 증가에 따라 조기 병목이 되지 않도록 해야 하기 때문이다.

### 운영 문서는 현재 저장소의 안정적인 경계만 기준선으로 유지한다 (2026-03-07)
- 상황:
  - `docs/frontend.md`, `docs/backend.md`가 실제 feature/package 경계와 어긋나면서 후속 proposal과 구현에서 오래된 명칭이 반복될 위험이 있었다.
- 결정:
  - 운영 문서는 현재 저장소에 실제로 존재하는 안정적인 경계와 지원 표면만 기록한다.
  - 프론트/백엔드 문서에는 변경 시 함께 확인할 최소 유지보수 체크리스트를 남긴다.
  - 문서 기준선 변경은 `docs/ai-context/status.md`, `todo.md` 동기화와 함께 닫는다.
- 이유:
  - 문서 드리프트를 줄이고 다음 세션의 설계/이슈/구현 작업이 현재 코드베이스를 기준으로 시작되게 하기 위함이다.
