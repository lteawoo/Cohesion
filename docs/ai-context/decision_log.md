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