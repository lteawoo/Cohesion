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