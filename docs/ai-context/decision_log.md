# 의사결정 로그 (Decision Log)

## 아키텍처 (Architecture)
... (기존 내용 유지) ...

## 파일 시스템 브라우징
### 시스템 디렉토리 필터링 (2026-02-03)
... (기존 내용 유지) ...

### GUI 탐색기 설계 (2026-02-03)
- **결정**: Google Drive와 유사한 사용자 경험을 위해 리스트 대신 `Table`과 `Breadcrumb`을 조합한 탐색기 구조 채택.
- **이유**: 단순 리스트보다 많은 정보(크기, 수정일)를 한눈에 보여줄 수 있으며, 복잡한 경로 탐색 시 Breadcrumb이 필수적임.
- **기술 선택**: Ant Design `Table`의 `onRow` 속성을 이용한 더블 클릭 진입 처리, `filepath.Clean` 기반의 경로 상태 관리.

## 데이터 모델
### FileInfo 확장 (2026-02-03)
- **결정**: `FileInfo` 구조체에 `Size` (int64) 및 `ModTime` (time.Time) 추가.
- **이유**: 파일 탐색기의 기본 기능인 '상세 보기' 및 '정렬' 기능을 지원하기 위함.

## Space와 파일 브라우저 연동
### Space 선택 시 자동 경로 이동 (2026-02-04)
- **결정**: Space 메뉴 클릭 시 해당 Space의 `space_path`를 파일 브라우저에 전달하여 자동 이동.
- **이유**: 사용자가 Space를 클릭했을 때 루트 디렉토리가 아닌 Space의 실제 경로에서 탐색을 시작해야 직관적임.
- **구현 방식**:
  - `MainLayout`에서 `selectedSpace`, `selectedPath` 상태 관리 및 `Outlet` context로 전달.
  - `FileExplorer`가 `useOutletContext`로 경로 정보 및 변경 핸들러를 받음.
  - `FolderTree`에 `rootPath` prop 추가: Space 경로가 있으면 해당 경로부터 트리 시작, 없으면 기존처럼 base directories 로드.
- **수정 파일**: `MainLayout/index.tsx`, `MainSider.tsx`, `FileExplorer.tsx`, `FolderTree.tsx`

### UI 레이아웃 통합 (2026-02-04)
- **결정**: `FolderTree`를 `FileExplorer`에서 분리하여 좌측 사이드바(`MainSider`)로 이동.
- **이유**: Space 목록과 FolderTree를 분리할 필요 없이, 좌측 사이드바에서 Space 선택 → FolderTree 탐색이 자연스러운 흐름.
- **레이아웃 변경**:
  - **이전**: 좌측 사이드바(Space 목록) / 중간(FolderTree) / 우측(FolderContent)
  - **이후**: 좌측 사이드바(Space 목록 + FolderTree) / 우측(FolderContent 전체)
- **장점**: 화면 공간 효율적 사용, 직관적인 계층 구조 표시.

### FolderTree 기본 동작 변경 (2026-02-04)
- **결정**: Space가 선택되지 않은 상태에서는 FolderTree가 아무것도 표시하지 않음.
- **이유**: 시스템 루트 디렉토리를 기본으로 표시할 필요 없음. Space 중심의 워크플로우를 강조.
- **구현**: `rootPath`가 없으면 빈 상태 메시지("Space를 선택하세요") 표시, base directories 자동 로드 제거.

### Space와 FolderTree 완전 통합 (2026-02-04)
- **결정**: Space 메뉴와 FolderTree를 하나의 통합된 트리 구조로 병합.
- **이유**:
  - Space 목록과 폴더 탐색을 별도로 보여줄 필요 없음.
  - 단일 계층 구조로 UX 단순화 및 일관성 향상.
  - 모든 Space를 한눈에 보면서 각 Space 내부를 탐색 가능.
- **구현**:
  - `FolderTree`에 `spaces` prop 추가.
  - Spaces를 `space-{id}` 형태의 key를 가진 루트 노드로 생성.
  - Space 노드 확장 시 `space_path`로부터 하위 디렉토리 lazy loading.
  - `MainSider`에서 Menu 제거, Spaces 헤더 + FolderTree만 표시.
  - Space 선택 시 `space_path` 반환하여 FolderContent 업데이트.
- **특수 케이스**: `showBaseDirectories` 플래그로 모달에서는 시스템 디렉토리 탐색 가능.

## 개발 프로세스
### Status WEB 접근 포트 계산 책임 분리 (2026-02-14)
- **문제**: WEB 포트(`5173`)를 백엔드에서 고정 계산하면 Vite 포트 변경(`--port`)이나 실행 환경 차이를 반영하지 못함.
- **결정**:
  - 백엔드 `/api/status`에서 WEB 포트 하드코딩 계산 로직을 제거한다.
  - 상태 UI의 웹 접근 주소는 프론트 런타임(`window.location.origin`) 기준으로 표시한다.
  - WebDAV/API 포트는 계속 백엔드 설정 포트(`config.server.port`)를 사용한다.
- **이유**:
  - 웹 접근 주소 판단 책임은 현재 페이지를 서빙하는 프론트 런타임에 두는 것이 정확하다.
  - 백엔드와 프론트의 역할 경계를 명확히 유지하며 포트 변경 내성을 확보할 수 있다.

### Status 팝오버의 HTTP 경로 표기 정정 (2026-02-14)
- **문제**: 서버 상태 팝오버에서 HTTP 항목이 `:3000/api/`로 표시되어, 실제 웹 접근 주소 정보와 혼동을 유발.
- **결정**:
  - `/api/status`의 HTTP 프로토콜 경로를 `/`로 반환한다.
  - 프론트 팝오버의 HTTP 라벨을 `WEB`으로 표기하고, 접근 주소는 `http://{host}/`로 표시한다.
- **이유**:
  - 상태 패널의 목적을 API 엔드포인트 안내보다 실제 웹 접근 안내에 맞춘다.
  - 개발/프로덕션 모두에서 사용자가 접근 가능한 URL을 직관적으로 확인할 수 있다.

### Table 뷰 스크롤 컨테이너 분리 (2026-02-13)
- **문제**: FolderContent의 Table 뷰에서 세로 스크롤이 생성되지 않아 하단 파일 목록 접근이 어려움.
- **결정**:
  - `FolderContent` 루트에 `minHeight: 0`을 적용한다.
  - Table 뷰를 별도 래퍼(`flex:1`, `overflowY:auto`)로 감싸 스크롤 축을 명시적으로 분리한다.
- **이유**:
  - Grid 뷰와 동일한 스크롤 전략으로 일관성 확보.
  - 부모/자식 flex 레이아웃에서 높이 계산 이슈로 인한 스크롤 누락 방지.

### Space 생성 모달 입력 상태 분리 + 설명 저장 연동 (2026-02-13)
- **문제**:
  - Space 생성 모달의 이름/설명 입력이 전역 `spaceStore.isLoading`에 묶여 비활성화될 수 있음.
  - 모달의 설명 입력값(`spaceDesc`)이 실제 Space 생성 요청에 포함되지 않아 저장되지 않음.
- **결정**:
  - 모달 입력 비활성 기준을 전역 로딩 상태에서 로컬 제출 상태(`isCreating`)로 분리한다.
  - `spaceStore.createSpace`에 `description` 파라미터를 추가하고, 요청 payload에 `space_desc`를 포함한다(비어있으면 제외).
- **이유**:
  - 전역 상태 결합을 줄여 모달 입력 UX를 안정화.
  - 사용자가 입력한 설명이 실제 데이터로 저장되도록 일관성 확보.

### 싱글/멀티 다운로드 분기 정책 (2026-02-13)
- **문제**: 상단 선택바의 다운로드 액션이 선택 개수와 무관하게 `download-multiple`를 호출해 단일 파일도 ZIP으로 내려갈 수 있었음.
- **결정**:
  - 단일 선택(1개)은 단일 다운로드 API(`/files/download`)를 사용한다.
  - 다중 선택(2개 이상)만 멀티 다운로드 API(`/files/download-multiple`)로 ZIP을 생성한다.
  - 백엔드 `download-multiple`에도 1개 요청 보호 분기를 둬 단일 파일은 ZIP 없이 스트리밍한다.
- **이유**:
  - 사용자 기대와 일치: 파일 1개 다운로드 시 즉시 원본 파일을 받는 것이 자연스러움.
  - 하위 호환성: 기존 멀티 선택 ZIP 다운로드 UX는 유지.
  - 안정성: 프론트 분기 누락 시에도 백엔드가 의도치 않은 ZIP 생성을 방지.

### antd 정적 message 사용 제거 (2026-02-13)
- **문제**: `message` 정적 API 사용 시 동적 테마 컨텍스트를 소비하지 못해 경고 발생.
- **결정**: 정적 `message`/`Modal.confirm` 호출 대신 `App.useApp()`에서 제공되는 `message`/`modal` 인스턴스를 사용.
- **구현**:
  - `/settings` 라우트의 `ConfigProvider` 하위에 `App` 프로바이더 추가.
  - `AdvancedSettings`, `ServerSettings`, `DirectorySetupModal`, `DestinationPickerModal`, `useDragAndDrop`를 컨텍스트 기반 API로 전환.
- **이유**:
  - Ant Design 권장 패턴과 일치.
  - 테마/컨텍스트 반영 가능한 일관된 메시지 렌더링 확보.
  - 콘솔 경고 제거로 디버깅 신뢰도 개선.

### Folder Explorer Grid 자동 컬럼 배치 + 가로 스크롤 억제 (2026-02-13)
- **결정**: Grid 뷰를 브레이크포인트 고정 컬럼이 아닌 `auto-fit + minmax` 기반 자동 컬럼 배치로 전환하고, 가로 스크롤 억제를 위해 스크롤 축을 분리한다.
- **이유**:
  - 해상도별 고정 단계(예: 6/4/2)보다 연속적인 폭 적응이 가능해 공간 활용이 더 좋음.
  - 카드 최소 폭을 유지하면서 가능한 한 많은 항목을 한 줄에 표시할 수 있음.
  - 가로 스크롤의 주 원인(중첩 overflow, toolbar/selection bar 비랩핑) 제거.
- **구현**:
  - `FolderContentGrid`: CSS Grid `gridTemplateColumns: repeat(auto-fit, minmax(180px, 1fr))` 적용.
  - `FileExplorer`: 외곽 `overflow: hidden`.
  - `FolderContent` Grid 컨테이너: `overflowY: auto`, `overflowX: hidden`, `minWidth: 0`.
  - `FolderContentToolbar`, `FolderContentSelectionBar`: `flexWrap` 및 `AntSpace wrap` 적용.
  - 썸네일 표시: `ImageThumbnail`을 `object-fit: cover` 중심으로 변경하고 프리뷰 박스(`128px`)에 밀착되게 렌더링.
  - 밀도 조정: Grid `gap`을 16px → 12px, 카드 body padding을 16px → 12px으로 축소.

### 트리 targeted invalidation 적용 (2026-02-13, #35)
- **문제**: 파일 작업 후 트리를 전역 invalidate하여 불필요한 노드 재초기화/재로딩이 발생.
- **결정**: invalidate payload에 영향 경로를 포함하고, 트리는 해당 노드만 부분 무효화.
- **구현**:
  - `browseStore`: `treeInvalidationTargets` 추가, `invalidateTree(targets?)`로 확장.
  - `useFileOperations`: 액션별 영향 경로 계산.
    - 이름변경/삭제: 소스 부모 경로.
    - 폴더생성/업로드: 대상(부모) 경로.
    - 이동: 소스 부모 + 대상 경로.
    - 복사: 대상 경로.
  - `FolderTree`: target key 해석 후 해당 노드 children/loadedKeys만 초기화하고 필요 시 재로딩.
- **이유**:
  - 영향 없는 트리를 유지해 UX 안정성 개선.
  - 모달 트리와 사이드바 트리의 갱신 정책 일관성 확보.
  - 기존 전역 리프레시 API와의 하위 호환 유지(`targets` 미전달 시 legacy fallback).

### Serena MCP 필수 사용 (2026-02-04)
- **결정**: 모든 코드 탐색 및 수정 작업에서 Serena MCP 툴을 필수로 사용.
- **이유**:
  - 토큰 효율성: 파일 전체가 아닌 필요한 심볼만 읽어 토큰 절약.
  - 정확성: 심볼 단위 수정으로 실수 방지.
  - 일관성: 모든 AI 모델이 동일한 방식으로 코드 작업.
- **금지**: `Read`/`Edit` 툴로 코드 파일 읽기/수정.
- **필수**: `get_symbols_overview` → `find_symbol` → `replace_symbol_body` 워크플로우.
- **문서화**: `CLAUDE.md`에 상세 가이드 추가.

### Playwright MCP 브라우저 테스트 필수화 (2026-02-04)
- **결정**: UI 수정 시 Playwright MCP로 브라우저 직접 테스트 필수.
- **이유**: 스크린샷으로 UI 직접 확인하여 디자인 적합성 판단.
- **절차**: 수정 → 테스트 → 스크린샷 → 브라우저 종료.
- **로그 위치**: `.playwright-mcp/` (gitignore 처리됨).

### 실행 환경 문서화 (2026-02-04)
- **결정**: AI 에이전트를 위한 실행 환경 정보를 별도 문서로 분리.
- **이유**:
  - 서버 실행, 빌드, 포트 등 실행 관련 정보가 분산되어 있음.
  - AI가 매번 프로젝트 구조를 탐색하는 비효율 방지.
  - 환경 설정 변경 시 한 곳만 수정하면 됨.
- **구현**:
  - `docs/AGENTS.md` 생성: 프로젝트 구조, 서버 실행, 빌드, 포트, 테스트 등 통합.
  - CLAUDE.md, GEMINI.md에 AGENTS.md 읽기 지시 추가.

### Space 상대 경로 Breadcrumb (2026-02-04)
- **결정**: Space 선택 시 Breadcrumb을 절대 경로 대신 Space 기준 상대 경로로 표시.
- **이유**:
  - 사용자가 Space 내에서의 위치를 직관적으로 파악 가능.
  - 긴 절대 경로(/Users/...) 대신 간결한 경로 표시로 UX 개선.
  - Space 중심 워크플로우 강화.
- **구현**:
  - FolderTree의 onSelect 콜백에 Space 정보 추가.
  - MainLayout에서 selectedSpace state 관리.
  - FolderContent에서 Space 경로를 기준으로 상대 경로 계산.
  - 예: `/Users/twlee/Downloads/folder1` → `Downloads / folder1`
- **수정 파일**: FolderTree.tsx, MainLayout/index.tsx, MainSider.tsx, FileExplorer.tsx, FolderContent.tsx

### 문서 구조 통합 (2026-02-04)
- **결정**: `docs/master_rule.md`를 모든 규칙의 단일 소스로 통합.
- **이유**:
  - 규칙 분산(CLAUDE.md, GEMINI.md, master_rule.md)으로 인한 혼란 방지.
  - AI 모델 간 일관성 확보: 모든 모델이 동일한 규칙 참조.
  - 유지보수 단순화: 한 곳만 수정하면 됨.
- **변경 사항**:
  - `GEMINI.md`: "master_rule.md를 먼저 읽기"만 남김.
  - `CLAUDE.md`: `.claude/CLAUDE.md`로 이동 (Claude Code CLI 전용).
  - `master_rule.md`: Serena MCP, Playwright, 디자인, 커밋 규칙 모두 포함.

### Space 삭제 기능 Context Menu 구현 (2026-02-04)
- **결정**: Space 노드에 Context Menu 방식의 삭제 기능 구현.
- **이유**:
  - 직관적인 UX: 우클릭 대신 "..." 버튼 클릭으로 메뉴 표시.
  - 일관성: Ant Design Tree와 Dropdown 컴포넌트 활용.
  - 안전성: Modal.confirm으로 삭제 확인 절차 추가.
- **구현**:
  - FolderTree: `titleRender`로 Space 노드에만 Dropdown 추가.
  - MainSider: `useDeleteSpace` 훅으로 DELETE API 호출.
  - 삭제 후 `refetch()`로 트리 자동 갱신.
  - 성공/실패 시 message 컴포넌트로 사용자에게 피드백.
- **백엔드**: DELETE `/api/spaces/:id` 엔드포인트는 이미 구현되어 있었음.

### 파일 표시 버그 수정 (2026-02-04)
- **문제**: FolderContent에서 폴더만 표시되고 파일이 표시되지 않는 버그 발견.
- **원인 분석**:
  - `browse_handler.go`의 `handleBrowse` 함수에서 `ListDirectory(true, targetPath)` 호출.
  - `isOnlyDir=true` 파라미터로 인해 `browse/service.go`의 `ListDirectory` 함수가 파일을 필터링.
  - 92-94라인: `if isOnlyDir && !entry.IsDir() { continue }` 로직으로 파일 제외.
- **결정**: `isOnlyDir` 파라미터를 `false`로 변경하여 파일과 폴더 모두 반환.
- **이유**:
  - 파일 탐색기의 핵심 기능은 파일과 폴더를 모두 보여주는 것.
  - FolderContent는 이미 파일과 폴더를 구분하여 표시하는 UI가 구현되어 있음.
  - 왼쪽 FolderTree는 폴더만 표시하고, 오른쪽 FolderContent는 파일과 폴더 모두 표시하는 것이 직관적.
- **수정 파일**: `apps/backend/internal/browse/handler/browse_handler.go:51`
- **결과**: Space 선택 시 폴더와 파일이 모두 정상 표시됨.

### 파일 다운로드 기능 구현 (2026-02-04)
- **결정**: 파일 클릭 시 브라우저의 기본 다운로드 기능을 사용하여 파일을 다운로드.
- **이유**:
  - 단순하고 직관적인 UX: 파일 이름을 클릭하면 바로 다운로드.
  - 브라우저 내장 다운로드 관리 활용: 사용자가 익숙한 방식.
  - 폴더와 파일 동작 구분: 폴더는 클릭 시 이동, 파일은 클릭 시 다운로드.
- **구현**:
  - **백엔드**: `handleDownload` 함수 추가.
    - `/api/browse/download?path=<파일경로>` 엔드포인트.
    - `Content-Disposition: attachment` 헤더로 다운로드 강제.
    - 보안: 디렉토리 다운로드 방지, 파일 존재 여부 및 권한 검증.
    - `io.Copy`로 파일 스트리밍.
  - **프론트엔드**: FolderContent의 render 함수 수정.
    - 폴더: `<a onClick={...}>` (기존 동작 유지)
    - 파일: `<a href="/api/browse/download?path=..." download>`
    - 조건부 렌더링으로 폴더와 파일 구분.
- **대안 검토**:
  - fetch API로 다운로드: 복잡하고 추가 코드 필요, 브라우저 기본 기능보다 장점 없음.
  - 우클릭 메뉴로만 다운로드: 덜 직관적, 추가 클릭 필요.
- **수정 파일**:
  - `apps/backend/internal/browse/handler/browse_handler.go` (handleDownload, RegisterRoutes)
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (render 함수)
- **결과**: 파일 클릭 시 다운로드 정상 작동, README.md 테스트 완료.

### Space 경로 보안 강화 (2026-02-04)
- **문제**: Space로 할당되지 않은 시스템 경로에도 자유롭게 접근 가능한 보안 취약점.
- **요구사항**: Space 경로만 접근 허용하되, Space 생성 시 시스템 탐색은 가능해야 함.
- **결정**: 백엔드에서 경로 검증 수행, Space 생성 모드는 별도 플래그로 구분.
- **이유**:
  - 보안: 사용자가 의도적으로 할당한 Space 외부 경로는 접근 불가.
  - 유연성: Space 생성 시에는 시스템 전체를 탐색할 수 있어야 폴더 선택 가능.
  - 서버 측 검증: 클라이언트 우회 방지, 신뢰할 수 있는 검증.
- **구현**:
  - **백엔드**:
    - Handler에 spaceService 주입 (Space 목록 조회).
    - `isPathAllowed(path)`: 요청 경로가 Space의 하위 경로인지 확인.
      - Space 목록을 가져와서 각 space_path와 비교.
      - `strings.HasPrefix`로 하위 경로 판별.
      - Space가 없으면 모든 경로 허용 (하위 호환성).
    - `handleBrowse`, `handleDownload`에서 경로 검증 수행.
    - `system=true` 쿼리 파라미터로 시스템 탐색 모드 지원.
      - systemMode일 때는 경로 검증 스킵.
  - **프론트엔드**:
    - `useBrowseApi`: `fetchDirectoryContents(path, systemMode=false)` 파라미터 추가.
    - `FolderTree`: `showBaseDirectories`일 때 `systemMode=true` 전달.
- **대안 검토**:
  - 별도 API 엔드포인트 (`/api/browse/system`): 중복 코드, 유지보수 복잡도 증가.
  - 클라이언트 검증: 보안 취약, 우회 가능.
  - Space가 없을 때만 전체 허용: 현재 채택, 단순하고 효과적.
- **보안 고려사항**:
  - Space가 없으면 모든 경로 허용: 초기 사용자 경험 향상, 첫 Space 생성 전까지는 제한 없음.
  - 경로 정규화: `filepath.Clean`으로 `..` 등 우회 시도 방지.
  - 403 에러: 명확한 에러 메시지로 권한 부족 알림.
- **수정 파일**:
  - `apps/backend/internal/browse/handler/browse_handler.go` (isPathAllowed, 검증 로직)
  - `apps/backend/main.go` (spaceService 주입)
  - `apps/frontend/src/features/browse/hooks/useBrowseApi.ts` (systemMode 파라미터)
  - `apps/frontend/src/features/browse/components/FolderTree.tsx` (systemMode 전달)
- **결과**:
  - Space 경로만 접근 가능, 외부 경로는 403 에러.
  - Space 생성 모달에서 시스템 전체 탐색 정상 작동.

### 파일/폴더 우클릭 컨텍스트 메뉴 구현 (2026-02-05)
- **결정**: 파일과 폴더에 우클릭 컨텍스트 메뉴를 추가하고 상위 폴더 버튼 제거.
- **이유**:
  - 현대적인 파일 관리자 UX: 우클릭으로 다양한 작업 접근.
  - UI 단순화: Breadcrumb만으로 충분히 탐색 가능, 별도 버튼 불필요.
  - 파일/폴더 타입별 차별화: 파일과 폴더에 맞는 작업 제공.
- **구현**:
  - **컨텍스트 메뉴 구조**:
    - 파일: "다운로드", "이름 변경", "삭제"
    - 폴더: "이름 변경", "삭제" (다운로드 제외)
  - **상위 폴더 버튼 제거**: Breadcrumb의 상위 경로 클릭으로 대체.
  - **컨텍스트 메뉴 상태 관리**:
    - 마우스 위치(x, y)와 대상 레코드 저장.
    - 메뉴 외부 클릭 시 자동 닫힘 (useEffect + document.addEventListener).
  - **Table onRow 이벤트**: `onContextMenu` 핸들러로 우클릭 감지.
  - **Ant Design Menu**: position: fixed로 마우스 위치에 표시.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
- **결과**: 상위 폴더 버튼 제거, 파일/폴더별 컨텍스트 메뉴 정상 작동.

### 파일/폴더 이름 변경 및 삭제 기능 구현 (2026-02-05)
- **결정**: 백엔드 API와 프론트엔드 모달로 이름 변경 및 삭제 기능 완전 구현.
- **이유**:
  - 파일 관리 기본 기능: 사용자가 파일/폴더를 직접 관리할 수 있어야 함.
  - 안전한 삭제: Modal.confirm으로 실수 방지.
  - 즉시 반영: 작업 후 목록 자동 새로고침으로 UX 향상.
- **구현**:
  - **백엔드 API**:
    - `POST /api/browse/rename`: 파일/폴더 이름 변경.
      - 요청: `{ oldPath, newName }`
      - 보안: 기존 경로와 새 경로 모두 Space 내부인지 검증.
      - `os.Rename` 사용.
    - `POST /api/browse/delete`: 파일/폴더 삭제.
      - 요청: `{ path }`
      - 보안: Space 내부 경로인지 검증.
      - 폴더는 `os.RemoveAll` (하위 파일 포함), 파일은 `os.Remove`.
  - **프론트엔드**:
    - **이름 변경 모달**:
      - 현재 이름을 기본값으로 표시.
      - Input에 Enter 키 입력 시 변경 실행.
      - 성공 시 `message.success`, 실패 시 `message.error`.
      - 작업 후 `fetchDirectoryContents`로 목록 새로고침.
    - **삭제 확인 모달**:
      - `Modal.confirm` 사용.
      - 폴더 삭제 시 "하위 파일 포함 삭제" 경고.
      - 확인 후 DELETE API 호출.
      - 성공 시 목록 자동 새로고침.
    - **컨텍스트 메뉴 onClick**:
      - "이름 변경": 모달 열기, 현재 이름을 state에 설정.
      - "삭제": 확인 모달 표시.
- **보안 고려사항**:
  - 이름 변경 시 새 경로도 Space 내부인지 검증: 경로 탈출 방지.
  - Space 외부 경로 접근 시 403 에러.
  - `filepath.Clean`으로 경로 정규화.
- **수정 파일**:
  - `apps/backend/internal/browse/handler/browse_handler.go` (handleRename, handleDelete)
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (모달, API 호출)
- **결과**: 이름 변경 및 삭제 모두 정상 작동, 목록 자동 새로고침 확인.

### 다운로드 우클릭 전용 변경 (2026-02-05)
- **결정**: 파일 이름 클릭 시 다운로드 제거, 우클릭 메뉴로만 다운로드 가능.
- **이유**:
  - 일관성: 모든 파일 작업을 우클릭 메뉴로 통일.
  - 실수 방지: 의도치 않은 다운로드 방지.
  - 폴더와 일관된 UX: 폴더는 클릭 시 이동, 파일은 클릭 무반응.
- **구현**:
  - 파일 이름을 `<a>` 태그에서 `<span>` 태그로 변경.
  - 다운로드는 컨텍스트 메뉴의 "다운로드" 항목으로만 가능.
  - 파일 아이콘과 이름은 시각적으로만 표시, 클릭 이벤트 없음.
- **대안 검토**:
  - 파일 클릭 시 미리보기: 아직 미리보기 기능 미구현, 추후 고려.
  - 더블 클릭으로 다운로드: 폴더 더블 클릭(이동)과 혼동 가능.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (render 함수)
- **결과**: 파일 클릭 시 다운로드 안 됨, 우클릭 메뉴로만 다운로드 가능.

### 파일 업로드 기능 구현 (2026-02-05)
- **결정**: Drag & Drop 방식의 파일 업로드 기능 구현, 중복 파일 덮어쓰기 확인 포함.
- **이유**:
  - 직관적인 UX: 파일을 드래그하거나 클릭하여 쉽게 업로드.
  - 현대적인 파일 관리자 표준: Google Drive, Dropbox 등 주요 서비스에서 사용.
  - Space 기반 워크플로우와 자연스럽게 통합.
  - 안전한 덮어쓰기: 사용자 확인 후 기존 파일 덮어쓰기 가능.
- **구현**:
  - **백엔드**:
    - `POST /api/browse/upload` 엔드포인트 추가.
    - multipart/form-data로 파일 수신 (최대 32MB).
    - `targetPath` 파라미터로 업로드 대상 디렉토리 지정.
    - `overwrite` 파라미터 (선택): `true`일 때 기존 파일 덮어쓰기.
    - Space 경로 검증: `isPathAllowed`로 대상 경로가 Space 내부인지 확인.
    - 파일 중복 처리:
      - `overwrite=false` (기본값): 동일 이름 파일 존재 시 409 Conflict 에러.
      - `overwrite=true`: 기존 파일 덮어쓰기 (`os.Create`가 자동으로 덮어씀).
    - `os.Create` + `io.Copy`로 파일 저장.
    - 실패 시 부분 저장된 파일 정리 (`os.Remove`).
  - **프론트엔드**:
    - Ant Design `Upload.Dragger` 컴포넌트 사용.
    - `customRequest`로 커스텀 업로드 로직 구현.
    - `performUpload` 함수로 업로드 로직 분리 (재사용 가능).
    - 409 에러 처리:
      - `Modal.confirm`으로 덮어쓰기 확인 모달 표시.
      - 확인 시 `overwrite=true`로 재업로드.
      - 취소 시 업로드 중단.
    - FormData로 파일, targetPath, overwrite 전송.
    - 업로드 성공 시 `message.success` 표시 및 목록 자동 새로고침.
    - `showUploadList: false`로 기본 업로드 목록 숨김 (즉시 새로고침으로 대체).
    - FolderContent 상단 Breadcrumb 아래 배치.
- **보안 고려사항**:
  - Space 경로 검증: Space 외부 경로 업로드 차단 (403 에러).
  - 파일 크기 제한: 32MB (서버 메모리 보호).
  - 덮어쓰기 확인: 사용자 명시적 확인 없이는 덮어쓰기 불가.
- **대안 검토**:
  - 별도 업로드 버튼: Drag & Drop보다 덜 직관적.
  - 무제한 파일 크기: 서버 메모리 고갈 위험.
  - 중복 파일 자동 덮어쓰기: 데이터 손실 위험, 사용자 확인 필수.
  - 중복 파일 자동 이름 변경: 사용자가 예상하지 못한 파일명, 혼란 야기.
- **수정 파일**:
  - `apps/backend/internal/browse/handler/browse_handler.go` (handleUpload, RegisterRoutes)
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (Upload.Dragger, 덮어쓰기 확인 모달 추가)
- **결과**: Drag & Drop 파일 업로드 정상 작동, 중복 파일 덮어쓰기 확인 모달 정상 작동.

### 뷰 전환 기능 구현 (2026-02-05)
- **결정**: 테이블 뷰와 그리드(앨범) 뷰를 전환할 수 있는 기능 구현.
- **이유**:
  - 사용자 선호도: 일부 사용자는 상세 정보를 선호하고, 일부는 시각적 탐색을 선호.
  - 파일 타입별 최적화: 이미지는 그리드가, 문서는 테이블이 더 적합.
  - 현대적인 파일 관리자 표준: Finder, Google Drive, Dropbox 모두 뷰 전환 기능 제공.
  - 유연성: 사용자가 상황에 따라 적합한 뷰 선택 가능.
- **구현**:
  - **뷰 전환 UI**:
    - Button.Group으로 두 버튼 묶음.
    - Breadcrumb 우측에 배치 (justify-space-between).
    - 활성 버튼은 `type="primary"`로 강조.
    - 아이콘: UnorderedListOutlined (테이블), AppstoreOutlined (그리드).
  - **테이블 뷰** (기존):
    - Ant Design Table 컴포넌트.
    - 파일명, 수정일, 크기 정보 표시.
    - 정렬, 더블 클릭 이동, 우클릭 메뉴 지원.
  - **그리드 뷰** (신규):
    - Ant Design Row/Col + Card 컴포넌트.
    - 반응형 그리드: xs=12 (2열), sm=8 (3열), md=6 (4열), lg=4 (6열), xl=3 (8열).
    - 카드 내용:
      - 큰 아이콘 (48px): 폴더(노란색), 파일(회색).
      - 파일 이름 (12px, word-break).
      - 파일 크기 (11px, 파일만).
    - 더블 클릭 이동, 우클릭 메뉴 기능 유지.
    - 중앙 정렬, 카드 호버 효과.
  - **상태 관리**:
    - `viewMode` state ('table' | 'grid').
    - 조건부 렌더링으로 뷰 전환.
    - 기본값: 'table'.
- **디자인 고려사항**:
  - 8px 그리드 시스템 준수: gutter=[16, 16], padding=16px.
  - 일관된 아이콘 색상: 폴더(#ffca28), 파일(#8c8c8c).
  - Card bodyStyle로 padding 조정 (16px 8px).
  - word-break로 긴 파일명 처리.
- **대안 검토**:
  - 리스트 뷰 추가: 테이블과 유사하여 불필요.
  - 아이콘 크기 조정 슬라이더: 복잡도 증가, 우선순위 낮음.
  - 뷰 모드 localStorage 저장: 현재는 세션별 초기화, 추후 고려.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (뷰 전환 버튼, 그리드 뷰 추가)
- **결과**: 테이블 ↔ 그리드 양방향 전환 정상 작동, 그리드 뷰 반응형 정상 작동.

### 업로드 UI 개선 (2026-02-05)
- **결정**: Upload.Dragger 섹션 제거, 전체 영역 드래그 앤 드롭 지원, 업로드 버튼 추가.
- **이유**:
  - 화면 공간 효율성: 별도 업로드 섹션이 화면 공간을 많이 차지.
  - 현대적인 UX: Google Drive, Dropbox 등은 전체 영역 드래그 지원.
  - 직관성: 파일 탐색기 어디에나 드롭 가능.
  - 일관성: 뷰 전환 버튼과 업로드 버튼을 같은 영역에 배치.
- **구현**:
  - **Upload.Dragger 제거**:
    - 기존 Upload.Dragger 섹션 완전 제거.
    - 파일 목록이 화면을 더 넓게 사용.
  - **전체 영역 드래그 앤 드롭**:
    - 최상위 div에 드래그 이벤트 리스너 추가.
    - `onDragEnter`, `onDragLeave`, `onDragOver`, `onDrop`.
    - `isDragging` state로 드래그 상태 관리.
    - 드래그 중 오버레이 표시:
      - 반투명 파란색 배경 (rgba(24, 144, 255, 0.1)).
      - 파란색 점선 테두리 (2px dashed #1890ff).
      - 중앙 정렬된 아이콘 + 메시지 ("파일을 놓아 업로드").
      - z-index: 999, position: absolute.
  - **업로드 버튼**:
    - 위치: Breadcrumb 우측, 뷰 전환 버튼 옆.
    - Space 컴포넌트로 버튼들 그룹화.
    - UploadOutlined 아이콘 + "업로드" 텍스트.
    - 숨겨진 file input (display: none).
    - 버튼 클릭 시 fileInputRef.current?.click().
  - **업로드 로직 통합**:
    - `handleFileUpload` 함수로 드래그/버튼 업로드 통합.
    - 중복 확인 및 덮어쓰기 모달 기능 유지.
    - performUpload 함수 재사용.
- **디자인 고려사항**:
  - 드래그 오버레이: 파란색(#1890ff) 테마 일관성.
  - 아이콘 크기: 64px (눈에 잘 띄도록).
  - pointerEvents: none (오버레이가 드롭 이벤트 방해 방지).
- **대안 검토**:
  - Upload.Dragger 유지: 화면 공간 낭비, 중복 기능.
  - 작은 드롭 영역 추가: 현대적인 UX와 맞지 않음.
  - 오버레이 없이 드래그만 지원: 시각적 피드백 부족.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (드래그 이벤트, 업로드 버튼 추가)
- **결과**: 전체 영역 드래그 앤 드롭 정상 작동, 업로드 버튼 정상 작동, 화면 공간 효율화.

### Breadcrumb 절대 경로 버그 수정 (2026-02-06)
- **문제**: 하위 폴더(2+ depth) 이동 시 breadcrumb이 절대 경로로 표시되는 버그.
- **원인**:
  - FolderTree에서 Space 노드가 아닌 하위 폴더 선택 시 Space 정보 없이 경로만 전달.
  - FolderContent에서 폴더 더블 클릭 시 `onPathChange(path)` 호출로 경로만 업데이트.
  - `selectedSpace` state가 현재 경로와 동기화되지 않아 breadcrumb 로직이 실패.
  - Breadcrumb 로직: `if (selectedSpace && selectedPath.startsWith(selectedSpace.space_path))`
  - 경로가 다른 Space에 속하거나 selectedSpace가 없으면 절대 경로로 fallback.
- **결정**: `handlePathSelect` 함수에서 Space가 명시되지 않으면 자동으로 찾도록 개선.
- **이유**:
  - 사용자가 어떤 방식으로 탐색하든 (사이드바 클릭, 더블 클릭, breadcrumb 클릭) 일관된 UX 제공.
  - Space 중심 워크플로우 유지: 항상 Space 기준 상대 경로 표시.
  - 코드 중복 방지: FolderTree, FolderContent 모두 수정 불필요.
- **구현**:
  - `MainLayout/index.tsx`의 `handlePathSelect(path, space?)` 수정:
    ```typescript
    if (space) {
      setSelectedSpace(space);
    } else {
      const matchedSpace = spaces?.find(s => path.startsWith(s.space_path));
      setSelectedSpace(matchedSpace);
    }
    ```
  - `path`가 어떤 Space의 하위 경로인지 자동 탐색.
  - 매칭되는 Space를 `selectedSpace`에 설정.
  - 매칭되는 Space가 없으면 `undefined` 설정 (절대 경로 표시).
- **대안 검토**:
  - FolderTree에서 항상 Space 정보 전달: 복잡한 로직, 여러 파일 수정 필요.
  - FolderContent에서 Space 찾기: 중복 로직, MainLayout에서 처리하는 것이 더 효율적.
  - Breadcrumb에서 Space 찾기: 로직이 여러 곳에 분산, 유지보수 어려움.
- **수정 파일**:
  - `apps/frontend/src/components/layout/MainLayout/index.tsx` (handlePathSelect 로직 개선)
- **결과**: 하위 폴더(2+ depth) 탐색 시 breadcrumb이 항상 상대 경로로 정상 표시.

### 그리드 뷰 기본값 변경 (2026-02-06)
- **결정**: 파일 탐색기의 기본 뷰를 테이블에서 그리드(앨범)로 변경.
- **이유**:
  - 사용자 요청: 기본적으로 앨범 형식을 선호.
  - 시각적 탐색 용이성: 그리드 뷰가 파일/폴더 구조를 한눈에 파악하기 쉬움.
  - 현대적인 파일 관리자 트렌드: Finder, Google Drive 등 대부분 그리드 뷰 기본 제공.
  - 이미지/미디어 중심 작업: 그리드 뷰가 파일 미리보기에 더 적합.
- **구현**:
  - `FolderContent.tsx`의 `viewMode` state 초기값 변경.
  - 기존: `useState<'table' | 'grid'>('table')`
  - 변경: `useState<'table' | 'grid'>('grid')`
  - 뷰 전환 기능은 유지: 사용자가 원하면 테이블 뷰로 전환 가능.
- **대안 검토**:
  - localStorage에 사용자 선호 뷰 저장: 추후 고려, 현재는 세션별 초기화.
  - 파일 타입별 자동 뷰 전환: 복잡도 증가, 우선순위 낮음.
  - 그리드 뷰만 제공: 유연성 부족, 사용자 선택권 제거.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (viewMode 기본값 변경)
- **결과**: Space 선택 시 그리드 뷰가 기본으로 표시, 뷰 전환 기능 정상 작동.

### 사이드바 트리 노드 증식 버그 수정 (2026-02-07)
- **문제**: 여러 Space를 열고 닫을 때 하위 폴더 노드가 중복 생성.
- **원인**: 같은 디렉토리를 가리키는 Space들의 하위 노드가 파일 경로를 key로 사용하여 Ant Design Tree key 충돌.
- **결정**: 자식 노드 key에 부모 Space prefix를 붙여 유일성 보장 + `expandedKeys` 명시적 상태 관리.
- **구현**:
  - key 형식: `space-{id}::{filePath}` (Space 하위), 기존 경로 (비-Space).
  - `handleSelect`에서 `::` 구분자로 실제 경로 추출.
  - `expandedKeys` state + `onExpand` 핸들러로 확장 상태 제어.
  - treeData 초기화 시 `loadedKeys`, `expandedKeys` 동시 초기화.
- **수정 파일**: `FolderTree.tsx`

### 서버 상태 표시 개선 (2026-02-07)
- **결정**: Status popover를 텍스트 바로 아래로 이동하고, 호스트/포트/경로 정보 추가.
- **이유**:
  - 기존 popover가 텍스트에서 떨어져 표시되어 연결성이 약함.
  - 사용자가 각 프로토콜의 접근 정보를 한눈에 확인할 수 있어야 함.
- **구현**:
  - 백엔드: `net.InterfaceAddrs()`로 IPv4 주소 수집, `ProtocolStatus`에 `Port`/`Path` 추가.
  - 프론트엔드: `placement="bottomLeft"`, 포트/경로 inline 표시, 호스트 목록 섹션 추가.
- **수정 파일**: `handler.go`, `main.go`, `types.ts`, `ServerStatus.tsx`

### WebDAV 루트 디렉토리 가상 Space 목록 (2026-02-07)
- **결정**: `/dav/` 접근 시 커스텀 `webdav.FileSystem`으로 모든 Space를 가상 폴더로 노출.
- **이유**:
  - WebDAV 클라이언트(Finder 등)가 `/dav/`에 마운트하면 모든 Space를 볼 수 있어야 함.
  - 기존 구현은 `/dav/{spaceName}` 형태만 지원하여 루트 접근 시 400 에러.
- **구현**:
  - `SpaceFS`: `webdav.FileSystem` 인터페이스 구현.
  - 루트(`/`): `Readdir`에서 Space 목록을 가상 디렉토리 FileInfo로 반환.
  - `/{spaceName}/...`: Space의 `SpacePath`로 실제 OS 파일시스템 위임.
  - 루트 Mkdir/RemoveAll/Rename 거부 (Space 관리는 API로만).
  - 경로 탈출 방지: `strings.HasPrefix(realPath, sp.SpacePath)`.
- **대안 검토**:
  - per-space 핸들러 유지 + 루트용 별도 핸들러: 중복 코드, 복잡도 증가.
  - 하나의 SpaceFS로 통합: 단일 핸들러로 루트와 하위 모두 처리, 더 깔끔.
- **수정 파일**: `spacefs.go`(신규), `service.go`, `webdav_handler.go`

### 레이아웃 여백 16px 통일 (2026-02-07)
- **결정**: 모든 레이아웃 영역의 수평 패딩을 `16px` (8-grid 2×8)로 일괄 통일.
- **이유**:
  - Header의 antd 기본 패딩(`0 50px`)이 50px로 8-grid 위반.
  - 사이드바 헤더의 세로 패딩 `12px`가 8-grid 위반.
  - 사이드바(16px), 메인 콘텐츠(24px) 간 수평 패딩 불일치.
  - 시각적으로 "Cohesion" 텍스트와 "Spaces" 라벨이 정렬되지 않음.
- **구현**:
  - Header: `padding: '0 16px'` 명시적 설정 (antd 기본값 오버라이드).
  - 사이드바 헤더: `'12px 16px'` → `'16px'` (상하좌우 균일).
  - 사이드바 트리: `'8px'` → `'8px 16px'` (수평만 통일, 수직은 컴팩트 유지).
  - 메인 콘텐츠: `'24px'` → `'16px'` (수평 패딩 통일).
- **대안 검토**:
  - `24px` 기준 통일: 사이드바에 과도한 여백, 300px 폭에서 비효율적.
  - `8px` 기준 통일: 너무 좁아 답답한 느낌.
- **수정 파일**:
  - `apps/frontend/src/components/layout/MainLayout/index.tsx` (Header padding)
  - `apps/frontend/src/components/layout/MainLayout/MainSider.tsx` (사이드바 헤더, 트리 padding)
  - `apps/frontend/src/features/browse/components/FileExplorer.tsx` (메인 콘텐츠 padding)
- **결과**: 모든 영역 수평 패딩 16px 통일, 다크/라이트 모드 및 그리드/테이블 뷰 모두 정상.

### 컨텍스트 메뉴 UI 개선 — Ant Design Dropdown 전환 (2026-02-08)
- **결정**: 기존 `Menu` + `position: fixed` 인라인 스타일 → Ant Design `Dropdown` 컴포넌트 래핑 공통 `ContextMenu` 생성.
- **이유**:
  - FolderContent, FolderTree에서 동일한 컨텍스트 메뉴 패턴 (상태관리 + document click 리스너) 중복.
  - 화면 경계 처리 없음 (메뉴가 화면 밖으로 잘릴 수 있음).
  - 애니메이션 없이 즉시 출현/사라짐.
  - ESC 키 닫기 미지원.
- **구현**:
  - `src/components/ContextMenu.tsx`: Ant Design `Dropdown` + `trigger={[]}` 제어 모드.
  - 투명 trigger span을 클릭 좌표에 `position: fixed`로 배치, Dropdown이 자동 위치 보정.
  - `useEffect`로 외부 클릭(document click) + ESC 키(keydown) 리스너 등록.
  - FolderContent, FolderTree: `Menu` import 제거, `ContextMenu` 사용, 중복 useEffect 제거.
- **대안 검토**:
  - 커스텀 컴포넌트 (framer-motion): 과도한 개발 비용, Ant Design 일관성 저하.
  - `Dropdown trigger={['contextMenu']}`: Table onRow / Tree onRightClick과 호환 어려움.
- **수정 파일**:
  - `apps/frontend/src/components/ContextMenu.tsx` (신규)
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
  - `apps/frontend/src/features/browse/components/FolderTree.tsx`
- **결과**: 화면 경계 자동 보정, 페이드 애니메이션, ESC/외부 클릭 닫기, 중복 코드 제거.

### 새 폴더 만들기 기능 구현 (2026-02-08)
- **결정**: 빈 영역 우클릭 시 "새 폴더 만들기" 메뉴를 표시하고 모달로 폴더 생성.
- **이유**:
  - 파일 관리 기본 기능: 사용자가 직접 폴더 구조를 만들 수 있어야 함.
  - 파일 복사/이동 기능(Task 2)의 기반: 빈 영역 컨텍스트 메뉴 인프라 구축.
  - 직관적인 UX: Google Drive, Finder 등 주요 파일 관리자와 일관된 경험.
  - 안전한 생성: 중복 확인 및 특수문자 검증으로 파일시스템 보호.
- **구현**:
  - **백엔드**:
    - `POST /api/browse/create-folder` 엔드포인트 추가.
    - 요청: `{ parentPath, folderName }`.
    - 검증 순서:
      1. POST 메서드 확인
      2. Request body 파싱 및 필드 검증 (빈 값 체크)
      3. `parentPath` Space 경로 검증 (`isPathAllowed`)
      4. 부모 디렉토리 존재 및 디렉토리 여부 확인 (`os.Stat`)
      5. 폴더명 유효성 검증 (`/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|` 금지)
      6. 중복 확인 (동일 이름 폴더 존재 시 409 Conflict)
      7. `os.Mkdir(folderPath, 0755)` 실행
      8. 성공 응답 반환 (path, name 포함)
    - 에러 코드: 400 (잘못된 요청), 403 (Space 외부), 404 (부모 없음), 409 (중복), 500 (OS 오류).
  - **프론트엔드**:
    - **빈 영역 감지**:
      - 기존: `e.currentTarget === e.target` (너무 엄격, Grid View/Table View에서 실패).
      - 개선: `!target.closest('.ant-card') && !target.closest('tr')` (카드나 테이블 행이 아닌 곳).
    - **빈 영역 컨텍스트 메뉴**:
      - `emptyAreaMenu` state로 빈 영역 메뉴 상태 관리.
      - `emptyAreaMenuItems`: "새 폴더 만들기" (FolderOutlined 아이콘).
      - ContextMenu 컴포넌트 재사용.
    - **폴더 생성 모달**:
      - `createFolderModal` state로 모달 상태 및 폴더명 관리.
      - Input autoFocus, Enter 키로 생성, 취소/생성 버튼.
      - `handleCreateFolder`: API 호출 → 성공 시 message.success → 목록 새로고침.
    - **API 호출**:
      - `POST /api/browse/create-folder` with `{ parentPath: selectedPath, folderName }`.
      - 409 에러 시 "폴더 이미 존재" 메시지.
- **대안 검토**:
  - 별도 "새 폴더" 버튼: 화면 공간 낭비, 덜 직관적.
  - 파일/폴더 메뉴에 "새 폴더" 추가: 빈 영역 메뉴가 더 직관적.
  - 빈 영역 더블 클릭으로 생성: 실수로 생성 가능, 명시적 메뉴가 더 안전.
  - 자동 이름 생성 ("새 폴더", "새 폴더 2", ...): 사용자 의도와 다를 수 있음, 직접 입력이 더 명확.
- **수정 파일**:
  - `apps/backend/internal/browse/handler/browse_handler.go` (handleCreateFolder, RegisterRoutes)
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (빈 영역 메뉴, 모달, 감지 로직)
- **결과**: 빈 영역 우클릭 → 메뉴 → 모달 → 폴더 생성 → 자동 새로고침 흐름 정상 작동.

### TypeScript 빌드 에러 수정 (2026-02-08)
- **문제**: 프론트엔드 빌드 시 TypeScript 타입 에러 발생.
  - `Upload`, `UploadProps` 임포트되었으나 사용되지 않음 (TS6133, TS6196).
  - 이벤트 핸들러 파라미터에 타입 명시 없음 (TS7006, TS7031).
  - 빌드 실패로 런타임에서 이전 코드가 실행되어 null 참조 에러 발생.
- **결정**: 엄격한 타입 체크를 준수하여 모든 파라미터에 명시적 타입 지정.
- **이유**:
  - TypeScript strict mode: 타입 안정성 향상 및 런타임 에러 예방.
  - 빌드 성공: 최신 코드가 브라우저에 반영되어야 버그 수정 효과 확인 가능.
  - 코드 가독성: 명시적 타입은 IDE 자동완성 및 문서화에 도움.
- **구현**:
  - **FolderContent.tsx**:
    - 미사용 import 제거: `Upload`, `UploadProps`.
    - `onContextMenu`: `(e: React.MouseEvent<HTMLElement>) => ...`
    - `onChange` (rename modal): `(e: React.ChangeEvent<HTMLInputElement>) => ...`
    - `onChange` (create folder modal): `(e: React.ChangeEvent<HTMLInputElement>) => ...`
  - **FolderTree.tsx**:
    - `handleExpand`: `(keys: React.Key[]) => ...`
    - `handleRightClick`: `({ event, node }: { event: React.MouseEvent; node: any }) => ...`
- **대안 검토**:
  - `@ts-ignore` 사용: 타입 체크 우회는 런타임 에러 위험 증가, 유지보수성 저하.
  - `any` 타입 사용: 타입 안정성 손실, 버그 탐지 어려움.
  - 타입 명시: 가장 안전하고 명확한 방법.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
  - `apps/frontend/src/features/browse/components/FolderTree.tsx`
- **결과**: 빌드 성공, 브라우저 새로고침 후 정상 동작 (null 참조 에러 해결).

### 파일 브라우저 정렬 기능 구현 (2026-02-09)
- **결정**: 클라이언트 정렬 방식으로 폴더 우선 정렬 구현.
- **이유**:
  - 현재 폴더 내 파일만 정렬하므로 클라이언트 정렬로 충분 (수백 개 파일도 빠름).
  - 백엔드 수정 불필요, 프론트엔드만 수정하여 빠르게 구현.
  - useMemo로 최적화하여 불필요한 재정렬 방지.
- **구현**:
  - **정렬 상태**: `sortBy` (name/modTime/size), `sortOrder` (ascend/descend).
  - **정렬 로직**:
    1. 폴더 우선: `a.isDir !== b.isDir ? (a.isDir ? -1 : 1)`
    2. sortBy 기준 정렬: localeCompare (이름), getTime (수정일), 숫자 비교 (크기)
    3. sortOrder 적용: ascend는 그대로, descend는 결과 반전
  - **그리드 뷰**: Select 드롭다운으로 6가지 정렬 옵션 제공.
  - **테이블 뷰**: Ant Design Table의 onChange 핸들러로 정렬 상태 업데이트.
  - **뷰 전환**: sortConfig state를 공유하여 뷰 전환 시 정렬 유지.
- **대안 검토**:
  - 서버 정렬: 불필요한 복잡도, 클라이언트 정렬로 충분.
  - 정렬 없이 백엔드 순서대로 표시: 사용자 경험 저하, 파일 관리 어려움.
  - 폴더와 파일 섞어서 정렬: 직관성 떨어짐, 대부분의 파일 관리자가 폴더 우선 채택.
- **기본 정렬**: 폴더 우선 + 이름 오름차순 (가장 직관적).
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
- **결과**: 모든 정렬 옵션 정상 작동, 뷰 전환 시 정렬 유지 확인.

### 드래그 앤 드롭 파일 이동 기능 (2026-02-09)
- **결정**: Google Drive 스타일의 드래그 앤 드롭으로 파일/폴더를 다른 폴더로 이동하는 기능 구현.
- **이유**:
  - 직관적인 UX: 사용자가 익숙한 드래그 앤 드롭 인터페이스 제공.
  - 작업 효율성: 우클릭 메뉴보다 빠른 파일 이동.
  - 다중 이동 지원: 선택된 여러 파일을 한 번에 이동 가능.
- **구현**:
  - **드래그 소스**:
    - 모든 Card와 Table Row에 `draggable={true}` 속성 추가.
    - `onDragStart`: 선택되지 않은 항목 드래그 시 자동 선택, dataTransfer에 경로 목록 저장.
    - 데이터 타입: `application/json` with `{type: 'cohesion-internal', paths: [...]}`
  - **드롭 타겟**:
    - 폴더에만 드롭 가능, 파일에는 불가.
    - `onDragOver`: 폴더에 호버 시 `dragOverFolder` 상태로 시각적 피드백 (파란 테두리/배경).
    - `onDrop`: 폴더에 드롭 시 해당 폴더로 이동, 자기 자신 이동 방지.
    - 빈 영역 드롭: 현재 폴더에 이동 (같은 폴더면 무시).
  - **외부 파일 업로드와 구분**:
    - `dataTransfer.files.length > 0`: 외부 파일 업로드.
    - `dataTransfer.getData('application/json')`: 내부 파일 이동.
    - 외부 파일 드래그 시에만 `isDragging` 상태 활성화 (오버레이 표시).
  - **텍스트 선택 방지**:
    - Card와 Table Row에 `userSelect: 'none'` CSS 적용.
    - 드래그 중 텍스트 선택 안 됨.
  - **시각적 피드백**:
    - 드롭 가능한 폴더: 파란 테두리 (그리드: 2px dashed, 테이블: 배경색).
    - 선택된 항목: 파란 테두리 + 배경색.
- **제약사항**:
  - 자기 자신으로 이동 방지 (message.warning 표시).
  - Space 외부 이동 방지 (기존 move API 검증 활용).
  - 부모 폴더를 자식 폴더로 이동 방지 (백엔드 검증).
- **기존 기능과의 호환성**:
  - 기존 우클릭 메뉴 이동/복사 기능 유지.
  - 외부 파일 업로드 드래그 앤 드롭 정상 작동.
  - 다중 선택 기능과 완벽 통합.
- **대안 검토**:
  - 우클릭만 사용: 작업 효율성 떨어짐, Google Drive 등 경쟁 제품 대비 UX 열등.
  - 별도 버튼으로 이동: 단계가 많아 불편함.
  - 드래그 앤 드롭: 가장 직관적이고 효율적.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
- **결과**: 단일/다중 파일 드래그 이동 성공, 외부 파일 업로드와 충돌 없음, 텍스트 선택 방지 확인.

### Shift 클릭 범위 선택 버그 수정 (2026-02-09)
- **문제**: 정렬된 상태에서 Shift+클릭으로 범위 선택 시 원치 않은 항목 선택.
- **원인**: `handleItemClick`에서 `content` 배열을 사용하지만, 화면에는 `sortedContent`가 표시되어 인덱스 불일치.
- **결정**: `content` 대신 `sortedContent` 사용.
- **이유**:
  - 사용자가 화면에서 보는 순서와 실제 선택되는 항목이 일치해야 직관적.
  - 정렬 기능과 다중 선택 기능이 함께 사용될 때 필수적인 수정.
- **구현**:
  - `handleItemClick`의 Shift+클릭 범위 선택 로직에서 `content[i]` → `sortedContent[i]`로 변경.
  - 인덱스 계산은 동일하게 유지 (화면에 표시된 순서 기준).
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
- **테스트**: 크기 오름차순 정렬 후 첫 항목부터 마지막 항목까지 Shift+클릭으로 9개 항목 선택 성공.
- **결과**: 정렬 상태와 무관하게 화면 순서대로 범위 선택 정상 작동.

## 의존성 관리
### gopsutil v4로 업그레이드 (2026-02-09)
- **결정**: `github.com/shirou/gopsutil` v3.21.11에서 v4.26.1로 업그레이드.
- **이유**:
  - macOS 12.0에서 deprecated된 `IOMasterPort` API 사용으로 인한 빌드 경고 제거.
  - v4에서 해당 API가 `IOMainPort`로 업데이트되어 경고 해결.
  - 최신 버전 사용으로 향후 macOS 호환성 확보.
- **구현**:
  - `go get github.com/shirou/gopsutil/v4@latest` 실행.
  - import 경로 변경: `github.com/shirou/gopsutil/disk` → `github.com/shirou/gopsutil/v4/disk`.
  - v3는 자동으로 제거되지 않고 `go.mod`에 남아있음 (하위 호환성).
- **영향**:
  - `apps/backend/internal/browse/service.go`의 디스크 파티션 정보 조회 기능.
  - API 변경 없음, 기존 코드와 동일하게 동작.
  - 빌드 시 더 이상 deprecation 경고 출력되지 않음.
- **대안 검토**:
  - 경고 무시: 향후 macOS 버전에서 API 제거 시 빌드 실패 가능성.
  - 다른 라이브러리 사용: gopsutil이 Go에서 가장 널리 사용되는 시스템 정보 라이브러리.
- **수정 파일**:
  - `apps/backend/go.mod`, `apps/backend/go.sum` (의존성 업데이트)
  - `apps/backend/internal/browse/service.go` (import 경로 변경)
- **결과**: 빌드 경고 제거, 정상 작동 확인.

## 전역 상태 관리
### Zustand 도입 (2026-02-09)
- **결정**: Zustand를 전역 상태 관리 라이브러리로 도입.
- **이유**:
  - Props Drilling 제거: Space 데이터를 3단계로 전달하는 불편함 해소.
  - 일관된 상태 관리: Context API와 로컬 state가 혼재된 상태를 통일.
  - 확장성: 새로운 전역 상태 추가 시 간단한 store 생성으로 해결.
  - 경량성: Redux보다 가볍고 간단한 API (bundle size 약 1.2KB).
  - 성능: 선택적 구독으로 불필요한 리렌더링 방지.
- **구조**: 기능별 독립 Store
  - `themeStore`: 다크모드 설정, localStorage 연동 (persist middleware).
  - `contextMenuStore`: 전역 컨텍스트 메뉴 상태 관리.
  - `spaceStore`: Space 목록, 선택된 Space, CRUD actions.
  - `browseStore`: 파일 탐색 경로, 컨텐츠 (예정).
- **마이그레이션 완료**:
  - **Phase 1 (Theme Store)**:
    - `themeStore.ts` 생성, localStorage persist 설정.
    - `MainLayout`에서 로컬 isDarkMode state → themeStore.
    - useState 제거, useThemeStore 사용.
  - **Phase 2 (Context Menu Store)**:
    - `contextMenuStore.ts` 생성.
    - `ContextMenu.tsx` 컴포넌트: props → store 직접 접근.
    - `FolderTree.tsx`, `FolderContent.tsx`: `useContextMenu()` → `useContextMenuStore()`.
    - `ContextMenuContext.tsx` 완전 제거.
    - `MainLayout`에서 `ContextMenuProvider` 제거, `<ContextMenu />` 직접 렌더링.
  - **Phase 3 (Space Store)**:
    - `spaceStore.ts` 생성: `fetchSpaces`, `createSpace`, `deleteSpace` actions.
    - `MainLayout`: `useSpaces()` → `useSpaceStore()`.
    - `MainSider`: `spaces`, `onSpaceCreated` props 제거, store 직접 접근.
    - `FolderTree`: `spaces` prop 제거, store 직접 접근.
    - `DirectorySetupModal`: `useCreateSpace()` → store의 `createSpace` action.
    - `useCreateSpace`, `useDeleteSpace` 훅 제거 (기능 store로 통합).
  - **Phase 4 (Browse Store)** (완료):
    - `browseStore.ts` 생성: `selectedPath`, `selectedSpace`, `content`, `isLoading`, `error` 상태 + `setPath`, `fetchDirectoryContents`, `clearContent` actions.
    - `MainLayout`: `pathState` 로컬 state → browseStore, Outlet context 제거.
    - `handlePathSelect`: Space가 명시되지 않으면 경로에서 자동으로 Space 탐지 (`spaces.find(s => path.startsWith(s.space_path))`).
    - `FileExplorer`: Outlet context 제거, props 전달 제거, `<FolderContent />` 단독 렌더링.
    - `FolderContent`: `selectedPath`, `selectedSpace`, `content` props 제거 → store 직접 접근.
    - API 호출 후 `setContent` 중복 제거: `fetchDirectoryContents`가 이미 store 업데이트하므로 8개 위치에서 `setContent` 호출 제거.
    - 미사용 import 제거: `Space` 타입, `useSpaceStore`, `get` 파라미터.
- **마이그레이션 완료**: 모든 Phase (1~4) 완료, Props Drilling 완전 제거, 전역 상태 일관성 확보.
- **수정 파일**:
  - 생성: `stores/themeStore.ts`, `stores/contextMenuStore.ts`, `stores/spaceStore.ts`.
  - 수정: `MainLayout/index.tsx`, `MainSider.tsx`, `ContextMenu.tsx`, `FolderTree.tsx`, `FolderContent.tsx`, `DirectorySetupModal.tsx`.
  - 제거: `contexts/ContextMenuContext.tsx`.
- **검증**: TypeScript 빌드 성공, 미사용 import 제거, ESLint 경고 없음.

## 성능 최적화
### API 중복 호출 최적화 (2026-02-09)
- **문제**: Space 클릭 시 동일한 API가 3번 호출되어 성능 저하.
  - `/api/browse?path=...` 엔드포인트가 중복 호출.
  - 사용자 경험 저하 (불필요한 네트워크 요청, 서버 부하).
- **원인 분석** (Chrome extension 네트워크 추적):
  1. **React StrictMode**: 개발 환경에서 useEffect를 두 번 실행 (+1번).
  2. **두 개의 독립적인 setState**: `selectedPath`와 `selectedSpace` 각각 호출.
     - 각 setState마다 리렌더링 발생 → FolderContent의 useEffect 실행.
  3. **useEffect 의존성**: `[selectedPath, fetchDirectoryContents]`
     - fetchDirectoryContents 함수 참조 변경 시 추가 실행 가능성.
- **결정**: 단일 state 객체로 통합하여 리렌더링 최소화.
- **이유**:
  - React의 배칭(batching)은 이벤트 핸들러 내에서 자동으로 작동하지만, 별개의 state 업데이트는 각각 리렌더링을 트리거할 수 있음.
  - 단일 객체로 관리하면 한 번의 setState로 모든 값 업데이트 → 리렌더링 1회.
  - StrictMode는 프로덕션에서 자동 비활성화되므로, 개발 환경 최적화로 제거 가능.
- **구현**:
  - **StrictMode 제거**: `main.tsx`에서 `<StrictMode>` 래퍼 제거.
  - **단일 state 통합**:
    ```typescript
    // 변경 전
    const [selectedPath, setSelectedPath] = useState<string>('');
    const [selectedSpace, setSelectedSpace] = useState<Space | undefined>();

    // 변경 후
    const [pathState, setPathState] = useState<{ path: string; space?: Space }>({ path: '' });
    ```
  - **useCallback 메모이제이션**: `handlePathSelect` 함수 최적화.
  - **단일 setState**: 한 번의 `setPathState({ path, space })` 호출.
- **결과**:
  - **3번 → 2번 호출**로 감소 (33% 개선).
  - 불필요한 네트워크 요청 감소.
  - Chrome extension 브라우저 테스트로 검증 완료.
- **남은 이슈**:
  - 여전히 2번 호출 발생 (완전한 1번 호출 미달성).
  - 추정 원인: React Router의 Outlet 이중 렌더링 또는 컴포넌트 라이프사이클.
  - 추가 조사 및 최적화 필요.
- **대안 검토**:
  - useEffect 의존성에서 fetchDirectoryContents 제거: ESLint 경고 무시 필요, 근본 해결 아님.
  - useMemo로 API 응답 캐싱: 동일 경로 재방문 시 유용하지만 초기 중복 호출은 해결 못함.
  - React Query 도입: 오버엔지니어링, 현재 상황에서는 과도.
- **수정 파일**:
  - `apps/frontend/src/main.tsx` (StrictMode 제거)
  - `apps/frontend/src/components/layout/MainLayout/index.tsx` (단일 state, useCallback)

## 에러 핸들링
### Space ID 검증 개선 (2026-02-09)
- **문제**: `/api/spaces/` (끝에 슬래시만 있는 경우) 요청 시 에러 발생.
  - `strings.TrimPrefix`로 ID 추출 시 빈 문자열 반환.
  - `strconv.ParseInt("")` 실행으로 파싱 에러 발생.
  - 에러 메시지: `"strconv.ParseInt: parsing \"\": invalid syntax"` (사용자에게 불친절).
- **결정**: `handleSpaceByID`에서 ID 파싱 전 빈 문자열 사전 체크 추가.
- **이유**:
  - 방어적 프로그래밍: 예상 가능한 잘못된 입력에 대한 명확한 처리.
  - 사용자 경험: 기술적인 에러 대신 명확한 메시지 제공.
  - 디버깅 용이성: 로그에서 문제 원인을 빠르게 파악 가능.
- **구현**:
  - `idStr == ""` 체크 추가.
  - 빈 ID: `400 Bad Request`, 메시지: `"Space ID is required"`.
  - 잘못된 형식: `400 Bad Request`, 메시지: `"Invalid space ID format"` (기존 "Invalid space ID"에서 개선).
- **대안 검토**:
  - 라우팅 변경 (`/api/spaces/` 경로 제거): 기존 API 구조 변경 불필요, 검증 추가가 더 간단.
  - 빈 ID를 목록 조회로 리다이렉트: RESTful 원칙 위반, 혼란 야기.
  - 에러 메시지만 개선: 근본 원인(빈 문자열 파싱 시도) 해결 못함.
- **추가 발견 및 수정** (브라우저 테스트 중):
  - Go의 `http.ServeMux`가 `/api/spaces` 요청을 `/api/spaces/` 패턴으로 라우팅.
  - `handleSpaceByID`에서 경로가 정확히 `/api/spaces`인 경우 `handleSpaces`로 위임하도록 수정.
  - 브라우저 테스트 (chrome-extension): 모든 케이스 정상 작동 확인.
- **수정 파일**:
  - `apps/backend/internal/space/handler/space_handler.go` (handleSpaceByID 함수)
- **결과**:
  - `/api/spaces`: 200 OK (Space 목록)
  - `/api/spaces/`: 400 Bad Request ("Space ID is required")
  - `/api/spaces/abc`: 400 Bad Request ("Invalid space ID format")

### 박스 선택 버그 수정 (2026-02-10)
- **문제 1**: 익스플로러 바깥에서도 드래그 시 박스 선택이 동작.
- **문제 2**: 드래그해도 파일/폴더가 다중선택되지 않음.
- **원인 분석**:
  - `useBoxSelection` 훅에서 이벤트 리스너를 `window` 전역에 등록.
  - 페이지 어디서나 드래그가 시작되어 컨테이너 범위 제한 없음.
  - `containerRef`를 의존성 배열에 포함하여 핸들러가 오래된 ref 참조.
  - ref.current가 null일 때 핸들러가 생성되면 이후에도 계속 null 참조.
- **결정**: 이벤트 리스너는 window에 유지하되, handleMouseDown에서 컨테이너 범위 체크.
- **이유**:
  - 컨테이너에 직접 이벤트 등록 시 ref lifecycle 문제 발생.
  - window에 등록하면 드래그가 컨테이너 밖으로 나가도 계속 추적 가능.
  - handleMouseDown에서 범위 체크하면 시작만 컨테이너 내부로 제한.
- **구현**:
  - `useBoxSelection` 훅 수정:
    - `UseBoxSelectionParams`에 `containerRef: RefObject<HTMLElement>` 추가.
    - `handleMouseDown`에서 `containerRef.current.getBoundingClientRect()` 로 범위 체크.
    - 컨테이너 외부 클릭 시 early return.
    - 카드(`ant-card`) 또는 테이블 행(`tr`) 클릭 시에도 early return.
    - 의존성 배열에서 `containerRef` 제거 (ref는 변경되지 않으므로 불필요).
  - `FolderContent.tsx` 수정:
    - `gridContainerRef = useRef<HTMLDivElement>(null)` 생성.
    - Grid를 감싸는 div에 `ref={gridContainerRef}` 연결.
    - `useBoxSelection`에 `containerRef: gridContainerRef` 전달.
- **대안 검토**:
  - 컨테이너에 직접 이벤트 등록: ref lifecycle 복잡도 증가, useEffect 재실행 필요.
  - callback ref 사용: 추가 복잡도, 불필요한 코드.
  - window 등록 + 범위 체크: 간단하고 효과적 (채택).
- **브라우저 테스트**:
  - Chrome extension으로 테스트 시도했으나 automation 한계로 정확한 검증 어려움.
  - 실제 사용자 마우스 테스트 필요.
- **수정 파일**:
  - `apps/frontend/src/features/browse/hooks/useBoxSelection.ts`
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
- **예상 결과**: 컨테이너 내부에서만 박스 선택 시작, 다중선택 정상 작동.

### 박스 선택 후 선택 해제 버그 수정 (2026-02-10)
- **문제**: 박스로 드래그한 파일들이 마우스를 놓으면 즉시 선택 해제됨.
- **원인 분석**:
  - 이벤트 순서: `mousedown` → `mousemove` → `mouseup` → **`click`**
  - `handleContainerClick`이 빈 영역 클릭으로 인식하여 `clearSelection()` 호출.
  - `useFileSelection`의 `handleContainerClick`: 카드/테이블 행이 아닌 곳 클릭 시 선택 해제.
  - 박스 선택 직후 `click` 이벤트가 발생하여 선택이 즉시 해제됨.
- **결정**: 박스 선택 직후 발생하는 `click` 이벤트를 무시하도록 수정.
- **이유**:
  - 박스 선택은 의도적인 다중 선택 동작이므로 유지되어야 함.
  - `preventDefault()`는 기본 동작만 막고 `click` 이벤트 발생 자체를 막지 못함.
  - 플래그 기반 접근이 가장 깔끔하고 명확함.
- **구현**:
  - `useBoxSelection`에 `wasRecentlySelecting` 상태 추가.
  - `handleMouseUp`에서 선택 확정 후:
    ```typescript
    setWasRecentlySelecting(true);
    setTimeout(() => setWasRecentlySelecting(false), 0);
    ```
  - `setTimeout(..., 0)`로 다음 이벤트 루프에서 플래그 해제 (click 이벤트보다 먼저).
  - `FolderContent`의 `handleContainerClick`에서 플래그 확인:
    ```typescript
    if (wasRecentlySelecting) return;
    ```
- **대안 검토**:
  - `stopPropagation()`: 다른 이벤트 핸들러에 영향, 부작용 가능.
  - `preventDefault()`: click 이벤트 발생 자체를 막지 못함.
  - 시간 기반 플래그: 타이밍 문제 가능성.
  - 이벤트 루프 기반 플래그: 가장 안정적 (채택).
- **추가 수정**:
  - TypeScript 빌드 에러 수정:
    - `useBoxSelection` containerRef 타입을 구조적 타이핑으로 변경.
    - `ColumnsType` → `TableColumnsType` import 변경 (antd v5).
    - `error.message` 타입 가드 추가.
- **수정 파일**:
  - `apps/frontend/src/features/browse/hooks/useBoxSelection.ts`
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
  - `apps/frontend/src/features/browse/components/FolderContent/FolderContentTable.tsx`
  - `apps/frontend/src/features/browse/hooks/useFileOperations.ts`
  - `apps/frontend/src/features/browse/constants.tsx`
- **테스트 결과**: 사용자 테스트 완료, 박스 선택 후 선택 상태 정상 유지.

### 모달 열 때 선택 해제 버그 수정 (2026-02-10)
- **문제**: 복사/이동 버튼 클릭 시 모달이 열리면서 선택이 해제됨.
- **원인**:
  - 버튼 클릭 → 이벤트 버블링 → `handleContainerClick` 실행.
  - `handleContainerClick`의 체크 로직:
    - 카드(`.ant-card`) 또는 테이블 행(`tr`)만 체크.
    - 버튼 클릭은 빈 영역 클릭으로 인식 → `clearSelection()` 호출.
- **결정**: 버튼과 입력 필드 클릭도 선택 유지하도록 수정.
- **이유**:
  - 사용자가 의도적으로 버튼을 클릭하는 것은 작업 수행이지 선택 해제 의도가 아님.
  - 모달 열기, 삭제 확인 등 모든 버튼 동작 시 선택 유지 필요.
  - 입력 필드도 마찬가지로 작업 중이므로 선택 유지.
- **구현**:
  ```typescript
  const isButton = target.closest('button');
  const isInput = target.closest('input');
  if (!isCard && !isTableRow && !isButton && !isInput) {
    clearSelection();
  }
  ```
- **대안 검토**:
  - `e.stopPropagation()` 사용: 모든 버튼에 추가 필요, 유지보수 어려움.
  - 선택 바에만 체크 추가: 다른 버튼(컨텍스트 메뉴 등)에서도 문제 발생 가능.
  - 포괄적인 `button`, `input` 체크: 가장 간단하고 확실 (채택).
- **수정 파일**: `apps/frontend/src/features/browse/components/FolderContent.tsx`
- **테스트 결과**: 복사/이동 버튼 클릭 시 선택 유지, 모달 정상 작동.

### 박스 선택 누적 방식 채택 (2026-02-11, #25)
- **문제**: 박스 선택 중 스크롤 시 박스를 벗어난 항목이 선택 해제되는 비직관적 UX.
  - 사용자 기대: "한 번이라도 박스에 걸린 항목은 계속 선택 상태여야 한다."
  - 기존 동작: 현재 박스와 교차하는 항목만 선택 (Viewport 좌표계).
  - 스크롤 시 새로운 항목이 선택되고 이전 항목은 해제됨.
- **원인 분석**:
  - `updateSelection`이 매번 교차 항목으로 선택 상태를 덮어씀.
  - `handleScroll`에서 현재 교차 항목만 전달하여 이전 선택이 사라짐.
  - 선택 박스는 viewport에 고정, 아이템들이 스크롤로 이동.
- **결정**: 드래그 중 교차한 모든 항목을 누적하는 방식으로 변경.
- **이유**:
  - 직관적인 UX: 대부분의 파일 탐색기(Windows, macOS)가 이 방식 사용.
  - 사용자 멘탈 모델 일치: "지나간 영역의 파일들이 선택된다".
  - 실수 방지: 우연히 스크롤해도 선택이 사라지지 않음.
  - 많은 파일 선택 시 효율적: 드래그 + 스크롤로 빠른 선택.
- **구현**:
  - `accumulatedSelection = useRef<Set<string>>(new Set())` 추가.
  - `handleMouseDown`: `accumulatedSelection` 초기화.
  - `handleMouseMove`, `handleScroll`, `handleMouseUp`:
    ```typescript
    intersected.forEach(path => accumulatedSelection.current.add(path));
    ```
  - Ctrl/Shift 모드 처리를 각 핸들러에서 직접 계산:
    - 일반 모드: `new Set(accumulatedSelection.current)` (누적된 항목만).
    - Shift 모드: `new Set([...initialSelection.current, ...accumulatedSelection.current])`.
    - Ctrl 모드: `initialSelection`에서 누적 항목 토글.
  - `updateSelection` 함수 제거 (더 이상 불필요).
- **대안 검토**:
  - **Content 좌표계**: 선택 박스를 content에 고정 (스크롤 offset 포함).
    - 장점: 선택 박스가 content와 함께 움직여 더 직관적일 수 있음.
    - 단점: 구현 복잡도 높음, 박스가 화면 밖으로 사라질 수 있음.
    - 기각 이유: 현재 viewport 좌표계를 유지하면서 누적 방식으로 UX 개선 가능.
  - **드래그 중 스크롤 비활성화**: 스크롤 이벤트 자체를 막음.
    - 장점: 선택 변경 문제 원천 차단.
    - 단점: 많은 파일 선택 시 불편함, 기능 제한.
    - 기각 이유: 스크롤은 유용한 기능이므로 유지하고 동작만 개선.
  - **자동 스크롤**: 드래그 중 마우스를 가장자리에 대면 자동 스크롤.
    - 장점: 일반적인 패턴, 마우스만으로 모든 조작 가능.
    - 단점: 추가 구현 필요, 현재 이슈와는 별개.
    - 판단: 향후 개선 사항으로 분류, 현재는 누적 방식으로 충분.
- **변경 사항**:
  - 126줄 추가, 74줄 삭제 (순 +52줄).
  - `updateSelection` 함수 삭제로 코드 중복 감소.
  - 각 핸들러에서 선택 계산 로직이 명확하게 드러남.
- **수정 파일**:
  - `apps/frontend/src/features/browse/hooks/useBoxSelection.ts`
- **Issue**: #25 (https://github.com/lteawoo/Cohesion/issues/25)
- **Commit**: `1c07bed`
- **향후 개선 고려 사항**:
  - 자동 스크롤 기능 추가 (마우스를 가장자리에 대면 자동으로 스크롤).
  - 선택 박스 시각적 피드백 개선 (누적되고 있음을 명확히 표시).
- **테스트 계획**: Grid 뷰에서 드래그 + 스크롤로 많은 파일 선택 후 선택 상태 유지 확인.
### Grid 뷰 이미지 썸네일 표시 방식 결정 (2026-02-11)
- **문제**: Grid 뷰에서 이미지 파일이 일반 파일 아이콘으로 표시되어 이미지 파일 식별이 어려움.
- **목표**: 이미지 파일의 실제 내용을 미리보기로 표시하여 사용자 경험 개선.
- **고려한 방안**:
  1. **프론트엔드 직접 로드 방식** (채택)
     - 기존 `/api/browse/download` 엔드포인트 활용.
     - 브라우저 네이티브 `loading="lazy"` 사용.
     - 장점: 빠른 구현, 별도 API 불필요, 간단한 아키텍처.
     - 단점: 원본 파일 다운로드 (네트워크 사용량 증가 가능).
  2. **백엔드 썸네일 생성 API**
     - 썸네일 생성 라이브러리 필요 (imaging, disintegration/imaging 등).
     - 썸네일 크기 조정 및 캐싱 구현.
     - 장점: 네트워크 효율적, 최적화된 이미지 제공.
     - 단점: 구현 복잡도 높음, 외부 의존성 추가, 캐싱 관리 필요.
  3. **하이브리드 방식**
     - 일정 크기 이하: 프론트엔드 직접 로드.
     - 일정 크기 이상: 백엔드 썸네일 생성.
     - 장점: 균형잡힌 접근.
     - 단점: 복잡도 증가, 일관성 부족.
- **결정**: 방안 1 (프론트엔드 직접 로드 방식) 채택.
- **이유**:
  - 빠른 구현: 기존 API 활용, 추가 백엔드 작업 없음.
  - 충분한 성능: 로컬 네트워크에서 원본 로드도 빠름, lazy loading으로 최적화.
  - 단순한 아키텍처: 유지보수 쉬움, 외부 의존성 없음.
  - 향후 확장 가능: 필요 시 백엔드 썸네일 생성으로 전환 가능.
- **구현**:
  - `ImageThumbnail` 컴포넌트 생성:
    ```typescript
    <img
      src={`/api/browse/download?path=${encodeURIComponent(path)}`}
      loading="lazy"
      onLoad={() => setLoading(false)}
      onError={() => setError(true)}
    />
    ```
  - `fileTypeUtils` 유틸리티: 이미지 확장자 감지 (jpg, jpeg, png, gif, webp, svg, bmp, ico).
  - `FolderContentGrid`: 이미지 파일 감지 시 `ImageThumbnail` 렌더링.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/ImageThumbnail.tsx` (신규)
  - `apps/frontend/src/features/browse/utils/fileTypeUtils.ts` (신규)
  - `apps/frontend/src/features/browse/components/FolderContent/FolderContentGrid.tsx`
- **Commit**: `6fade96`
- **향후 개선 고려 사항**:
  - 네트워크 사용량이 문제가 되면 백엔드 썸네일 생성 API 추가.
  - 더 많은 이미지 포맷 지원 (tiff, heic 등).
  - 비디오 파일 썸네일 지원.

### 이동/복사 모달 Space 트리 구조 결정 (2026-02-11)
- **문제**: 이동/복사 모달의 FolderTree가 잘못된 방식으로 동작.
  - **단일 Space만 표시**: `selectedSpace`가 있을 때 해당 Space 내부만 표시, Space 간 이동 불가능.
  - **시스템 디렉토리 노출**: `selectedSpace`가 없을 때 `/Users`, `/Applications` 등 시스템 디렉토리 표시 (보안/UX 문제).
- **올바른 동작**: 모든 Space 목록 표시, Space 간 파일 이동/복사 가능.
- **원인 분석**:
  - `DestinationPickerModal`이 `FolderTree`에 props 전달:
    ```typescript
    <FolderTree
      rootPath={selectedSpace?.space_path}
      rootName={selectedSpace?.space_name}
      showBaseDirectories={!selectedSpace}
    />
    ```
  - `FolderTree` 로직 우선순위:
    1. `showBaseDirectories`: 시스템 디렉토리 표시 (Space 등록용).
    2. `rootPath`, `rootName`: 단일 Space 표시.
    3. (props 없음): 모든 Space 목록 표시 (원하는 동작!).
- **고려한 방안**:
  1. **새로운 prop 추가** (`spacePickerMode`)
     - `FolderTree`에 `spacePickerMode` prop 추가하여 명시적으로 모드 구분.
     - 장점: 명확한 의도 표현.
     - 단점: prop 증가, 로직 복잡도 증가, 기존 동작 변경 위험.
  2. **Props 제거** (채택)
     - `DestinationPickerModal`에서 FolderTree props를 모두 제거.
     - `FolderTree`가 자동으로 모든 Space 표시 (3순위 로직).
     - 장점: 가장 간단, 명확, 기존 로직 활용.
     - 단점: 없음.
  3. **별도 컴포넌트 생성** (`SpacePicker`)
     - Space 선택 전용 컴포넌트 생성.
     - 장점: 명확한 분리.
     - 단점: 코드 중복, 유지보수 어려움, 과도한 추상화.
- **결정**: 방안 2 (Props 제거) 채택.
- **이유**:
  - 가장 간단한 해결책: 3줄 삭제로 문제 해결.
  - 기존 로직 활용: `FolderTree`가 이미 Space 목록 표시 기능 보유.
  - 메인 사이드바와 동일한 동작: 일관성 유지.
  - 안전한 변경: Space 등록 모달(`showBaseDirectories`)과 충돌 없음.
- **구현**:
  ```diff
  <FolderTree
    onSelect={handleSelect}
  - rootPath={selectedSpace?.space_path}
  - rootName={selectedSpace?.space_name}
  - showBaseDirectories={!selectedSpace}
  />
  ```
- **기술적 고려사항**:
  - **Space 경로 검증**: 백엔드 `isPathAllowed`로 Space 외부 이동 차단 (기존 구현).
  - **하위 폴더 순환 참조 방지**: 프론트엔드에서 사전 체크 (기존 구현).
  - **Space 목록 동기화**: Zustand store에서 자동으로 최신 목록 표시.
- **영향 분석**:
  - **Space 등록 모달**: `showBaseDirectories={true}` → 1순위 로직 (변경 없음).
  - **메인 사이드바**: props 없음 → 3순위 로직 (변경 없음).
  - **이동/복사 모달**: props 제거 → 3순위 로직 (개선됨).
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/DestinationPickerModal.tsx` (3줄 삭제)
- **Commit**: `85ddd94`
- **테스트 계획**:
  - Space 간 파일 이동/복사 정상 작동 확인.
  - Space 등록 모달 정상 작동 확인 (회귀 테스트).
  - 메인 사이드바 정상 작동 확인 (회귀 테스트).
- **향후 개선 고려 사항**:
  - 현재 Space 강조 표시 (`defaultExpandedKeys`).
  - 최근 사용 폴더 기록 (localStorage).
  - Space 간 이동 시 확인 모달 추가.

### 이동/복사 모달 클릭 이벤트 버블링 처리 결정 (2026-02-13)
- **문제**: 이동/복사 모달에서 대상 폴더를 선택하면 파일 익스플로러의 기존 선택이 해제됨.
- **원인 분석**:
  - `FolderContent` 루트 컨테이너의 `onClick`(`handleContainerClick`)는 빈 영역 클릭 시 `clearSelection()` 수행.
  - 이동/복사 모달은 React Portal로 렌더링되지만 이벤트 버블링은 React 트리를 따라 상위 컴포넌트(`FolderContent`)로 전달됨.
  - 모달 내부의 Tree 노드 클릭이 컨테이너 빈 영역 클릭으로 오인되어 선택이 해제됨.
- **결정**: `handleContainerClick`에서 `.ant-modal` 내부 클릭은 선택 해제 로직에서 제외.
- **이유**:
  - 선택 해제 의도는 파일 목록 빈 영역 클릭에 한정되어야 함.
  - 모달 내부 조작은 원본 selection state를 유지해야 이동/복사 작업의 일관성이 보장됨.
  - 영향 범위가 작고 회귀 위험이 낮음.
- **구현**:
  - `const isModalContent = target.closest('.ant-modal');`
  - `if (isModalContent) return;`
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
- **검증**:
  - 린트/빌드 통과.
  - 브라우저 수동 테스트로 모달 폴더 선택 후에도 `N개 선택됨` 유지 확인.
  - 증빙 스크린샷: `.playwright-mcp/move-modal-selection-fixed.png`

### 이동/복사 모달에서 source selection 안정성 보강 결정 (2026-02-13)
- **추가 원인**:
  - 기존 구조는 이동/복사 모달이 `selectedItems`를 실시간 참조.
  - 전역 경로 상태(`selectedPath`, `selectedSpace`) 변화가 발생하면 `FolderContent`의 effect에서 `clearSelection()`이 실행되어 모달 작업 중 source가 사라질 수 있음.
- **근본 대응 결정**:
  1. 이동/복사 모달 오픈 시점의 source 목록을 스냅샷으로 고정.
  2. 모달 열림 중에는 네비게이션 변화가 있어도 자동 selection clear를 수행하지 않음.
- **구현**:
  - `useModalManager`의 `DestinationModalData`에 `sources: string[]` 추가.
  - `openModal('destination', { mode, sources })`로 현재 선택 목록 캡처.
  - `handleMoveConfirm`/`handleCopyConfirm`은 실시간 `selectedItems` 대신 `modals.destination.data.sources` 사용.
  - `selectedPath/selectedSpace` effect에서 `modals.destination.visible`일 때 `clearSelection()` skip.
- **수정 파일**:
  - `apps/frontend/src/features/browse/hooks/useModalManager.ts`
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`

### 파일 작업 후 트리 반영을 위한 invalidate 구조 결정 (2026-02-13)
- **문제**: 복사/이동/삭제/이름변경/폴더생성 후 우측 목록은 갱신되지만, 좌측/모달 트리는 stale 상태가 남음.
- **원인**:
  - 파일 작업 훅(`useFileOperations`)은 `refreshContents()`로 현재 폴더 내용만 재조회.
  - `FolderTree`는 `treeData`, `loadedKeys`, `expandedKeys` 로컬 캐시를 유지하여 자동 무효화되지 않음.
- **결정**: 전역 `treeRefreshVersion` 기반 invalidate 패턴 도입.
- **구현**:
  - `browseStore`에 `treeRefreshVersion`, `invalidateTree()` 추가.
  - 파일 작업 성공 후(`rename/create-folder/delete/move/copy`) `invalidateTree()` 호출.
  - `FolderTree`는 `treeRefreshVersion` 변경을 감지해 초기 트리 데이터 재구성 및 로컬 캐시 초기화.
- **효과**:
  - 파일 작업 직후 트리 컴포넌트(사이드바/모달) stale 상태 해소.
  - 별도 수동 새로고침 없이 최신 구조 반영.

### 에이전트 실행 문서 위치 정리 결정 (2026-02-13)
- **결정**: 실행 환경 가이드를 `docs/AGENTS.md`에서 루트 `AGENTS.md`로 통합.
- **이유**:
  - 에이전트 진입 시점에 루트에서 즉시 참조 가능.
  - `CLAUDE.md`, `GEMINI.md`와의 참조 일관성 유지.
- **변경**:
  - `docs/AGENTS.md` 제거.
  - 루트 `AGENTS.md` 유지/갱신.
  - `master_rule_v2.md`의 검증 절차 문구 최신화.
